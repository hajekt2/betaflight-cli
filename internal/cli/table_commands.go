package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) vtxTableCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "vtxtable", Short: "Inspect and change VTX table CLI rows"}
	cmd.AddCommand(a.configListCommand("list", "List VTX table rows", func(doc bfconfig.Document) any {
		return map[string]any{"vtx_table": doc.VTXTable, "vtx": doc.VTX, "lines": doc.Sections["vtx_table"]}
	}))
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set ARGS...",
		Short: "Plan or set one vtxtable row using Betaflight CLI syntax",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.planOrApplyCLI(cmd, []string{"vtxtable " + strings.Join(args, " ")}, "vtxtable", flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set, a.vtxTableSetBandCommand(), a.vtxTableSetPowerCommand())
	return cmd
}

func (a *app) vtxTableSetBandCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-band BAND NAME LETTER FACTORY FREQ_MHZ...",
		Short: "Set one VTX table band over MSP",
		Args:  cobra.RangeArgs(5, 12),
		RunE: func(cmd *cobra.Command, args []string) error {
			band, err := parseUint8Arg("band", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			name := strings.ToUpper(args[1])
			if len(name) == 0 || len(name) > 8 {
				return validationFailure(a, cmd, fmt.Errorf("name must be 1-8 bytes"))
			}
			letter := strings.ToUpper(args[2])
			if len(letter) != 1 {
				return validationFailure(a, cmd, fmt.Errorf("letter must be exactly 1 byte"))
			}
			factory, err := parseBoolArg("factory", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			frequencies := make([]uint16, 0, len(args)-4)
			for i, value := range args[4:] {
				frequency, err := parseUint16Arg("frequency_mhz", value)
				if err != nil {
					return validationFailure(a, cmd, fmt.Errorf("frequency %d: %v", i+1, err))
				}
				frequencies = append(frequencies, frequency)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "VTX table band changes can affect RF channel mappings; pass --yes"))
			}
			config := bfcommands.VTXTableBandSetConfig{
				Band:           band,
				Name:           name,
				Letter:         letter,
				Factory:        factory,
				FrequenciesMHz: frequencies,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetVTXTableBand(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"vtxtable_band": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "vtxtable_band", Command: "MSP_SET_VTXTABLE_BAND", Detail: "VTX table band changed but not saved"})
				return env
			})
		},
	}
}

func (a *app) vtxTableSetPowerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-power LEVEL VALUE LABEL",
		Short: "Set one VTX table power level over MSP",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			level, err := parseUint8Arg("level", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			value, err := parseUint16Arg("value", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			label := strings.ToUpper(args[2])
			if len(label) == 0 || len(label) > 3 {
				return validationFailure(a, cmd, fmt.Errorf("label must be 1-3 bytes"))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "VTX table power changes can affect RF output; pass --yes"))
			}
			config := bfcommands.VTXTablePowerSetConfig{Level: level, Value: value, Label: label}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetVTXTablePower(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"vtxtable_power": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "vtxtable_power", Command: "MSP_SET_VTXTABLE_POWERLEVEL", Detail: "VTX table power level changed but not saved"})
				return env
			})
		},
	}
}

func (a *app) ledsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "leds", Short: "Inspect and change LED strip rows"}
	cmd.AddCommand(a.configListCommand("list", "List LED strip rows", func(doc bfconfig.Document) any {
		return map[string]any{"leds": doc.LEDs, "lines": doc.Sections["leds"]}
	}))
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read LED strip layout and colors over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				leds, warnings, err := bfcommands.ReadLEDStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"leds":     leds,
					"warnings": warnings,
				})
			})
		},
	})
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set INDEX CONFIG",
		Short: "Plan or set one LED strip row",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args[:1]); err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.planOrApplyCLI(cmd, []string{"led " + strings.Join(args, " ")}, "led", flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set, a.ledValuesCommand())
	return cmd
}

func (a *app) ledValuesCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-values BRIGHTNESS RAINBOW_DELTA RAINBOW_FREQ",
		Short: "Set LED strip brightness and rainbow values over MSP2",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			brightness, err := parseUint8Arg("brightness", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			rainbowDelta, err := parseUint16Arg("rainbow_delta", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			rainbowFreq, err := parseUint16Arg("rainbow_freq", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "LED values change configuration; pass --yes"))
			}
			values := bfcommands.LEDConfigValues{
				Brightness:   brightness,
				RainbowDelta: rainbowDelta,
				RainbowFreq:  rainbowFreq,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetLEDConfigValues(cmd.Context(), client, values)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"led_values": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "led_values",
					Command: "MSP2_SET_LED_STRIP_CONFIG_VALUES",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) servosCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "servos", Short: "Inspect and change servo rows"}
	cmd.AddCommand(a.configListCommand("list", "List servo rows and reverse mixer rows", func(doc bfconfig.Document) any {
		return map[string]any{"servos": doc.Servos, "smix": doc.SMix, "lines": append(doc.Sections["servos"], doc.Sections["smix"]...)}
	}))
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read servo outputs, configurations, and mix rules over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				servos, warnings, err := bfcommands.ReadServoStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"servos":   servos,
					"warnings": warnings,
				})
			})
		},
	})
	var setFlags changeFlags
	set := &cobra.Command{
		Use:   "set INDEX MIN MAX MIDDLE RATE FORWARD",
		Short: "Plan or set one servo row",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			if strings.EqualFold(args[5], "none") {
				args[5] = "-1"
			}
			if err := requireInts(args); err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.planOrApplyCLI(cmd, []string{"servo " + strings.Join(args, " ")}, "servo", setFlags)
		},
	}
	addChangeFlags(set, &setFlags)
	var reverseFlags changeFlags
	reverse := &cobra.Command{
		Use:   "reverse SERVO SOURCE r|n",
		Short: "Plan or set one servo mixer reverse row",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args[:2]); err != nil {
				return validationFailure(a, cmd, err)
			}
			mode := strings.ToLower(args[2])
			if mode != "r" && mode != "n" {
				return validationFailureMessage(a, cmd, "reverse mode must be r or n")
			}
			return a.planOrApplyCLI(cmd, []string{"smix reverse " + args[0] + " " + args[1] + " " + mode}, "smix", reverseFlags)
		},
	}
	addChangeFlags(reverse, &reverseFlags)
	cmd.AddCommand(set, reverse, a.servoSetConfigCommand(), a.servoSetMixRuleCommand())
	return cmd
}

