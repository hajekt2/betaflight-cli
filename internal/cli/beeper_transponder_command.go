package cli

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) beeperCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "beeper", Short: "Inspect beeper and DShot beacon configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Read beeper configuration over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				beeper, err := bfcommands.ReadBeeperConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"beeper": beeper,
				})
			})
		},
	})
	var enableFlags changeFlags
	enable := &cobra.Command{
		Use:   "enable NAME",
		Short: "Plan or enable one beeper condition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := bfcommands.ValidateBeeperModeName(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"beeper " + mode}, "beeper", enableFlags)
		},
	}
	addChangeFlags(enable, &enableFlags)
	cmd.AddCommand(enable)
	var disableFlags changeFlags
	disable := &cobra.Command{
		Use:   "disable NAME",
		Short: "Plan or disable one beeper condition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := bfcommands.ValidateBeeperModeName(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"beeper -" + mode}, "beeper", disableFlags)
		},
	}
	addChangeFlags(disable, &disableFlags)
	cmd.AddCommand(disable, a.beeperSetJSONCommand(), a.beeperSetConfigCommand(), a.beeperSetConfigJSONCommand())
	return cmd
}

func (a *app) beeperSetJSONCommand() *cobra.Command {
	var flags changeFlags
	cmd := &cobra.Command{
		Use:   "set-json FILE",
		Short: "Plan or set beeper condition enables and disables from JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			lines, err := parseBeeperSetJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.planOrApplyCLI(cmd, lines, "beeper", flags)
		},
	}
	addChangeFlags(cmd, &flags)
	return cmd
}

func parseBeeperSetJSON(data []byte) ([]string, error) {
	type beeperSetInput struct {
		Enable   json.RawMessage `json:"enable"`
		Enabled  json.RawMessage `json:"enabled"`
		Disable  json.RawMessage `json:"disable"`
		Disabled json.RawMessage `json:"disabled"`
	}
	var wrapped struct {
		Beeper  *beeperSetInput `json:"beeper"`
		Changes *beeperSetInput `json:"changes"`
		Set     *beeperSetInput `json:"set"`
		beeperSetInput
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	input := wrapped.beeperSetInput
	for _, candidate := range []*beeperSetInput{wrapped.Beeper, wrapped.Changes, wrapped.Set} {
		if candidate != nil {
			input = *candidate
			break
		}
	}
	enable, err := parseBeeperNameList(firstRawMessage(input.Enable, input.Enabled))
	if err != nil {
		return nil, err
	}
	disable, err := parseBeeperNameList(firstRawMessage(input.Disable, input.Disabled))
	if err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(enable)+len(disable))
	for _, name := range enable {
		mode, err := bfcommands.ValidateBeeperModeName(name)
		if err != nil {
			return nil, err
		}
		lines = append(lines, "beeper "+mode)
	}
	for _, name := range disable {
		mode, err := bfcommands.ValidateBeeperModeName(name)
		if err != nil {
			return nil, err
		}
		lines = append(lines, "beeper -"+mode)
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("at least one beeper enable or disable entry is required")
	}
	return lines, nil
}

