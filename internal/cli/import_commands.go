package cli

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/batch"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

type importCommandOptions struct {
	file            string
	includeDefaults bool
	save            bool
}

func (a *app) restoreCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "restore", Short: "Plan and apply restore files from Betaflight CLI output"}
	cmd.AddCommand(a.importPlanCommand("plan", "Validate and print a restore plan without connecting", "restore", "restore_text"))
	cmd.AddCommand(a.importApplyCommand("apply", "Apply a validated restore plan", "restore", "restore_text"))
	return cmd
}

func (a *app) presetsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "presets", Short: "Plan and apply local Betaflight preset files"}
	cmd.AddCommand(a.importPlanCommand("plan", "Validate and print a local preset plan without connecting", "preset", "preset_text"))
	cmd.AddCommand(a.importApplyCommand("apply", "Apply a local preset plan", "preset", "preset_text"))
	return cmd
}

func (a *app) importPlanCommand(use, short, kind, sourceFormat string) *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			imported, err := a.readImportPlan(opts, kind, sourceFormat)
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

func (a *app) importApplyCommand(use, short, kind, sourceFormat string) *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.includeDefaults && !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--include-defaults requires --yes because defaults nosave resets configuration before applying lines"))
			}
			imported, err := a.readImportPlan(opts, kind, sourceFormat)
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

func addImportFlags(cmd *cobra.Command, opts *importCommandOptions, apply bool) {
	cmd.Flags().StringVar(&opts.file, "file", "-", "restore or preset file path, or - for stdin")
	cmd.Flags().BoolVar(&opts.includeDefaults, "include-defaults", false, "include exact defaults nosave lines from the import file; apply requires --yes")
	if apply {
		cmd.Flags().BoolVar(&opts.save, "save", false, "persist after applying; requires --yes")
	}
}

func (a *app) readImportPlan(opts importCommandOptions, kind, sourceFormat string) (batch.ImportResult, error) {
	data, err := a.readInput(opts.file)
	if err != nil {
		return batch.ImportResult{}, err
	}
	return batch.ImportCLI(data, batch.ImportOptions{
		Kind:            kind,
		SourceFormat:    sourceFormat,
		IncludeDefaults: opts.includeDefaults,
	})
}

func (a *app) applyImportPlan(cmd *cobra.Command, imported batch.ImportResult, opts importCommandOptions, op connection.OperationClass) error {
	if opts.save && !a.opts.yes {
		return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--save requires --yes because it persists and usually reboots the flight controller"))
	}
	if opts.save {
		op = connection.Dangerous
	}
	data := importPlanData(imported, opts, true)
	return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
		responses := map[string][]string{}
		for _, line := range imported.Plan.CLILines {
			responseLines, err := client.ExecCLI(cmd.Context(), line)
			if err != nil {
				return a.failure(commandPath(cmd), &target, err)
			}
			responses[line] = responseLines
		}
		data["response_lines"] = responses
		env := output.Success(commandPath(cmd), &target, data)
		for _, line := range imported.Plan.CLILines {
			env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: line, Detail: "configuration change applied but not saved"})
		}
		if opts.save {
			saveLines, err := client.ExecCLI(cmd.Context(), "save")
			if err != nil {
				addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
			}
			data["saved"] = true
			data["save_response_lines"] = saveLines
			env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
		}
		return env
	})
}

func importPlanData(imported batch.ImportResult, opts importCommandOptions, applied bool) map[string]any {
	return map[string]any{
		"schema_version":    imported.Plan.SchemaVersion,
		"kind":              imported.Plan.Kind,
		"cli_lines":         imported.Plan.CLILines,
		"source_format":     imported.Plan.SourceFormat,
		"include_defaults":  opts.includeDefaults,
		"save_requested":    opts.save,
		"applied":           applied,
		"saved":             false,
		"skipped_lines":     imported.Skipped,
		"raw_authoritative": true,
	}
}
