package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/batch"
	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
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
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read active feature mask over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				features, err := bfcommands.ReadFeatureStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"features": features,
				})
			})
		},
	})
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
	cmd.AddCommand(enable, disable, a.featureSetMaskCommand())
	return cmd
}

func (a *app) featureSetMaskCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-mask MASK",
		Short: "Set the complete feature mask through MSP_SET_FEATURE_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			parsed, err := strconv.ParseUint(args[0], 0, 32)
			if err != nil {
				return validationFailureMessage(a, cmd, "mask must be an unsigned 32-bit integer")
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "feature mask changes configuration; pass --yes"))
			}
			mask := uint32(parsed)
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetFeatureMask(cmd.Context(), client, mask)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"feature_mask": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "feature_mask",
					Command: "MSP_SET_FEATURE_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) serialCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "serial", Short: "Inspect and change serial port configuration"}
	cmd.AddCommand(a.configListCommand("list", "List serial CLI rows", func(doc bfconfig.Document) any {
		return map[string]any{"serial": doc.Serial, "lines": doc.Sections["serial"]}
	}))
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read serial port configuration over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				serial, warnings, err := bfcommands.ReadSerialPortStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"serial":   serial,
					"warnings": warnings,
				})
			})
		},
	})
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
	cmd.AddCommand(set, a.serialApplyConfigJSONCommand())
	return cmd
}

func (a *app) serialApplyConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "apply-config-json FILE",
		Short: "Apply a complete serial port table from JSON through MSP_SET_CF_SERIAL_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			ports, err := parseSerialPortConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "serial port configuration changes port settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetSerialPortConfig(cmd.Context(), client, ports)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"serial_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "serial_config",
					Command: "MSP_SET_CF_SERIAL_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseSerialPortConfigJSON(data []byte) ([]bfcommands.SerialPort, error) {
	var ports []bfcommands.SerialPort
	if err := json.Unmarshal(data, &ports); err != nil {
		var wrapped struct {
			Ports []bfcommands.SerialPort `json:"ports"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return nil, err
		}
		ports = wrapped.Ports
	}
	if len(ports) == 0 {
		return nil, fmt.Errorf("serial config must include at least one port")
	}
	for i, port := range ports {
		if port.FunctionMask > 0xffff {
			return nil, fmt.Errorf("ports[%d].function_mask exceeds legacy MSP_SET_CF_SERIAL_CONFIG maximum 65535", i)
		}
	}
	return ports, nil
}

func (a *app) modesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "modes", Short: "Inspect and change AUX mode ranges"}
	cmd.AddCommand(a.configListCommand("list", "List AUX mode ranges and mode color commands", func(doc bfconfig.Document) any {
		return map[string]any{"aux": doc.Aux, "lines": doc.Sections["modes"]}
	}))
	cmd.AddCommand(&cobra.Command{
		Use:   "active",
		Short: "Read active mode definitions and ranges over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				modes, warnings, err := bfcommands.ReadModeConfiguration(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"modes":    modes,
					"warnings": warnings,
				})
			})
		},
	})
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
	cmd.AddCommand(set, a.modeSetJSONCommand(), a.modeSetRangeCommand())
	return cmd
}

func (a *app) modeSetJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-json FILE",
		Short: "Set AUX mode range rows from JSON through MSP_SET_MODE_RANGE",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseModeRangeTableJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "mode range changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetModeRangeTable(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"mode_ranges": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "mode_ranges",
					Command: "MSP_SET_MODE_RANGE",
					Detail:  "mode range rows changed but not saved",
				})
				return env
			})
		},
	}
}

func parseModeRangeTableJSON(data []byte) (bfcommands.ModeRangeTableSetConfig, error) {
	if isJSONArray(data) {
		var ranges []bfcommands.ModeRange
		if err := json.Unmarshal(data, &ranges); err != nil {
			return bfcommands.ModeRangeTableSetConfig{}, err
		}
		return validateModeRangeTableConfig(bfcommands.ModeRangeTableSetConfig{Ranges: ranges})
	}
	var wrapped struct {
		ModeRanges *bfcommands.ModeRangeTableSetConfig `json:"mode_ranges"`
		Modes      *bfcommands.ModeRangeTableSetConfig `json:"modes"`
		Ranges     []bfcommands.ModeRange              `json:"ranges"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.ModeRangeTableSetConfig{}, err
	}
	if wrapped.ModeRanges != nil {
		return validateModeRangeTableConfig(*wrapped.ModeRanges)
	}
	if wrapped.Modes != nil {
		return validateModeRangeTableConfig(*wrapped.Modes)
	}
	if wrapped.Ranges != nil {
		return validateModeRangeTableConfig(bfcommands.ModeRangeTableSetConfig{Ranges: wrapped.Ranges})
	}
	return bfcommands.ModeRangeTableSetConfig{}, fmt.Errorf("expected mode_ranges, modes, ranges, or a direct array of mode range rows")
}

