package cli

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

type settingJSONChange struct {
	Name     string            `json:"name"`
	Value    string            `json:"value"`
	Metadata settings.Metadata `json:"metadata"`
}

type settingJSONRow struct {
	Name  string          `json:"name"`
	Value json.RawMessage `json:"value"`
}

func (a *app) settingsSetJSONCommand() *cobra.Command {
	var flags changeFlags
	cmd := &cobra.Command{
		Use:   "set-json FILE",
		Short: "Plan or apply multiple metadata-backed setting changes from JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			changes, err := parseSettingsSetJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			lines := make([]string, len(changes))
			for i, change := range changes {
				lines[i] = fmt.Sprintf("set %s = %s", change.Name, change.Value)
			}
			plan := map[string]any{
				"kind":      "settings",
				"settings":  changes,
				"cli_lines": lines,
				"applied":   false,
				"saved":     false,
			}
			if !flags.apply {
				return a.render(output.Success(commandPath(cmd), nil, plan))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "apply requires --yes"))
			}
			op := connection.Write
			if flags.save {
				op = connection.Dangerous
			}
			return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
				responses := map[string][]string{}
				for _, line := range lines {
					responseLines, err := client.ExecCLI(cmd.Context(), line)
					if err != nil {
						return a.failure(commandPath(cmd), &target, err)
					}
					responses[line] = responseLines
				}
				plan["applied"] = true
				plan["response_lines"] = responses
				env := output.Success(commandPath(cmd), &target, plan)
				for _, line := range lines {
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: line, Detail: "configuration change applied but not saved"})
				}
				if flags.save {
					saveLines, err := client.ExecCLI(cmd.Context(), "save")
					if err != nil {
						addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
					}
					plan["saved"] = true
					plan["save_response_lines"] = saveLines
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
				}
				return env
			})
		},
	}
	addChangeFlags(cmd, &flags)
	return cmd
}

func parseSettingsSetJSON(data []byte) ([]settingJSONChange, error) {
	if isJSONArray(data) {
		var rows []settingJSONRow
		if err := json.Unmarshal(data, &rows); err != nil {
			return nil, err
		}
		return validateSettingJSONRows(rows)
	}
	var wrapped struct {
		Settings json.RawMessage `json:"settings"`
		Changes  json.RawMessage `json:"changes"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	source := data
	if len(wrapped.Settings) > 0 {
		source = wrapped.Settings
	} else if len(wrapped.Changes) > 0 {
		source = wrapped.Changes
	}
	var rows []settingJSONRow
	if isJSONArray(source) {
		if err := json.Unmarshal(source, &rows); err != nil {
			return nil, err
		}
		return validateSettingJSONRows(rows)
	}
	var values map[string]json.RawMessage
	if err := json.Unmarshal(source, &values); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(values))
	for name := range values {
		names = append(names, name)
	}
	sort.Strings(names)
	rows = make([]settingJSONRow, 0, len(names))
	for _, name := range names {
		rows = append(rows, settingJSONRow{Name: name, Value: values[name]})
	}
	return validateSettingJSONRows(rows)
}

func validateSettingJSONRows(rows []settingJSONRow) ([]settingJSONChange, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("settings JSON must include at least one setting")
	}
	seen := map[string]bool{}
	changes := make([]settingJSONChange, 0, len(rows))
	for i, row := range rows {
		name := strings.TrimSpace(row.Name)
		if name == "" {
			return nil, fmt.Errorf("settings[%d].name is required", i)
		}
		metadata, ok := settings.DefaultRegistry.Lookup(name)
		if !ok {
			return nil, fmt.Errorf("%q is not a known setting", name)
		}
		if seen[strings.ToLower(metadata.Name)] {
			return nil, fmt.Errorf("%s appears more than once", metadata.Name)
		}
		seen[strings.ToLower(metadata.Name)] = true
		value, err := settingJSONValueString(row.Value)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", metadata.Name, err)
		}
		if err := metadata.Validate(value); err != nil {
			return nil, err
		}
		changes = append(changes, settingJSONChange{Name: metadata.Name, Value: value, Metadata: metadata})
	}
	return changes, nil
}

func settingJSONValueString(raw json.RawMessage) (string, error) {
	trimmed := strings.TrimSpace(string(raw))
	if trimmed == "" || trimmed == "null" {
		return "", fmt.Errorf("value is required")
	}
	if strings.HasPrefix(trimmed, `"`) {
		var value string
		if err := json.Unmarshal(raw, &value); err != nil {
			return "", err
		}
		return value, nil
	}
	if trimmed == "true" {
		return "ON", nil
	}
	if trimmed == "false" {
		return "OFF", nil
	}
	if strings.HasPrefix(trimmed, "[") {
		var values []json.RawMessage
		if err := json.Unmarshal(raw, &values); err != nil {
			return "", err
		}
		parts := make([]string, len(values))
		for i, value := range values {
			part, err := settingJSONValueString(value)
			if err != nil {
				return "", fmt.Errorf("array[%d]: %w", i, err)
			}
			parts[i] = part
		}
		return strings.Join(parts, ","), nil
	}
	if strings.HasPrefix(trimmed, "{") {
		return "", fmt.Errorf("object values are not supported")
	}
	return trimmed, nil
}