func parseBeeperNameList(raw json.RawMessage) ([]string, error) {
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var one string
	if err := json.Unmarshal(raw, &one); err == nil {
		if strings.TrimSpace(one) == "" {
			return nil, fmt.Errorf("beeper mode must not be empty")
		}
		return []string{one}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil {
		return nil, fmt.Errorf("beeper mode list must be a string or array of strings")
	}
	for _, name := range many {
		if strings.TrimSpace(name) == "" {
			return nil, fmt.Errorf("beeper mode must not be empty")
		}
	}
	return many, nil
}

func firstRawMessage(values ...json.RawMessage) json.RawMessage {
	for _, value := range values {
		if len(value) != 0 && string(value) != "null" {
			return value
		}
	}
	return nil
}

func (a *app) beeperSetConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config DISABLED_MASK DSHOT_TONE DSHOT_DISABLED_MASK",
		Short: "Set beeper and DShot beacon masks over MSP",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			disabled, err := parseUint32FlexibleArg("disabled_mask", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			tone, err := parseUint8Arg("dshot_tone", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			dshotDisabled, err := parseUint32FlexibleArg("dshot_disabled_mask", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "beeper configuration changes beeper settings; pass --yes"))
			}
			config := bfcommands.BeeperConfig{
				DisabledMask:            disabled,
				DShotBeaconTone:         tone,
				DShotBeaconDisabledMask: dshotDisabled,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetBeeperConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"beeper_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "beeper_config",
					Command: "MSP_SET_BEEPER_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) beeperSetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set beeper and DShot beacon masks from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseBeeperConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "beeper configuration changes beeper settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetBeeperConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"beeper_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "beeper_config",
					Command: "MSP_SET_BEEPER_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseBeeperConfigJSON(data []byte) (bfcommands.BeeperConfig, error) {
	var wrapped struct {
		BeeperConfig *bfcommands.BeeperConfig `json:"beeper_config"`
		Beeper       *bfcommands.BeeperConfig `json:"beeper"`
		Config       *bfcommands.BeeperConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.BeeperConfig{}, err
	}
	switch {
	case wrapped.BeeperConfig != nil:
		return *wrapped.BeeperConfig, nil
	case wrapped.Beeper != nil:
		return *wrapped.Beeper, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var config bfcommands.BeeperConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.BeeperConfig{}, err
	}
	return config, nil
}

func (a *app) transponderCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "transponder", Short: "Inspect IR transponder configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Read transponder provider and code data over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				transponder, err := bfcommands.ReadTransponderConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"transponder": transponder,
				})
			})
		},
	})
	var setProviderFlags changeFlags
	setProvider := &cobra.Command{
		Use:   "set-provider NAME",
		Short: "Plan or set transponder provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := bfcommands.ValidateTransponderProviderName(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{fmt.Sprintf("set transponder_provider = %s", provider)}, "transponder", setProviderFlags)
		},
	}
	addChangeFlags(setProvider, &setProviderFlags)
	cmd.AddCommand(setProvider)
	var setDataFlags changeFlags
	setData := &cobra.Command{
		Use:   "set-data BYTES",
		Short: "Plan or set transponder data bytes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := bfcommands.ValidateTransponderData(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{bfcommands.FormatTransponderData(data)}, "transponder", setDataFlags)
		},
	}
	addChangeFlags(setData, &setDataFlags)
	cmd.AddCommand(setData, a.transponderSetJSONCommand(), a.transponderSetConfigCommand(), a.transponderSetConfigJSONCommand())
	return cmd
}

func (a *app) transponderSetJSONCommand() *cobra.Command {
	var flags changeFlags
	cmd := &cobra.Command{
		Use:   "set-json FILE",
		Short: "Plan or set transponder provider and data from JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			lines, err := parseTransponderSetJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.planOrApplyCLI(cmd, lines, "transponder", flags)
		},
	}
	addChangeFlags(cmd, &flags)
	return cmd
}

