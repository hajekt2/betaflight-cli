package cli

import (
	"encoding/json"
	"fmt"
	"strings"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"

	"github.com/hajekt2/betaflight-cli/internal/batch"
	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
	"github.com/spf13/cobra"
)

func (a *app) configurationCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "configuration", Short: "Inspect configuration state and write readiness"}
	cmd.AddCommand(a.configurationValidateCommand())
	cmd.AddCommand(a.configurationPlanCommand())
	cmd.AddCommand(a.configurationResetCommand())
	cmd.AddCommand(a.configurationApplyCommand())
	cmd.AddCommand(a.configurationCompareCommand())
	cmd.AddCommand(a.configurationExportCommand())
	cmd.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Read and parse configuration diff output",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), "diff all")
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				doc := bfconfig.Parse(lines, settings.DefaultRegistry)
				return output.Success(commandPath(cmd), &target, map[string]any{
					"command":           "diff all",
					"lines":             lines,
					"raw":               strings.Join(lines, "\n"),
					"sections":          doc.Sections,
					"configuration":     doc,
					"inventory":         bfconfig.BuildInventory(doc),
					"raw_authoritative": true,
				})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read configuration state, profiles, arming blockers, and write guidance",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				status, warnings := bfcommands.ReadConfigurationStatus(cmd.Context(), client)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"configuration": status,
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "snapshot",
		Short: "Read dump and diff configuration snapshots for review",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				snapshot, err := bfcommands.ReadConfigurationSnapshot(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"configuration_snapshot": snapshot,
				})
			})
		},
	})
	return cmd
}

func (a *app) configurationPlanCommand() *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Validate and print a configuration change plan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			imported, err := a.readImportPlan(opts, "configuration", "configuration_text")
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: opts.includeDefaults})
			if !ok {
				return a.render(env)
			}
			return a.render(output.Success(commandPath(cmd), nil, importPlanData(imported, opts, false)))
		},
	}
	addImportFlags(cmd, &opts, false)
	return cmd
}

func (a *app) configurationApplyCommand() *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a validated configuration plan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.includeDefaults && !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--include-defaults requires --yes because defaults nosave resets configuration before applying lines"))
			}
			imported, err := a.readImportPlan(opts, "configuration", "configuration_text")
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: opts.includeDefaults})
			if !ok {
				return a.render(env)
			}
			op := connection.Write
			if opts.includeDefaults {
				op = connection.Dangerous
			}
			return a.applyImportPlan(cmd, imported, opts, op)
		},
	}
	addImportFlags(cmd, &opts, true)
	return cmd
}

func (a *app) configurationResetCommand() *cobra.Command {
	var save bool
	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset flight controller configuration to factory defaults",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "configuration reset requires --yes because it erases all configuration on the flight controller"))
			}
			imported, err := batch.ImportCLI([]byte("defaults nosave\n"), batch.ImportOptions{
				Kind:            "restore",
				SourceFormat:    "configuration_text",
				IncludeDefaults: true,
			})
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			if env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: true}); !ok {
				return a.render(env)
			}
			opts := importCommandOptions{includeDefaults: true, save: save}
			data := importPlanData(imported, opts, false)
			connect := a.connect
			if connect == nil {
				connect = connection.Connect
			}
			client, targetInfo, err := connect(cmd.Context(), a.connectionConfig(), connection.Dangerous)
			target := toOutputTarget(targetInfo)
			if err != nil {
				env := a.failure(commandPath(cmd), &target, err)
				env.Data = data
				return a.render(env)
			}
			defer client.Close()
			renderConnected := func(env output.Envelope) error {
				a.addUnsupportedFirmwareWarning(&env, targetInfo)
				return a.render(env)
			}
			responses, appliedLines, failedLine, err := executeCLIPlan(cmd.Context(), client, imported.Plan.CLILines)
			data["response_lines"] = responses
			data["applied_cli_lines"] = appliedLines
			if err != nil {
				data["partially_applied"] = len(appliedLines) > 0
				data["failed_cli_line"] = failedLine
				refreshChangePlan(data)
				env := a.failure(commandPath(cmd), &target, err)
				env.Data = data
				addCLIApplySideEffects(&env, appliedLines)
				return renderConnected(env)
			}
			data["applied"] = true
			refreshChangePlan(data)
			env := output.Success(commandPath(cmd), &target, data)
			addCLIApplySideEffects(&env, appliedLines)
			if save {
				saveLines, err := client.ExecCLI(cmd.Context(), "save")
				if err != nil {
					addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
				}
				data["saved"] = true
				data["save_response_lines"] = saveLines
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
			}
			addStringWarnings(&env, []string{"factory reset erases all configuration on the flight controller; run 'backup create --raw-cli' beforehand to keep a restorable copy"})
			return renderConnected(env)
		},
	}
	cmd.Flags().BoolVar(&save, "save", false, "persist defaults after resetting; requires --yes")
	return cmd
}

