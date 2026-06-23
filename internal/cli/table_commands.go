package cli

import (
	"encoding/json"
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
	cmd.AddCommand(set, a.vtxTableSetJSONCommand(), a.vtxTableSetBandCommand(), a.vtxTableSetBandJSONCommand(), a.vtxTableSetPowerCommand(), a.vtxTableSetPowerJSONCommand())
	return cmd
}

func (a *app) vtxTableSetJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-json FILE",
		Short: "Set VTX table band and power rows from JSON through typed MSP writes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseVTXTableJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "VTX table changes can affect RF channel and power mappings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetVTXTable(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"vtxtable": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "vtxtable",
					Command: strings.Join(result.MSPNames, ","),
					Detail:  "VTX table rows changed but not saved",
				})
				return env
			})
		},
	}
}

func parseVTXTableJSON(data []byte) (bfcommands.VTXTableSetConfig, error) {
	var wrapped struct {
		VTXTable *bfcommands.VTXTableSetConfig `json:"vtxtable"`
		Table    *bfcommands.VTXTableSetConfig `json:"vtx_table"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.VTXTableSetConfig{}, err
	}
	if wrapped.VTXTable != nil {
		return validateVTXTableConfig(normalizeVTXTableConfig(*wrapped.VTXTable))
	}
	if wrapped.Table != nil {
		return validateVTXTableConfig(normalizeVTXTableConfig(*wrapped.Table))
	}
	var config bfcommands.VTXTableSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.VTXTableSetConfig{}, err
	}
	return validateVTXTableConfig(normalizeVTXTableConfig(config))
}

func normalizeVTXTableConfig(config bfcommands.VTXTableSetConfig) bfcommands.VTXTableSetConfig {
	for i := range config.Bands {
		config.Bands[i].Name = strings.ToUpper(config.Bands[i].Name)
		config.Bands[i].Letter = strings.ToUpper(config.Bands[i].Letter)
	}
	for i := range config.Powers {
		config.Powers[i].Label = strings.ToUpper(config.Powers[i].Label)
	}
	return config
}

func validateVTXTableConfig(config bfcommands.VTXTableSetConfig) (bfcommands.VTXTableSetConfig, error) {
	if err := bfcommands.ValidateVTXTable(config); err != nil {
		return bfcommands.VTXTableSetConfig{}, err
	}
	return config, nil
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

func (a *app) vtxTableSetBandJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-band-json FILE",
		Short: "Set one VTX table band from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseVTXTableBandJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "VTX table band changes can affect RF channel mappings; pass --yes"))
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

func parseVTXTableBandJSON(data []byte) (bfcommands.VTXTableBandSetConfig, error) {
	var wrapped struct {
		VTXTableBand *bfcommands.VTXTableBandSetConfig `json:"vtxtable_band"`
		Band         *bfcommands.VTXTableBandSetConfig `json:"band"`
		Config       *bfcommands.VTXTableBandSetConfig `json:"config"`
		VTXTable     *struct {
			Band *bfcommands.VTXTableBandSetConfig `json:"band"`
		} `json:"vtxtable"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.VTXTableBandSetConfig{}, err
	}
	var config bfcommands.VTXTableBandSetConfig
	switch {
	case wrapped.VTXTableBand != nil:
		config = *wrapped.VTXTableBand
	case wrapped.Band != nil:
		config = *wrapped.Band
	case wrapped.Config != nil:
		config = *wrapped.Config
	case wrapped.VTXTable != nil && wrapped.VTXTable.Band != nil:
		config = *wrapped.VTXTable.Band
	default:
		if err := json.Unmarshal(data, &config); err != nil {
			return bfcommands.VTXTableBandSetConfig{}, err
		}
	}
	config.Name = strings.ToUpper(config.Name)
	config.Letter = strings.ToUpper(config.Letter)
	if err := bfcommands.ValidateVTXTableBand(config); err != nil {
		return bfcommands.VTXTableBandSetConfig{}, err
	}
	return config, nil
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

func (a *app) vtxTableSetPowerJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-power-json FILE",
		Short: "Set one VTX table power level from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseVTXTablePowerJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "VTX table power changes can affect RF output; pass --yes"))
			}
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

