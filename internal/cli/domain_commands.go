package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/batch"
	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

type changeFlags struct {
	apply bool
	save  bool
}

func (a *app) featuresCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "features", Short: "Inspect and change Betaflight features"}
	cmd.AddCommand(a.configListCommand("list", "List configured features", func(doc bfconfig.Document) any {
		return map[string]any{"features": doc.Features}
	}))
	var enableFlags changeFlags
	enable := &cobra.Command{
		Use:   "enable NAME",
		Short: "Plan or enable a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.ToUpper(args[0])
			return a.planOrApplyCLI(cmd, []string{"feature " + name}, "feature", enableFlags)
		},
	}
	addChangeFlags(enable, &enableFlags)
	var disableFlags changeFlags
	disable := &cobra.Command{
		Use:   "disable NAME",
		Short: "Plan or disable a feature",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := strings.ToUpper(args[0])
			return a.planOrApplyCLI(cmd, []string{"feature -" + name}, "feature", disableFlags)
		},
	}
	addChangeFlags(disable, &disableFlags)
	cmd.AddCommand(enable, disable)
	return cmd
}

func (a *app) serialCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "serial", Short: "Inspect and change serial port configuration"}
	cmd.AddCommand(a.configListCommand("list", "List serial CLI rows", func(doc bfconfig.Document) any {
		return map[string]any{"serial": doc.Serial, "lines": doc.Sections["serial"]}
	}))
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set PORT FUNCTION_MASK MSP_BAUD GPS_BAUD TELEMETRY_BAUD BLACKBOX_BAUD",
		Short: "Plan or set one serial CLI row",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			line := "serial " + strings.Join(args, " ")
			return a.planOrApplyCLI(cmd, []string{line}, "serial", flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set)
	return cmd
}

func (a *app) modesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "modes", Short: "Inspect and change AUX mode ranges"}
	cmd.AddCommand(a.configListCommand("list", "List AUX mode ranges and mode color commands", func(doc bfconfig.Document) any {
		return map[string]any{"aux": doc.Aux, "lines": doc.Sections["modes"]}
	}))
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set INDEX MODE_ID CHANNEL RANGE_START RANGE_END [EXTRA...]",
		Short: "Plan or set one AUX range",
		Args:  cobra.MinimumNArgs(5),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args[:5]); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			line := "aux " + strings.Join(args, " ")
			return a.planOrApplyCLI(cmd, []string{line}, "aux", flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set)
	return cmd
}

func (a *app) resourcesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "resources", Short: "Inspect and change resource assignments"}
	cmd.AddCommand(a.configListCommand("list", "List resource assignments", func(doc bfconfig.Document) any {
		return map[string]any{"resources": doc.Resources, "lines": doc.Sections["resources"]}
	}))
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set KIND INDEX TARGET",
		Short: "Plan or set one resource assignment",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			line := "resource " + strings.ToUpper(args[0]) + " " + args[1] + " " + strings.ToUpper(args[2])
			return a.planOrApplyCLI(cmd, []string{line}, "resource", flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set)
	return cmd
}

func (a *app) profilesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "profiles", Short: "Inspect and select PID and rate profiles"}
	cmd.AddCommand(a.configListCommand("list", "List profile selectors found in dump output", func(doc bfconfig.Document) any {
		return map[string]any{"profiles": filterProfiles(doc.Profiles, "profile"), "rateprofiles": filterProfiles(doc.Profiles, "rateprofile"), "lines": doc.Sections["profiles"]}
	}))
	var profileFlags changeFlags
	profile := &cobra.Command{
		Use:   "select INDEX",
		Short: "Plan or select PID profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"profile " + args[0]}, "profile", profileFlags)
		},
	}
	addChangeFlags(profile, &profileFlags)
	var rateFlags changeFlags
	rate := &cobra.Command{
		Use:   "rate-select INDEX",
		Short: "Plan or select rate profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"rateprofile " + args[0]}, "rateprofile", rateFlags)
		},
	}
	addChangeFlags(rate, &rateFlags)
	cmd.AddCommand(profile, rate)
	return cmd
}

func (a *app) rateprofilesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "rateprofiles", Short: "Inspect and select rate profiles"}
	cmd.AddCommand(a.configListCommand("list", "List rate profile selectors found in dump output", func(doc bfconfig.Document) any {
		return map[string]any{"rateprofiles": filterProfiles(doc.Profiles, "rateprofile"), "lines": doc.Sections["profiles"]}
	}))
	var flags changeFlags
	selectCmd := &cobra.Command{
		Use:   "select INDEX",
		Short: "Plan or select rate profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"rateprofile " + args[0]}, "rateprofile", flags)
		},
	}
	addChangeFlags(selectCmd, &flags)
	cmd.AddCommand(selectCmd)
	return cmd
}

func (a *app) batchCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "batch", Short: "Plan and apply multi-line Betaflight CLI changes"}
	var planFile string
	planCmd := &cobra.Command{
		Use:   "plan",
		Short: "Validate and print a batch change plan without connecting",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := a.readBatchPlan(planFile)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateBatchPlan(cmd, plan)
			if !ok {
				return a.render(env)
			}
			return a.render(output.Success(commandPath(cmd), nil, batchPlanData(plan, false)))
		},
	}
	planCmd.Flags().StringVar(&planFile, "file", "-", "batch plan file path or - for stdin")

	var applyFile string
	var save bool
	applyCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a validated batch change plan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := a.readBatchPlan(applyFile)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			if plan.Save {
				save = true
			}
			env, ok := a.validateBatchPlan(cmd, plan)
			if !ok {
				return a.render(env)
			}
			return a.planOrApplyCLI(cmd, plan.CLILines, plan.Kind, changeFlags{apply: true, save: save})
		},
	}
	applyCmd.Flags().StringVar(&applyFile, "file", "-", "batch plan file path or - for stdin")
	applyCmd.Flags().BoolVar(&save, "save", false, "persist after applying; requires --yes")
	cmd.AddCommand(planCmd, applyCmd)
	return cmd
}