func (a *app) servoSetConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config INDEX MIN MAX MIDDLE RATE FORWARD_FROM_CHANNEL REVERSED_SOURCES_MASK",
		Short: "Set one servo configuration row over MSP",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			min, err := parseUint16Arg("min", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			max, err := parseUint16Arg("max", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			middle, err := parseUint16Arg("middle", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			rate, err := parseInt8Arg("rate", args[4])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			forward, err := parseUint8Arg("forward_from_channel", args[5])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			reversed, err := parseUint32Arg("reversed_sources_mask", args[6])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "servo configuration changes servo settings; pass --yes"))
			}
			config := bfcommands.ServoConfiguration{
				Index:               int(index),
				Min:                 min,
				Max:                 max,
				Middle:              middle,
				Rate:                rate,
				ForwardFromChannel:  forward,
				ReversedSourcesMask: reversed,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetServoConfiguration(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"servo_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "servo_config",
					Command: "MSP_SET_SERVO_CONFIGURATION",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) servoSetMixRuleCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-mix-rule INDEX TARGET_CHANNEL INPUT_SOURCE RATE SPEED MIN MAX BOX",
		Short: "Set one servo mixer rule over MSP",
		Args:  cobra.ExactArgs(8),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			target, err := parseUint8Arg("target_channel", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			source, err := parseUint8Arg("input_source", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			rate, err := parseInt8Arg("rate", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			speed, err := parseUint8Arg("speed", args[4])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			min, err := parseUint8Arg("min", args[5])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			max, err := parseUint8Arg("max", args[6])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			box, err := parseUint8Arg("box", args[7])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "servo mix rule changes servo settings; pass --yes"))
			}
			rule := bfcommands.ServoMixRule{
				Index:         int(index),
				TargetChannel: target,
				InputSource:   source,
				Rate:          rate,
				Speed:         speed,
				Min:           min,
				Max:           max,
				Box:           box,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetServoMixRule(cmd.Context(), client, rule)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"servo_mix_rule": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "servo_mix_rule",
					Command: "MSP_SET_SERVO_MIX_RULE",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) adjustmentsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "adjustments", Short: "Inspect and change adjustment range rows"}
	cmd.AddCommand(a.configListCommand("list", "List adjustment range rows", func(doc bfconfig.Document) any {
		return map[string]any{"adjranges": doc.AdjRanges, "lines": doc.Sections["adjranges"]}
	}))
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read adjustment ranges over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				adjustments, err := bfcommands.ReadAdjustmentStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"adjustments": adjustments,
				})
			})
		},
	})
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set INDEX UNUSED RANGE_CHANNEL START END FUNCTION SELECT_CHANNEL CENTER SCALE",
		Short: "Plan or set one adjrange row",
		Args:  cobra.ExactArgs(9),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args); err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.planOrApplyCLI(cmd, []string{"adjrange " + strings.Join(args, " ")}, "adjrange", flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set, a.adjustmentSetRangeCommand())
	return cmd
}

func (a *app) adjustmentSetRangeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-range INDEX SLOT AUX_CHANNEL START_STEP END_STEP FUNCTION SELECT_CHANNEL CENTER SCALE",
		Short: "Set one adjustment range over MSP",
		Args:  cobra.ExactArgs(9),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			slot, err := parseUint8Arg("slot", args[1])
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
			function, err := parseUint8Arg("function", args[5])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			selectChannel, err := parseUint8Arg("select_channel", args[6])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			center, err := parseUint16Arg("center", args[7])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			scale, err := parseUint16Arg("scale", args[8])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "adjustment range changes configuration; pass --yes"))
			}
			row := bfcommands.AdjustmentRange{
				Index:                 int(index),
				SlotIndex:             slot,
				AuxChannelIndex:       aux,
				RangeStartStep:        start,
				RangeEndStep:          end,
				AdjustmentFunction:    function,
				AuxSwitchChannelIndex: selectChannel,
				AdjustmentCenter:      center,
				AdjustmentScale:       scale,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetAdjustmentRange(cmd.Context(), client, row)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"adjustment_range": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "adjustment_range",
					Command: "MSP_SET_ADJUSTMENT_RANGE",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) rxRangeCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "rxrange", Short: "Inspect and change receiver channel range rows"}
	cmd.AddCommand(a.configListCommand("list", "List receiver channel range rows", func(doc bfconfig.Document) any {
		return map[string]any{"rxranges": doc.RXRanges, "lines": doc.Sections["rxranges"]}
	}))
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set CHANNEL MIN MAX",
		Short: "Plan or set one rxrange row",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireInts(args); err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.planOrApplyCLI(cmd, []string{"rxrange " + strings.Join(args, " ")}, "rxrange", flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set)
	return cmd
}