func parseVTXTablePowerJSON(data []byte) (bfcommands.VTXTablePowerSetConfig, error) {
	var wrapped struct {
		VTXTablePower *bfcommands.VTXTablePowerSetConfig `json:"vtxtable_power"`
		Power         *bfcommands.VTXTablePowerSetConfig `json:"power"`
		Config        *bfcommands.VTXTablePowerSetConfig `json:"config"`
		VTXTable      *struct {
			Power *bfcommands.VTXTablePowerSetConfig `json:"power"`
		} `json:"vtxtable"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.VTXTablePowerSetConfig{}, err
	}
	var config bfcommands.VTXTablePowerSetConfig
	switch {
	case wrapped.VTXTablePower != nil:
		config = *wrapped.VTXTablePower
	case wrapped.Power != nil:
		config = *wrapped.Power
	case wrapped.Config != nil:
		config = *wrapped.Config
	case wrapped.VTXTable != nil && wrapped.VTXTable.Power != nil:
		config = *wrapped.VTXTable.Power
	default:
		if err := json.Unmarshal(data, &config); err != nil {
			return bfcommands.VTXTablePowerSetConfig{}, err
		}
	}
	config.Label = strings.ToUpper(config.Label)
	if err := bfcommands.ValidateVTXTablePower(config); err != nil {
		return bfcommands.VTXTablePowerSetConfig{}, err
	}
	return config, nil
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
	cmd.AddCommand(set, a.ledValuesCommand(), a.ledValuesJSONCommand(), a.ledColorsJSONCommand(), a.ledModeColorCommand(), a.ledModeColorJSONCommand())
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

func (a *app) ledValuesJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-values-json FILE",
		Short: "Set LED strip brightness and rainbow values from JSON over MSP2",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			values, err := parseLEDValuesJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "LED values change configuration; pass --yes"))
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

func parseLEDValuesJSON(data []byte) (bfcommands.LEDConfigValues, error) {
	var wrapped struct {
		LEDValues *bfcommands.LEDConfigValues `json:"led_values"`
		Values    *bfcommands.LEDConfigValues `json:"values"`
		Config    *bfcommands.LEDConfigValues `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.LEDConfigValues{}, err
	}
	switch {
	case wrapped.LEDValues != nil:
		return *wrapped.LEDValues, nil
	case wrapped.Values != nil:
		return *wrapped.Values, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var values bfcommands.LEDConfigValues
	if err := json.Unmarshal(data, &values); err != nil {
		return bfcommands.LEDConfigValues{}, err
	}
	return values, nil
}

func (a *app) ledColorsJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-colors-json FILE",
		Short: "Set the full LED HSV color table through MSP_SET_LED_COLORS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			colors, err := parseLEDColorsJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "LED color table changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetLEDColors(cmd.Context(), client, colors)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"led_colors": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "led_colors",
					Command: "MSP_SET_LED_COLORS",
					Detail:  "LED color table changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) ledModeColorCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-mode-color MODE DIRECTION COLOR",
		Short: "Set one LED mode color row through MSP_SET_LED_STRIP_MODECOLOR",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := parseUint8Arg("mode", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			direction, err := parseUint8Arg("direction", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			color, err := parseUint8Arg("color", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "LED mode color changes configuration; pass --yes"))
			}
			modeColor := bfcommands.LEDModeColor{Mode: mode, Direction: direction, Color: color}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetLEDModeColor(cmd.Context(), client, modeColor)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"led_mode_color": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "led_mode_color",
					Command: "MSP_SET_LED_STRIP_MODECOLOR",
					Detail:  "LED mode color changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) ledModeColorJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-mode-color-json FILE",
		Short: "Set one LED mode color row from JSON through MSP_SET_LED_STRIP_MODECOLOR",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			modeColor, err := parseLEDModeColorJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "LED mode color changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetLEDModeColor(cmd.Context(), client, modeColor)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"led_mode_color": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "led_mode_color",
					Command: "MSP_SET_LED_STRIP_MODECOLOR",
					Detail:  "LED mode color changed but not saved",
				})
				return env
			})
		},
	}
}

