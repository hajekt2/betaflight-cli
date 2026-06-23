package cli

import (
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
	cmd.AddCommand(set)
	return cmd
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
	cmd.AddCommand(set, reverse)
	return cmd
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
	cmd.AddCommand(set)
	return cmd
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
