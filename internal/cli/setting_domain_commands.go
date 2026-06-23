package cli

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

type settingDomain struct {
	use     string
	short   string
	matches func(settings.Metadata) bool
}

func (a *app) settingDomainCommand(domain settingDomain) *cobra.Command {
	cmd := &cobra.Command{Use: domain.use, Short: domain.short}
	switch domain.use {
	case "vtx":
		cmd.AddCommand(a.vtxListCommand())
	case "osd":
		cmd.AddCommand(a.osdListCommand())
	default:
		cmd.AddCommand(a.configListCommand("list", "List current "+domain.use+" settings", func(doc bfconfig.Document) any {
			return map[string]any{
				"settings": currentDomainSettings(doc, domain.matches),
				"metadata": settings.DefaultRegistry.Filter(domain.matches),
			}
		}))
	}
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set NAME VALUE",
		Short: "Plan or set one " + domain.use + " setting",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, value := args[0], args[1]
			metadata, ok := settings.DefaultRegistry.Lookup(name)
			if !ok || !domain.matches(metadata) {
				return a.render(output.Failure(commandPath(cmd), nil, "unknown_setting", fmt.Sprintf("%q is not a known %s setting", name, domain.use)))
			}
			if err := metadata.Validate(value); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{fmt.Sprintf("set %s = %s", metadata.Name, value)}, domain.use, flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set)
	if domain.use == "vtx" {
		cmd.AddCommand(a.vtxConfigCommand())
	}
	if domain.use == "receiver" {
		cmd.AddCommand(a.receiverStatusCommand())
		cmd.AddCommand(a.receiverRXFailCommand())
		cmd.AddCommand(a.receiverRSSIChannelCommand())
	}
	if domain.use == "gps" {
		cmd.AddCommand(a.gpsStatusCommand())
	}
	if domain.use == "osd" {
		cmd.AddCommand(a.osdStatusCommand())
	}
	if domain.use == "pid" {
		cmd.AddCommand(a.pidStatusCommand())
	}
	if domain.use == "rates" {
		cmd.AddCommand(a.ratesStatusCommand())
	}
	if domain.use == "filters" {
		cmd.AddCommand(a.filtersStatusCommand())
	}
	if domain.use == "battery" {
		cmd.AddCommand(a.batteryStatusCommand())
		cmd.AddCommand(a.batteryVoltageMeterCommand())
	}
	if domain.use == "failsafe" {
		cmd.AddCommand(a.failsafeStatusCommand())
		cmd.AddCommand(a.failsafeBoardAlignmentCommand())
	}
	return cmd
}

func (a *app) vtxListCommand() *cobra.Command {
	matches := vtxSettingMatch()
	return a.configListCommand("list", "List VTX settings and VTX table rows", func(doc bfconfig.Document) any {
		return map[string]any{
			"settings":  currentDomainSettings(doc, matches),
			"metadata":  settings.DefaultRegistry.Filter(matches),
			"vtx":       doc.VTX,
			"vtx_table": doc.VTXTable,
			"lines":     doc.Sections["vtx_table"],
		}
	})
}

func (a *app) osdListCommand() *cobra.Command {
	matches := osdSettingMatch()
	return a.configListCommand("list", "List OSD settings and OSD layout rows", func(doc bfconfig.Document) any {
		return map[string]any{
			"settings": currentDomainSettings(doc, matches),
			"metadata": settings.DefaultRegistry.Filter(matches),
			"osd":      doc.OSD,
			"lines":    doc.Sections["osd"],
		}
	})
}

func (a *app) receiverStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read receiver configuration and live channels over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				receiver, warnings, err := bfcommands.ReadReceiverStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"receiver": receiver,
					"warnings": warnings,
				})
			})
		},
	}
}

func vtxSettingMatch() func(settings.Metadata) bool {
	return func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "VTX") || strings.HasPrefix(s.Name, "vtx_")
	}
}

func (a *app) receiverRXFailCommand() *cobra.Command {
	var flags changeFlags
	cmd := &cobra.Command{
		Use:   "rxfail INDEX MODE [VALUE]",
		Short: "Plan or set one receiver failsafe channel row",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			line, err := receiverRXFailLine(args)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{line}, "rxfail", flags)
		},
	}
	addChangeFlags(cmd, &flags)
	return cmd
}