func parseLEDModeColorJSON(data []byte) (bfcommands.LEDModeColor, error) {
	var wrapped struct {
		LEDModeColor *bfcommands.LEDModeColor `json:"led_mode_color"`
		ModeColor    *bfcommands.LEDModeColor `json:"mode_color"`
		Config       *bfcommands.LEDModeColor `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.LEDModeColor{}, err
	}
	switch {
	case wrapped.LEDModeColor != nil:
		return *wrapped.LEDModeColor, nil
	case wrapped.ModeColor != nil:
		return *wrapped.ModeColor, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var modeColor bfcommands.LEDModeColor
	if err := json.Unmarshal(data, &modeColor); err != nil {
		return bfcommands.LEDModeColor{}, err
	}
	return modeColor, nil
}

func parseLEDColorsJSON(data []byte) ([]bfcommands.LEDColor, error) {
	var colors []bfcommands.LEDColor
	if err := json.Unmarshal(data, &colors); err != nil {
		var wrapped struct {
			Colors []bfcommands.LEDColor `json:"colors"`
			LEDs   *struct {
				Colors []bfcommands.LEDColor `json:"colors"`
			} `json:"leds"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return nil, err
		}
		switch {
		case wrapped.Colors != nil:
			colors = wrapped.Colors
		case wrapped.LEDs != nil:
			colors = wrapped.LEDs.Colors
		default:
			return nil, fmt.Errorf("LED color JSON must be an array or object with colors")
		}
	}
	if err := bfcommands.ValidateLEDColors(colors); err != nil {
		return nil, err
	}
	return colors, nil
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
	cmd.AddCommand(set, reverse, a.servoSetJSONCommand(), a.servoSetConfigCommand(), a.servoSetConfigJSONCommand(), a.servoSetMixRuleCommand(), a.servoSetMixRuleJSONCommand())
	return cmd
}

func (a *app) servoSetJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-json FILE",
		Short: "Set servo configuration and mix rows from JSON through typed MSP writes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseServoTableJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "servo table changes servo settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetServoTable(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"servo_table": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "servo_table",
					Command: strings.Join(result.MSPNames, ","),
					Detail:  "servo rows changed but not saved",
				})
				return env
			})
		},
	}
}

func parseServoTableJSON(data []byte) (bfcommands.ServoTableSetConfig, error) {
	var wrapped struct {
		ServoTable *bfcommands.ServoTableSetConfig `json:"servo_table"`
		Servos     *bfcommands.ServoTableSetConfig `json:"servos"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.ServoTableSetConfig{}, err
	}
	if wrapped.ServoTable != nil {
		return validateServoTableConfig(*wrapped.ServoTable)
	}
	if wrapped.Servos != nil {
		return validateServoTableConfig(*wrapped.Servos)
	}
	var config bfcommands.ServoTableSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.ServoTableSetConfig{}, err
	}
	return validateServoTableConfig(config)
}

func validateServoTableConfig(config bfcommands.ServoTableSetConfig) (bfcommands.ServoTableSetConfig, error) {
	if err := bfcommands.ValidateServoTable(config); err != nil {
		return bfcommands.ServoTableSetConfig{}, err
	}
	return config, nil
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

func (a *app) servoSetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set one servo configuration row from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseServoConfigurationJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "servo configuration changes servo settings; pass --yes"))
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

func parseServoConfigurationJSON(data []byte) (bfcommands.ServoConfiguration, error) {
	var wrapped struct {
		ServoConfig *bfcommands.ServoConfiguration `json:"servo_config"`
		Servo       *bfcommands.ServoConfiguration `json:"servo"`
		Config      *bfcommands.ServoConfiguration `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.ServoConfiguration{}, err
	}
	switch {
	case wrapped.ServoConfig != nil:
		return validateServoConfiguration(*wrapped.ServoConfig)
	case wrapped.Servo != nil:
		return validateServoConfiguration(*wrapped.Servo)
	case wrapped.Config != nil:
		return validateServoConfiguration(*wrapped.Config)
	}
	var config bfcommands.ServoConfiguration
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.ServoConfiguration{}, err
	}
	return validateServoConfiguration(config)
}