func parseTransponderSetJSON(data []byte) ([]string, error) {
	type transponderSetInput struct {
		Provider     string          `json:"provider"`
		ProviderName string          `json:"provider_name"`
		Name         string          `json:"name"`
		Data         json.RawMessage `json:"data"`
		DataBytes    json.RawMessage `json:"data_bytes"`
		Bytes        json.RawMessage `json:"bytes"`
		DataHex      string          `json:"data_hex"`
	}
	var wrapped struct {
		Transponder *transponderSetInput `json:"transponder"`
		Changes     *transponderSetInput `json:"changes"`
		Set         *transponderSetInput `json:"set"`
		transponderSetInput
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	input := wrapped.transponderSetInput
	for _, candidate := range []*transponderSetInput{wrapped.Transponder, wrapped.Changes, wrapped.Set} {
		if candidate != nil {
			input = *candidate
			break
		}
	}
	var lines []string
	providerText := firstString(input.Provider, input.ProviderName, input.Name)
	if providerText != "" {
		provider, err := bfcommands.ValidateTransponderProviderName(providerText)
		if err != nil {
			return nil, err
		}
		lines = append(lines, fmt.Sprintf("set transponder_provider = %s", provider))
	}
	dataRaw := firstRawMessage(input.Data, input.DataBytes, input.Bytes)
	if len(dataRaw) != 0 || strings.TrimSpace(input.DataHex) != "" {
		values, err := parseTransponderDataJSON(dataRaw, input.DataHex)
		if err != nil {
			return nil, err
		}
		lines = append(lines, bfcommands.FormatTransponderData(values))
	}
	if len(lines) == 0 {
		return nil, fmt.Errorf("provider or data is required")
	}
	return lines, nil
}

func parseTransponderDataJSON(raw json.RawMessage, dataHex string) ([]uint8, error) {
	if strings.TrimSpace(dataHex) != "" {
		decoded, err := hex.DecodeString(strings.TrimSpace(dataHex))
		if err != nil {
			return nil, fmt.Errorf("data_hex must be hexadecimal bytes: %w", err)
		}
		out := make([]uint8, 0, len(decoded))
		for _, value := range decoded {
			out = append(out, uint8(value))
		}
		return out, nil
	}
	if len(raw) == 0 || string(raw) == "null" {
		return nil, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err == nil {
		return bfcommands.ValidateTransponderData(text)
	}
	var numbers []int
	if err := json.Unmarshal(raw, &numbers); err != nil {
		return nil, fmt.Errorf("transponder data must be a comma string, byte array, or data_hex")
	}
	parts := make([]string, 0, len(numbers))
	for _, value := range numbers {
		parts = append(parts, strconv.Itoa(value))
	}
	return bfcommands.ValidateTransponderData(strings.Join(parts, ","))
}

func (a *app) transponderSetConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config PROVIDER BYTES",
		Short: "Set transponder provider and data over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := parseUint8Arg("provider", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			data, err := bfcommands.ValidateTransponderData(args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if provider == 0 && len(data) != 0 {
				return validationFailureMessage(a, cmd, "provider 0 must not include transponder data")
			}
			if provider != 0 && len(data) == 0 {
				return validationFailureMessage(a, cmd, "non-zero provider requires transponder data")
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "transponder configuration changes transponder settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetTransponderConfig(cmd.Context(), client, provider, data)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"transponder_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "transponder_config",
					Command: "MSP_SET_TRANSPONDER_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) transponderSetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set transponder provider and data from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			raw, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			provider, data, err := parseTransponderConfigJSON(raw)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if provider == 0 && len(data) != 0 {
				return validationFailureMessage(a, cmd, "provider 0 must not include transponder data")
			}
			if provider != 0 && len(data) == 0 {
				return validationFailureMessage(a, cmd, "non-zero provider requires transponder data")
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "transponder configuration changes transponder settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetTransponderConfig(cmd.Context(), client, provider, data)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"transponder_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "transponder_config",
					Command: "MSP_SET_TRANSPONDER_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseTransponderConfigJSON(data []byte) (uint8, []uint8, error) {
	var wrapped struct {
		TransponderConfig *bfcommands.TransponderConfig `json:"transponder_config"`
		Transponder       *bfcommands.TransponderConfig `json:"transponder"`
		Config            *bfcommands.TransponderConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return 0, nil, err
	}
	switch {
	case wrapped.TransponderConfig != nil:
		return transponderConfigFields(*wrapped.TransponderConfig)
	case wrapped.Transponder != nil:
		return transponderConfigFields(*wrapped.Transponder)
	case wrapped.Config != nil:
		return transponderConfigFields(*wrapped.Config)
	}
	var config bfcommands.TransponderConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return 0, nil, err
	}
	return transponderConfigFields(config)
}

func transponderConfigFields(config bfcommands.TransponderConfig) (uint8, []uint8, error) {
	data := append([]uint8(nil), config.Data...)
	if len(data) == 0 && strings.TrimSpace(config.DataHex) != "" {
		decoded, err := hex.DecodeString(strings.TrimSpace(config.DataHex))
		if err != nil {
			return 0, nil, fmt.Errorf("data_hex must be hexadecimal bytes: %w", err)
		}
		data = decoded
	}
	return config.Provider, data, nil
}