func (a *app) receiverRSSIChannelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rssi-channel CHANNEL",
		Short: "Set RSSI channel through MSP_SET_RSSI_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channel, err := strconv.Atoi(args[0])
			if err != nil || channel < 0 || channel > 18 {
				return validationFailureMessage(a, cmd, "channel must be an integer in [0..18]")
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "RSSI channel changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRSSIChannel(cmd.Context(), client, uint8(channel))
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rssi_channel": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rssi_channel",
					Command: "MSP_SET_RSSI_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func receiverRXFailLine(args []string) (string, error) {
	channel, err := strconv.Atoi(args[0])
	if err != nil || channel < 0 || channel >= 18 {
		return "", fmt.Errorf("channel index must be an integer in [0..17]")
	}
	mode := strings.ToLower(args[1])
	if mode != "a" && mode != "h" && mode != "s" {
		return "", fmt.Errorf("mode must be one of a, h, or s")
	}
	if channel >= 4 && mode == "a" {
		return "", fmt.Errorf("mode a is only valid for flight channels 0..3")
	}
	if mode == "s" {
		if len(args) != 3 {
			return "", fmt.Errorf("mode s requires a channel value")
		}
		if _, err := strconv.Atoi(args[2]); err != nil {
			return "", fmt.Errorf("value must be an integer")
		}
		return fmt.Sprintf("rxfail %d %s %s", channel, mode, args[2]), nil
	}
	if len(args) == 3 {
		return "", fmt.Errorf("value is only valid with mode s")
	}
	return fmt.Sprintf("rxfail %d %s", channel, mode), nil
}