func isJSONArray(data []byte) bool {
	for _, b := range data {
		switch b {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			return b == '['
		}
	}
	return false
}

func validateModeRangeTableConfig(config bfcommands.ModeRangeTableSetConfig) (bfcommands.ModeRangeTableSetConfig, error) {
	if err := bfcommands.ValidateModeRangeTable(config); err != nil {
		return bfcommands.ModeRangeTableSetConfig{}, err
	}
	return config, nil
}

func (a *app) modeSetRangeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-range INDEX MODE_ID AUX_CHANNEL START_STEP END_STEP [LOGIC LINKED_TO]",
		Short: "Set one AUX mode range over MSP",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 5 && len(args) != 7 {
				return fmt.Errorf("accepts 5 or 7 arg(s), received %d", len(args))
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			modeID, err := parseUint8Arg("mode_id", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			aux, err := parseUint8Arg("aux_channel", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			start, err := parseUint8Arg("start_step", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			end, err := parseUint8Arg("end_step", args[4])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			row := bfcommands.ModeRange{
				Index:           int(index),
				ID:              modeID,
				AuxChannelIndex: aux,
				Range: bfcommands.StepRange{
					StartStep: start,
					EndStep:   end,
				},
			}
			if len(args) == 7 {
				logic, err := parseUint8Arg("logic", args[5])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				linkedTo, err := parseUint8Arg("linked_to", args[6])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				row.ModeLogic = &logic
				row.LinkedTo = &linkedTo
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "mode range changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetModeRange(cmd.Context(), client, row)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"mode_range": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "mode_range",
					Command: "MSP_SET_MODE_RANGE",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) resourcesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "resources", Short: "Inspect and change resource assignments"}
	cmd.AddCommand(a.configListCommand("list", "List resource assignments", func(doc bfconfig.Document) any {
		return map[string]any{"resources": doc.Resources, "lines": doc.Sections["resources"]}
	}))
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read resource, timer, and DMA assignments through Betaflight CLI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				resources, err := bfcommands.ReadResourceDiagnostics(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"resources": resources,
				})
			})
		},
	})
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
	cmd.AddCommand(set, a.resourcesSetJSONCommand())
	return cmd
}

type resourceSetRow struct {
	Kind   string `json:"kind"`
	Index  int    `json:"index"`
	Target string `json:"target"`
}

func (a *app) resourcesSetJSONCommand() *cobra.Command {
	var flags changeFlags
	cmd := &cobra.Command{
		Use:   "set-json FILE",
		Short: "Plan or set resource assignments from JSON using native Betaflight CLI syntax",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			rows, err := parseResourceRowsJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			lines := make([]string, 0, len(rows))
			for _, row := range rows {
				lines = append(lines, fmt.Sprintf("resource %s %d %s", strings.ToUpper(row.Kind), row.Index, strings.ToUpper(row.Target)))
			}
			return a.planOrApplyCLI(cmd, lines, "resource", flags)
		},
	}
	addChangeFlags(cmd, &flags)
	return cmd
}

