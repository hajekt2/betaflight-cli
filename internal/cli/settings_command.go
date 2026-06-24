package cli

import (
	"fmt"
	"strings"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"

	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
	"github.com/spf13/cobra"
)

func (a *app) settingsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "settings", Short: "Plan and apply setting changes"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List compiled setting metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.render(output.Success(commandPath(cmd), nil, settings.DefaultRegistry))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "metadata NAME",
		Short: "Show compiled metadata for one setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			setting, ok := settings.DefaultRegistry.Lookup(args[0])
			if !ok {
				return a.render(output.Failure(commandPath(cmd), nil, "unknown_setting", fmt.Sprintf("no compiled metadata for %q", args[0])))
			}
			return a.render(output.Success(commandPath(cmd), nil, setting))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get NAME",
		Short: "Read a setting through Betaflight CLI get",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), "get "+name)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				data := map[string]any{
					"setting": name,
					"lines":   lines,
					"raw":     strings.Join(lines, "\n"),
				}
				if setting, ok := settings.DefaultRegistry.Lookup(name); ok {
					data["metadata"] = setting
				}
				return output.Success(commandPath(cmd), &target, data)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "firmware-get NAME",
		Short: "Read a setting through MSP2_CLI_SETTING",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				setting, err := bfcommands.ReadFirmwareSetting(cmd.Context(), client, name)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				data := map[string]any{"firmware_setting": setting}
				if compiled, ok := settings.DefaultRegistry.Lookup(name); ok {
					data["compiled_metadata"] = compiled
				}
				env := output.Success(commandPath(cmd), &target, data)
				if !setting.Supported {
					env.Warnings = append(env.Warnings, output.Warning{Code: "unsupported_msp", Message: setting.UnsupportedReason})
				}
				return env
			})
		},
	})
	var infoOffset uint16
	infoCmd := &cobra.Command{
		Use:   "firmware-info NAME",
		Short: "Read firmware setting metadata text through MSP2_CLI_SETTING_INFO",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				info, err := bfcommands.ReadFirmwareSettingInfo(cmd.Context(), client, name, infoOffset)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				data := map[string]any{"firmware_setting_info": info}
				if compiled, ok := settings.DefaultRegistry.Lookup(name); ok {
					data["compiled_metadata"] = compiled
				}
				env := output.Success(commandPath(cmd), &target, data)
				if !info.Supported {
					env.Warnings = append(env.Warnings, output.Warning{Code: "unsupported_msp", Message: info.UnsupportedReason})
				}
				return env
			})
		},
	}
	infoCmd.Flags().Uint16Var(&infoOffset, "offset", 0, "byte offset into firmware setting info text")
	cmd.AddCommand(infoCmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Run non-interactive `diff all` and parse response",
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
	var apply bool
	var save bool
	set := &cobra.Command{
		Use:   "set NAME VALUE",
		Short: "Plan or apply a CLI-backed setting change",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, value := args[0], args[1]
			setting, known := settings.DefaultRegistry.Lookup(name)
			if known {
				if err := setting.Validate(value); err != nil {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
				}
			}
			line := fmt.Sprintf("set %s = %s", name, value)
			plan := map[string]any{
				"setting":   name,
				"value":     value,
				"cli_lines": []string{line},
				"applied":   false,
				"saved":     false,
			}
			if known {
				plan["metadata"] = setting
			}
			if !apply {
				env := output.Success(commandPath(cmd), nil, withChangePlanRoot(plan))
				if !known {
					env.Warnings = append(env.Warnings, output.Warning{Code: "metadata_missing", Message: fmt.Sprintf("no compiled metadata for %q; validation skipped", name)})
				}
				return a.render(env)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "apply requires --yes"))
			}
			if save && !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--save requires --yes because it persists and usually reboots the flight controller"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), line)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				plan["applied"] = true
				plan["response_lines"] = lines
				env := output.Success(commandPath(cmd), &target, withChangePlanRoot(plan))
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: line, Detail: "configuration change applied but not saved"})
				if save {
					saveLines, err := client.ExecCLI(cmd.Context(), "save")
					if err != nil {
						addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
					}
					plan["saved"] = true
					plan["save_response_lines"] = saveLines
					env.Data = withChangePlanRoot(plan)
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
				}
				return env
			})
		},
	}
	set.Flags().BoolVar(&apply, "apply", false, "send the CLI-backed setting change")
	set.Flags().BoolVar(&save, "save", false, "persist after applying; requires --yes")
	cmd.AddCommand(set, a.settingsSetJSONCommand())
	return cmd
}