func validateServoConfiguration(config bfcommands.ServoConfiguration) (bfcommands.ServoConfiguration, error) {
	if err := bfcommands.ValidateServoConfiguration(config); err != nil {
		return bfcommands.ServoConfiguration{}, err
	}
	return config, nil
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

func (a *app) servoSetMixRuleJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-mix-rule-json FILE",
		Short: "Set one servo mixer rule from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			rule, err := parseServoMixRuleJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "servo mix rule changes servo settings; pass --yes"))
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

func parseServoMixRuleJSON(data []byte) (bfcommands.ServoMixRule, error) {
	var wrapped struct {
		ServoMixRule *bfcommands.ServoMixRule `json:"servo_mix_rule"`
		MixRule      *bfcommands.ServoMixRule `json:"mix_rule"`
		Rule         *bfcommands.ServoMixRule `json:"rule"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.ServoMixRule{}, err
	}
	switch {
	case wrapped.ServoMixRule != nil:
		return validateServoMixRule(*wrapped.ServoMixRule)
	case wrapped.MixRule != nil:
		return validateServoMixRule(*wrapped.MixRule)
	case wrapped.Rule != nil:
		return validateServoMixRule(*wrapped.Rule)
	}
	var rule bfcommands.ServoMixRule
	if err := json.Unmarshal(data, &rule); err != nil {
		return bfcommands.ServoMixRule{}, err
	}
	return validateServoMixRule(rule)
}

func validateServoMixRule(rule bfcommands.ServoMixRule) (bfcommands.ServoMixRule, error) {
	if err := bfcommands.ValidateServoMixRule(rule); err != nil {
		return bfcommands.ServoMixRule{}, err
	}
	return rule, nil
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
	cmd.AddCommand(set, a.adjustmentSetJSONCommand(), a.adjustmentSetRangeCommand(), a.adjustmentSetRangeJSONCommand())
	return cmd
}

func (a *app) adjustmentSetJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-json FILE",
		Short: "Set adjustment range rows from JSON through MSP_SET_ADJUSTMENT_RANGE",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseAdjustmentTableJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "adjustment range changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetAdjustmentTable(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"adjustment_table": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "adjustment_table",
					Command: "MSP_SET_ADJUSTMENT_RANGE",
					Detail:  "adjustment range rows changed but not saved",
				})
				return env
			})
		},
	}
}

func parseAdjustmentTableJSON(data []byte) (bfcommands.AdjustmentTableSetConfig, error) {
	var wrapped struct {
		AdjustmentTable *bfcommands.AdjustmentTableSetConfig `json:"adjustment_table"`
		Adjustments     *bfcommands.AdjustmentTableSetConfig `json:"adjustments"`
		Ranges          []bfcommands.AdjustmentRange         `json:"ranges"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.AdjustmentTableSetConfig{}, err
	}
	if wrapped.AdjustmentTable != nil {
		return validateAdjustmentTableConfig(*wrapped.AdjustmentTable)
	}
	if wrapped.Adjustments != nil {
		return validateAdjustmentTableConfig(*wrapped.Adjustments)
	}
	if wrapped.Ranges != nil {
		return validateAdjustmentTableConfig(bfcommands.AdjustmentTableSetConfig{Ranges: wrapped.Ranges})
	}
	var ranges []bfcommands.AdjustmentRange
	if err := json.Unmarshal(data, &ranges); err != nil {
		return bfcommands.AdjustmentTableSetConfig{}, err
	}
	return validateAdjustmentTableConfig(bfcommands.AdjustmentTableSetConfig{Ranges: ranges})
}

func validateAdjustmentTableConfig(config bfcommands.AdjustmentTableSetConfig) (bfcommands.AdjustmentTableSetConfig, error) {
	if err := bfcommands.ValidateAdjustmentTable(config); err != nil {
		return bfcommands.AdjustmentTableSetConfig{}, err
	}
	return config, nil
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

func (a *app) adjustmentSetRangeJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-range-json FILE",
		Short: "Set one adjustment range from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			row, err := parseAdjustmentRangeJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "adjustment range changes configuration; pass --yes"))
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

func parseAdjustmentRangeJSON(data []byte) (bfcommands.AdjustmentRange, error) {
	var wrapped struct {
		AdjustmentRange *bfcommands.AdjustmentRange `json:"adjustment_range"`
		Range           *bfcommands.AdjustmentRange `json:"range"`
		Config          *bfcommands.AdjustmentRange `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.AdjustmentRange{}, err
	}
	switch {
	case wrapped.AdjustmentRange != nil:
		return validateAdjustmentRange(*wrapped.AdjustmentRange)
	case wrapped.Range != nil:
		return validateAdjustmentRange(*wrapped.Range)
	case wrapped.Config != nil:
		return validateAdjustmentRange(*wrapped.Config)
	}
	var row bfcommands.AdjustmentRange
	if err := json.Unmarshal(data, &row); err != nil {
		return bfcommands.AdjustmentRange{}, err
	}
	return validateAdjustmentRange(row)
}

func validateAdjustmentRange(row bfcommands.AdjustmentRange) (bfcommands.AdjustmentRange, error) {
	if err := bfcommands.ValidateAdjustmentRange(row); err != nil {
		return bfcommands.AdjustmentRange{}, err
	}
	return row, nil
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
	cmd.AddCommand(set, a.rxRangeSetJSONCommand())
	return cmd
}

func (a *app) rxRangeSetJSONCommand() *cobra.Command {
	var flags changeFlags
	cmd := &cobra.Command{
		Use:   "set-json FILE",
		Short: "Plan or set receiver channel range rows from JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			rows, err := parseRXRangeRowsJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			lines := make([]string, 0, len(rows))
			for _, row := range rows {
				lines = append(lines, fmt.Sprintf("rxrange %d %d %d", row.Channel, row.Min, row.Max))
			}
			return a.planOrApplyCLI(cmd, lines, "rxrange", flags)
		},
	}
	addChangeFlags(cmd, &flags)
	return cmd
}

type rxRangeSetRow struct {
	Channel int `json:"channel"`
	Min     int `json:"min"`
	Max     int `json:"max"`
}

func parseRXRangeRowsJSON(data []byte) ([]rxRangeSetRow, error) {
	var wrapped struct {
		RXRanges []rxRangeSetRow `json:"rxranges"`
		Ranges   []rxRangeSetRow `json:"ranges"`
		RXRange  *rxRangeSetRow  `json:"rxrange"`
		Range    *rxRangeSetRow  `json:"range"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	switch {
	case wrapped.RXRanges != nil:
		return validateRXRangeRows(wrapped.RXRanges)
	case wrapped.Ranges != nil:
		return validateRXRangeRows(wrapped.Ranges)
	case wrapped.RXRange != nil:
		return validateRXRangeRows([]rxRangeSetRow{*wrapped.RXRange})
	case wrapped.Range != nil:
		return validateRXRangeRows([]rxRangeSetRow{*wrapped.Range})
	}
	var rows []rxRangeSetRow
	if err := json.Unmarshal(data, &rows); err == nil {
		return validateRXRangeRows(rows)
	}
	var row rxRangeSetRow
	if err := json.Unmarshal(data, &row); err != nil {
		return nil, err
	}
	return validateRXRangeRows([]rxRangeSetRow{row})
}

func validateRXRangeRows(rows []rxRangeSetRow) ([]rxRangeSetRow, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("at least one rxrange row is required")
	}
	for i, row := range rows {
		if row.Channel < 0 || row.Channel > 255 {
			return nil, fmt.Errorf("rows[%d].channel must be 0..255", i)
		}
		if row.Min < 0 || row.Min > 2500 {
			return nil, fmt.Errorf("rows[%d].min must be 0..2500", i)
		}
		if row.Max < 0 || row.Max > 2500 {
			return nil, fmt.Errorf("rows[%d].max must be 0..2500", i)
		}
		if row.Min > row.Max {
			return nil, fmt.Errorf("rows[%d].min must be less than or equal to max", i)
		}
	}
	return rows, nil
}