func (a *app) configurationExportCommand() *cobra.Command {
	var source string
	var redact bool
	var rawCLI bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export current Betaflight configuration as JSON or raw CLI text",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cliLine := ""
			switch strings.ToLower(source) {
			case "full", "dump", "dump-all":
				cliLine = "dump all"
			case "diff", "diff-all":
				cliLine = "diff all"
			default:
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "--source must be one of full or diff"))
			}
			return a.runBackupCommand(cmd, cliLine, redact, rawCLI)
		},
	}
	cmd.Flags().StringVar(&source, "source", "full", "configuration source: full or diff")
	cmd.Flags().BoolVar(&redact, "redact", false, "redact sensitive values for sharing")
	cmd.Flags().BoolVar(&rawCLI, "raw-cli", false, "emit raw CLI text as the command data")
	return cmd
}

func (a *app) configurationCompareCommand() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare local Betaflight CLI configuration text with the current controller dump",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := a.readInput(file)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_error", err.Error()))
			}
			referenceLines, _ := configurationLinesFromInput(data)
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				compare, err := bfcommands.ReadConfigurationCompare(cmd.Context(), client, referenceLines)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"configuration_compare": compare,
				})
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "-", "reference configuration text file path, or - for stdin")
	return cmd
}

func (a *app) configurationValidateCommand() *cobra.Command {
	var file string
	var includeDefaults bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Betaflight CLI configuration text without connecting",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := a.readInput(file)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_error", err.Error()))
			}
			lines, sourceFormat := configurationLinesFromInput(data)
			doc := bfconfig.Parse(lines, settings.DefaultRegistry)
			importData := []byte(strings.Join(lines, "\n"))
			result := map[string]any{
				"source_format":     sourceFormat,
				"line_count":        countNonEmptyInputLines(lines),
				"section_counts":    countConfigurationSections(doc.Sections),
				"inventory":         bfconfig.BuildInventory(doc),
				"unknown_count":     len(doc.Unknown),
				"unknown_settings":  countUnknownConfigurationSettings(doc.Settings),
				"document":          doc,
				"include_defaults":  includeDefaults,
				"raw_authoritative": true,
			}
			validation := map[string]any{
				"valid":  true,
				"errors": []output.Error{},
			}
			imported, err := batch.ImportCLI(importData, batch.ImportOptions{
				Kind:            "configuration_validation",
				SourceFormat:    sourceFormat,
				IncludeDefaults: includeDefaults,
			})
			if err != nil {
				validation["valid"] = false
				validation["errors"] = []output.Error{{Code: "validation_error", Message: err.Error()}}
			} else {
				result["plan"] = imported.Plan
				result["skipped_lines"] = imported.Skipped
				if env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: includeDefaults}); !ok {
					validation["valid"] = false
					validation["errors"] = env.Errors
				}
			}
			if len(doc.Unknown) > 0 || countUnknownConfigurationSettings(doc.Settings) > 0 {
				validation["review_required"] = true
			} else {
				validation["review_required"] = validation["valid"] == false
			}
			result["validation"] = validation
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"configuration_validation": result,
			}))
		},
	}
	cmd.Flags().StringVar(&file, "file", "-", "configuration text file path, or - for stdin")
	cmd.Flags().BoolVar(&includeDefaults, "include-defaults", false, "include exact defaults nosave lines in validation")
	return cmd
}

func countNonEmptyInputLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func configurationLinesFromInput(data []byte) ([]string, string) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		var payload map[string]any
		if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
			if lines, ok := configurationLinesFromJSONMap(payload); ok {
				return lines, "json"
			}
			if nested, ok := payload["data"].(map[string]any); ok {
				if lines, ok := configurationLinesFromJSONMap(nested); ok {
					return lines, "json"
				}
			}
		}
	}
	return strings.Split(string(data), "\n"), "cli_text"
}

func configurationLinesFromJSONMap(payload map[string]any) ([]string, bool) {
	if lines, ok := payload["lines"].([]any); ok {
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			text, ok := line.(string)
			if !ok {
				return nil, false
			}
			out = append(out, text)
		}
		return out, true
	}
	if raw, ok := payload["raw"].(string); ok {
		return strings.Split(raw, "\n"), true
	}
	return nil, false
}

func countConfigurationSections(sections map[string][]string) map[string]int {
	counts := map[string]int{}
	for name, lines := range sections {
		counts[name] = len(lines)
	}
	return counts
}

func countUnknownConfigurationSettings(settings []bfconfig.Setting) int {
	count := 0
	for _, setting := range settings {
		if !setting.Known {
			count++
		}
	}
	return count
}