func (a *app) gpsStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read GPS configuration, position, and Rescue state over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				gps, warnings, err := bfcommands.ReadGPSStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"gps":      gps,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) osdStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read OSD configuration, canvas, and active warning text over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				osd, warnings, err := bfcommands.ReadOSDStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"osd":      osd,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) pidStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read active PID gains, rate profile, and advanced tuning over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				pid, warnings, err := bfcommands.ReadPIDStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"pid":      pid,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) ratesStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read active rate profile and TPA settings over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				rates, warnings, err := bfcommands.ReadRateStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"rates":    rates,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) filtersStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read active filter and advanced loop configuration over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				filters, warnings, err := bfcommands.ReadFilterStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"filters":  filters,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) batteryStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read battery profile, runtime state, and meter configuration over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				battery, warnings, err := bfcommands.ReadBatteryStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"battery":  battery,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) batteryVoltageMeterCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-voltage-meter ID SCALE RES_DIV_VAL RES_DIV_MULTIPLIER",
		Short: "Set voltage meter calibration through MSP_SET_VOLTAGE_METER_CONFIG",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseUint8Arg("id", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			scale, err := parseUint8Arg("scale", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			resDiv, err := parseUint8Arg("res_div_val", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			multiplier, err := parseUint8Arg("res_div_multiplier", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "voltage meter calibration changes configuration; pass --yes"))
			}
			config := bfcommands.VoltageMeterConfig{
				ID:                   id,
				SensorType:           0,
				VBATScale:            scale,
				VBATResDivVal:        resDiv,
				VBATResDivMultiplier: multiplier,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetVoltageMeterConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"voltage_meter_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "voltage_meter_config",
					Command: "MSP_SET_VOLTAGE_METER_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) failsafeStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read failsafe, arming, and board-alignment safety state over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				safety, warnings, err := bfcommands.ReadSafetyStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"failsafe": safety,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) failsafeBoardAlignmentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-board-alignment ROLL_DEGREES PITCH_DEGREES YAW_DEGREES",
		Short: "Set board alignment through MSP_SET_BOARD_ALIGNMENT_CONFIG",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			roll, err := parseInt16Arg("roll_degrees", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			pitch, err := parseInt16Arg("pitch_degrees", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			yaw, err := parseInt16Arg("yaw_degrees", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "board alignment changes configuration; pass --yes"))
			}
			alignment := bfcommands.BoardAlignment{
				RollDegrees:  roll,
				PitchDegrees: pitch,
				YawDegrees:   yaw,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetBoardAlignment(cmd.Context(), client, alignment)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"board_alignment": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "board_alignment",
					Command: "MSP_SET_BOARD_ALIGNMENT_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func currentDomainSettings(doc bfconfig.Document, matches func(settings.Metadata) bool) []bfconfig.Setting {
	out := []bfconfig.Setting{}
	for _, setting := range doc.Settings {
		metadata, ok := settings.DefaultRegistry.Lookup(setting.Name)
		if ok && matches(metadata) {
			out = append(out, setting)
		}
	}
	return out
}

func pidDomain() settingDomain {
	return settingDomain{use: "pid", short: "Inspect and change PID settings", matches: func(s settings.Metadata) bool {
		name := s.Name
		if strings.HasPrefix(name, "p_") || strings.HasPrefix(name, "i_") || strings.HasPrefix(name, "d_") || strings.HasPrefix(name, "f_") {
			return true
		}
		return strings.Contains(s.PG, "PID") && !rateSettingName(name) && !filterSettingName(name)
	}}
}

func ratesDomain() settingDomain {
	return settingDomain{use: "rates", short: "Inspect and change rate settings", matches: func(s settings.Metadata) bool {
		return s.Scope == "rateprofile" || rateSettingName(s.Name)
	}}
}

func filtersDomain() settingDomain {
	return settingDomain{use: "filters", short: "Inspect and change filter settings", matches: func(s settings.Metadata) bool {
		return filterSettingName(s.Name)
	}}
}

func receiverDomain() settingDomain {
	return settingDomain{use: "receiver", short: "Inspect and change receiver settings", matches: func(s settings.Metadata) bool {
		name := s.Name
		return strings.Contains(s.PG, "RX") || strings.HasPrefix(name, "serialrx_") || strings.Contains(name, "deadband") || strings.Contains(name, "rssi")
	}}
}

func vtxDomain() settingDomain {
	return settingDomain{use: "vtx", short: "Inspect and change VTX settings", matches: func(s settings.Metadata) bool {
		return vtxSettingMatch()(s)
	}}
}

func osdDomain() settingDomain {
	return settingDomain{use: "osd", short: "Inspect and change OSD settings", matches: func(s settings.Metadata) bool {
		return osdSettingMatch()(s)
	}}
}

func osdSettingMatch() func(settings.Metadata) bool {
	return func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "OSD") || strings.HasPrefix(s.Name, "osd_")
	}
}

func gpsDomain() settingDomain {
	return settingDomain{use: "gps", short: "Inspect and change GPS settings", matches: func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "GPS") || strings.HasPrefix(s.Name, "gps_")
	}}
}

func batteryDomain() settingDomain {
	return settingDomain{use: "battery", short: "Inspect and change battery and power meter settings", matches: func(s settings.Metadata) bool {
		name := s.Name
		return strings.Contains(s.PG, "BATTERY") ||
			strings.Contains(s.PG, "VOLTAGE") ||
			strings.Contains(s.PG, "CURRENT") ||
			s.Scope == "batteryprofile" ||
			strings.HasPrefix(name, "vbat_") ||
			strings.Contains(name, "battery") ||
			strings.Contains(name, "current_meter") ||
			strings.Contains(name, "battery_meter") ||
			strings.Contains(name, "bat_capacity") ||
			strings.Contains(name, "cbat_")
	}}
}

func failsafeDomain() settingDomain {
	return settingDomain{use: "failsafe", short: "Inspect and change failsafe settings", matches: func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "FAILSAFE") || strings.HasPrefix(s.Name, "failsafe_")
	}}
}

func rateSettingName(name string) bool {
	return strings.Contains(name, "rate") || strings.Contains(name, "expo") || strings.HasPrefix(name, "thr_") || strings.HasPrefix(name, "tpa_")
}

func filterSettingName(name string) bool {
	return strings.Contains(name, "lpf") ||
		strings.Contains(name, "notch") ||
		strings.Contains(name, "filter") ||
		strings.HasPrefix(name, "dyn_notch") ||
		strings.HasPrefix(name, "rpm_filter")
}