func parseResourceRowsJSON(data []byte) ([]resourceSetRow, error) {
	var wrapped struct {
		Resource  *resourceSetRow  `json:"resource"`
		Row       *resourceSetRow  `json:"row"`
		Resources []resourceSetRow `json:"resources"`
		Rows      []resourceSetRow `json:"rows"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	switch {
	case wrapped.Resource != nil:
		return validateResourceRows([]resourceSetRow{*wrapped.Resource})
	case wrapped.Row != nil:
		return validateResourceRows([]resourceSetRow{*wrapped.Row})
	case wrapped.Resources != nil:
		return validateResourceRows(wrapped.Resources)
	case wrapped.Rows != nil:
		return validateResourceRows(wrapped.Rows)
	}
	var rows []resourceSetRow
	if err := json.Unmarshal(data, &rows); err == nil {
		return validateResourceRows(rows)
	}
	var row resourceSetRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}
	return validateResourceRows([]resourceSetRow{row})
}

func validateResourceRows(rows []resourceSetRow) ([]resourceSetRow, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("at least one resource row is required")
	}
	for i, row := range rows {
		if strings.TrimSpace(row.Kind) == "" {
			return nil, fmt.Errorf("row %d kind is required", i)
		}
		if strings.ContainsAny(row.Kind, " \t\r\n") {
			return nil, fmt.Errorf("row %d kind must be a single CLI token", i)
		}
		if row.Index < 0 {
			return nil, fmt.Errorf("row %d index must be >= 0", i)
		}
		if strings.TrimSpace(row.Target) == "" {
			return nil, fmt.Errorf("row %d target is required", i)
		}
		if strings.ContainsAny(row.Target, " \t\r\n") {
			return nil, fmt.Errorf("row %d target must be a single CLI token", i)
		}
	}
	return rows, nil
}

func (a *app) profilesCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "profiles", Short: "Inspect and select PID and rate profiles"}
	cmd.AddCommand(a.configListCommand("list", "List profile selectors found in dump output", func(doc bfconfig.Document) any {
		return map[string]any{"profiles": filterProfiles(doc.Profiles, "profile"), "rateprofiles": filterProfiles(doc.Profiles, "rateprofile"), "lines": doc.Sections["profiles"]}
	}))
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read active PID, rate, and battery profile selections over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				profiles, err := bfcommands.ReadProfileStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"profiles": profiles,
				})
			})
		},
	})
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
	var batteryFlags changeFlags
	battery := &cobra.Command{
		Use:   "battery-select INDEX",
		Short: "Plan or select battery profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"battery_profile " + args[0]}, "battery_profile", batteryFlags)
		},
	}
	addChangeFlags(battery, &batteryFlags)
	copyProfile := &cobra.Command{
		Use:   "copy KIND SOURCE DESTINATION",
		Short: "Copy PID or rate profile using MSP_COPY_PROFILE",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			kind, err := parseProfileCopyKind(args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			source, err := parseUint8Arg("source", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			destination, err := parseUint8Arg("destination", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "profile copy changes configuration; pass --yes"))
			}
			request := bfcommands.ProfileCopyRequest{
				Kind:        kind,
				Source:      source,
				Destination: destination,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.CopyProfile(cmd.Context(), client, request)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"profile_copy": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "profile_copy",
					Command: "MSP_COPY_PROFILE",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
	cmd.AddCommand(profile, rate, battery, copyProfile)
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
	data, err := a.readInput(path)
	if err != nil {
		return batch.Plan{}, err
	}
	return batch.Parse(data)
}

func (a *app) readInput(path string) ([]byte, error) {
	var reader io.Reader
	if path == "" || path == "-" {
		reader = a.in
		if reader == nil {
			reader = os.Stdin
		}
	} else {
		file, err := os.Open(path)
		if err != nil {
			return nil, err
		}
		defer file.Close()
		reader = file
	}
	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func (a *app) validateBatchPlan(cmd *cobra.Command, plan batch.Plan) (output.Envelope, bool) {
	return a.validateChangePlan(cmd, plan, planValidationOptions{})
}

type planValidationOptions struct {
	allowDefaultsNoSave bool
}

func (a *app) validateChangePlan(cmd *cobra.Command, plan batch.Plan, opts planValidationOptions) (output.Envelope, bool) {
	if len(plan.CLILines) == 0 {
		return output.Failure(commandPath(cmd), nil, "validation_error", "batch plan has no CLI lines"), false
	}
	for _, line := range plan.CLILines {
		if opts.allowDefaultsNoSave && isDefaultsNoSave(line) {
			continue
		}
		if isBatchAllowed(line) {
			if err := validateSetLine(line); err != nil {
				return output.Failure(commandPath(cmd), nil, "validation_error", err.Error()), false
			}
			continue
		}
		class := classifyCLI(line)
		if !isKnownCLICommand(line) {
			return output.Failure(commandPath(cmd), nil, "validation_error", fmt.Sprintf("%q is unknown and cannot be planned", line)), false
		}
		if class == cliReadOnly {
			return output.Failure(commandPath(cmd), nil, "validation_error", fmt.Sprintf("%q is read-only and does not belong in a change batch", line)), false
		}
		if class == cliDangerous {
			return output.Failure(commandPath(cmd), nil, "dangerous_action_blocked", fmt.Sprintf("%q is dangerous and cannot be run through batch apply", line)), false
		}
	}
	return output.Envelope{}, true
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

func isDefaultsNoSave(line string) bool {
	return strings.EqualFold(strings.TrimSpace(line), "defaults nosave")
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
	return a.planOrApplyCLIWithOperation(cmd, lines, kind, flags, connection.Write)
}

func (a *app) planOrApplyCLIWithOperation(cmd *cobra.Command, lines []string, kind string, flags changeFlags, op connection.OperationClass) error {
	plan := map[string]any{
		"kind":      kind,
		"cli_lines": lines,
		"applied":   false,
		"saved":     false,
	}
	if len(lines) == 1 {
		plan["command_preview"] = lines[0]
	}
	if !flags.apply {
		return a.render(output.Success(commandPath(cmd), nil, plan))
	}
	if !a.opts.yes {
		return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "apply requires --yes"))
	}
	if flags.save && !a.opts.yes {
		return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--save requires --yes because it persists and usually reboots the flight controller"))
	}
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
}

func requireInts(values []string) error {
	for _, value := range values {
		if _, err := strconv.Atoi(value); err != nil {
			return fmt.Errorf("%q must be an integer", value)
		}
	}
	return nil
}

func parseInt16Arg(name, value string) (int16, error) {
	parsed, err := strconv.ParseInt(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%s must be a signed 16-bit integer", name)
	}
	return int16(parsed), nil
}

func parseUint8Arg(name, value string) (uint8, error) {
	parsed, err := strconv.ParseUint(value, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned 8-bit integer", name)
	}
	return uint8(parsed), nil
}

func parseBoolArg(name, value string) (bool, error) {
	switch strings.ToLower(value) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, fmt.Errorf("%s must be a boolean", name)
	}
}

func parseInt8Arg(name, value string) (int8, error) {
	parsed, err := strconv.ParseInt(value, 10, 8)
	if err != nil {
		return 0, fmt.Errorf("%s must be a signed 8-bit integer", name)
	}
	return int8(parsed), nil
}

func parseUint16Arg(name, value string) (uint16, error) {
	parsed, err := strconv.ParseUint(value, 10, 16)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned 16-bit integer", name)
	}
	return uint16(parsed), nil
}

func parseUint32Arg(name, value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 10, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned 32-bit integer", name)
	}
	return uint32(parsed), nil
}

func parseUint32FlexibleArg(name, value string) (uint32, error) {
	parsed, err := strconv.ParseUint(value, 0, 32)
	if err != nil {
		return 0, fmt.Errorf("%s must be an unsigned 32-bit integer", name)
	}
	return uint32(parsed), nil
}

func parseProfileCopyKind(value string) (bfcommands.ProfileCopyKind, error) {
	switch strings.ToLower(value) {
	case string(bfcommands.ProfileCopyPID):
		return bfcommands.ProfileCopyPID, nil
	case string(bfcommands.ProfileCopyRate):
		return bfcommands.ProfileCopyRate, nil
	default:
		return "", fmt.Errorf("kind must be one of: pid, rate")
	}
}

func validationFailure(a *app, cmd *cobra.Command, err error) error {
	return validationFailureMessage(a, cmd, err.Error())
}

func validationFailureMessage(a *app, cmd *cobra.Command, message string) error {
	return a.render(output.Failure(commandPath(cmd), nil, "validation_error", message))
}