func filterProfiles(profiles []bfconfig.Profile, kind string) []bfconfig.Profile {
	out := []bfconfig.Profile{}
	for _, profile := range profiles {
		if profile.Kind == kind {
			out = append(out, profile)
		}
	}
	return out
}

func (a *app) readBatchPlan(path string) (batch.Plan, error) {
	var reader io.Reader
	if path == "" || path == "-" {
		reader = a.in
		if reader == nil {
			reader = os.Stdin
		}
	} else {
		file, err := os.Open(path)
		if err != nil {
			return batch.Plan{}, err
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return batch.Plan{}, err
	}
	return batch.Parse(data)
}

func (a *app) validateBatchPlan(cmd *cobra.Command, plan batch.Plan) (output.Envelope, bool) {
	if len(plan.CLILines) == 0 {
		return output.Failure(commandPath(cmd), nil, "validation_error", "batch plan has no CLI lines"), false
	}
	for _, line := range plan.CLILines {
		class := classifyCLI(line)
		if class == cliReadOnly {
			return output.Failure(commandPath(cmd), nil, "validation_error", fmt.Sprintf("%q is read-only and does not belong in a change batch", line)), false
		}
		if class == cliDangerous {
			return output.Failure(commandPath(cmd), nil, "dangerous_action_blocked", fmt.Sprintf("%q is dangerous and cannot be run through batch apply", line)), false
		}
		if !isBatchAllowed(line) {
			return output.Failure(commandPath(cmd), nil, "validation_error", fmt.Sprintf("%q is not a supported batch configuration command", line)), false
		}
		if err := validateSetLine(line); err != nil {
			return output.Failure(commandPath(cmd), nil, "validation_error", err.Error()), false
		}
	}
	return output.Envelope{}, true
}

func isBatchAllowed(line string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(line)))
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "set", "feature", "serial", "aux", "resource", "profile", "rateprofile", "vtxtable", "mode_color", "color", "led", "servo", "smix", "adjrange", "rxrange":
		return true
	default:
		return false
	}
}

func validateSetLine(line string) error {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(strings.ToLower(trimmed), "set ") {
		return nil
	}
	body := strings.TrimSpace(trimmed[4:])
	parts := strings.SplitN(body, "=", 2)
	if len(parts) != 2 {
		return fmt.Errorf("%q is not a valid set line", line)
	}
	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	metadata, ok := settings.DefaultRegistry.Lookup(name)
	if !ok {
		return nil
	}
	if err := metadata.Validate(value); err != nil {
		return err
	}
	return nil
}

func batchPlanData(plan batch.Plan, applied bool) map[string]any {
	return map[string]any{
		"schema_version": plan.SchemaVersion,
		"kind":           plan.Kind,
		"cli_lines":      plan.CLILines,
		"source_format":  plan.SourceFormat,
		"save_requested": plan.Save,
		"applied":        applied,
		"saved":          false,
	}
}

func (a *app) configListCommand(use, short string, selectData func(bfconfig.Document) any) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withConfiguration(cmd.Context(), commandPath(cmd), func(target output.Target, doc bfconfig.Document, lines []string) output.Envelope {
				return output.Success(commandPath(cmd), &target, map[string]any{
					"source_command":    "dump all",
					"view":              selectData(doc),
					"configuration":     doc,
					"raw":               strings.Join(lines, "\n"),
					"raw_authoritative": true,
				})
			})
		},
	}
}

func (a *app) withConfiguration(ctx context.Context, command string, fn func(output.Target, bfconfig.Document, []string) output.Envelope) error {
	return a.withClient(ctx, command, connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
		lines, err := client.ExecCLI(ctx, "dump all")
		if err != nil {
			return a.failure(command, &target, err)
		}
		return fn(target, bfconfig.Parse(lines, settings.DefaultRegistry), lines)
	})
}

func addChangeFlags(cmd *cobra.Command, flags *changeFlags) {
	cmd.Flags().BoolVar(&flags.apply, "apply", false, "send the CLI-backed change")
	cmd.Flags().BoolVar(&flags.save, "save", false, "persist after applying; requires --yes")
}

func (a *app) planOrApplyCLI(cmd *cobra.Command, lines []string, kind string, flags changeFlags) error {
	plan := map[string]any{
		"kind":      kind,
		"cli_lines": lines,
		"applied":   false,
		"saved":     false,
	}
	if !flags.apply {
		return a.render(output.Success(commandPath(cmd), nil, plan))
	}
	if flags.save && !a.opts.yes {
		return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--save requires --yes because it persists and usually reboots the flight controller"))
	}
	return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
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
}

func requireInts(values []string) error {
	for _, value := range values {
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("%q must be an integer", value)
		}
	}
	return nil
}

func validationFailure(a *app, cmd *cobra.Command, err error) error {
	return validationFailureMessage(a, cmd, err.Error())
}

func validationFailureMessage(a *app, cmd *cobra.Command, message string) error {
	return a.render(output.Failure(commandPath(cmd), nil, "validation_error", message))
}
