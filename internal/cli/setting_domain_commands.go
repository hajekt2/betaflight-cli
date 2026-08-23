package cli

import (
	"encoding/json"
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
		cmd.AddCommand(a.vtxDeviceStatusCommand())
		cmd.AddCommand(a.vtxSetConfigCommand())
		cmd.AddCommand(a.vtxSetConfigJSONCommand())
	}
	if domain.use == "receiver" {
		cmd.AddCommand(a.receiverStatusCommand())
		cmd.AddCommand(a.receiverSetConfigJSONCommand())
		cmd.AddCommand(a.receiverRXFailCommand())
		cmd.AddCommand(a.receiverSetRXFailCommand())
		cmd.AddCommand(a.receiverSetRXFailJSONCommand())
		cmd.AddCommand(a.receiverRSSIChannelCommand())
		cmd.AddCommand(a.receiverRSSIChannelJSONCommand())
		cmd.AddCommand(a.receiverMapCommand())
		cmd.AddCommand(a.receiverMapJSONCommand())
		cmd.AddCommand(a.receiverDeadbandCommand())
		cmd.AddCommand(a.receiverDeadbandJSONCommand())
	}
	if domain.use == "gps" {
		cmd.AddCommand(a.gpsStatusCommand())
		cmd.AddCommand(a.gpsSetConfigCommand())
		cmd.AddCommand(a.gpsSetConfigJSONCommand())
		cmd.AddCommand(a.gpsSetRescueCommand())
		cmd.AddCommand(a.gpsSetRescueJSONCommand())
		cmd.AddCommand(a.gpsSetRescuePIDCommand())
		cmd.AddCommand(a.gpsSetRescuePIDJSONCommand())
	}
	if domain.use == "osd" {
		cmd.AddCommand(a.osdStatusCommand())
		cmd.AddCommand(a.osdSetGeneralJSONCommand())
		cmd.AddCommand(a.osdSetCanvasCommand())
		cmd.AddCommand(a.osdSetVideoSystemCommand())
		cmd.AddCommand(a.osdSetVideoSystemJSONCommand())
		cmd.AddCommand(a.osdSetPositionCommand())
		cmd.AddCommand(a.osdSetPositionJSONCommand())
		cmd.AddCommand(a.osdSetStatCommand())
		cmd.AddCommand(a.osdSetStatJSONCommand())
		cmd.AddCommand(a.osdSetTimerCommand())
		cmd.AddCommand(a.osdSetTimerJSONCommand())
		cmd.AddCommand(a.osdCharGetCommand())
		cmd.AddCommand(a.osdCharSetCommand())
		cmd.AddCommand(a.osdVideoConfigCommand())
		cmd.AddCommand(a.osdSetVideoConfigJSONCommand())
	}
	if domain.use == "pid" {
		cmd.AddCommand(a.pidStatusCommand())
		cmd.AddCommand(a.pidPreviewSimplifiedJSONCommand())
		cmd.AddCommand(a.pidValidateSimplifiedCommand())
		cmd.AddCommand(a.pidSetGainsJSONCommand())
		cmd.AddCommand(a.pidSetAdvancedJSONCommand())
		cmd.AddCommand(a.pidSetSimplifiedJSONCommand())
	}
	if domain.use == "rates" {
		cmd.AddCommand(a.ratesStatusCommand())
		cmd.AddCommand(a.ratesSetProfileJSONCommand())
	}
	if domain.use == "filters" {
		cmd.AddCommand(a.filtersStatusCommand())
		cmd.AddCommand(a.filtersSetAdvancedJSONCommand())
		cmd.AddCommand(a.filtersSetFilterJSONCommand())
	}
	if domain.use == "battery" {
		cmd.AddCommand(a.batteryStatusCommand())
		cmd.AddCommand(a.batterySetConfigJSONCommand())
		cmd.AddCommand(a.batterySetProfileJSONCommand())
		cmd.AddCommand(a.batteryVoltageMeterCommand())
		cmd.AddCommand(a.batteryVoltageMeterJSONCommand())
		cmd.AddCommand(a.batteryCurrentMeterCommand())
		cmd.AddCommand(a.batteryCurrentMeterJSONCommand())
	}
	if domain.use == "failsafe" {
		cmd.AddCommand(a.failsafeStatusCommand())
		cmd.AddCommand(a.failsafeSetArmingJSONCommand())
		cmd.AddCommand(a.failsafeSetConfigJSONCommand())
		cmd.AddCommand(a.failsafeBoardAlignmentCommand())
		cmd.AddCommand(a.failsafeBoardAlignmentJSONCommand())
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

func (a *app) receiverSetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set receiver configuration from JSON through MSP_SET_RX_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseReceiverConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "receiver changes affect control input; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetReceiverConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"receiver_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "receiver_config",
					Command: "MSP_SET_RX_CONFIG",
					Detail:  "receiver config changed but not saved",
				})
				return env
			})
		},
	}
}

func parseReceiverConfigJSON(data []byte) (bfcommands.ReceiverConfig, error) {
	var wrapped struct {
		ReceiverConfig *bfcommands.ReceiverConfig `json:"receiver_config"`
		Config         *bfcommands.ReceiverConfig `json:"config"`
		Receiver       *struct {
			Config *bfcommands.ReceiverConfig `json:"config"`
		} `json:"receiver"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.ReceiverConfig{}, err
	}
	if wrapped.ReceiverConfig != nil {
		return *wrapped.ReceiverConfig, nil
	}
	if wrapped.Config != nil {
		return *wrapped.Config, nil
	}
	if wrapped.Receiver != nil && wrapped.Receiver.Config != nil {
		return *wrapped.Receiver.Config, nil
	}
	var config bfcommands.ReceiverConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.ReceiverConfig{}, err
	}
	return config, nil
}

func (a *app) receiverSetRXFailJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rxfail-json FILE",
		Short: "Set receiver failsafe channel rows from JSON through MSP_SET_RXFAIL_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseRXFailTableJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if err := bfcommands.ValidateRXFailTable(config); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "receiver failsafe changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRXFailTable(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rx_fail_table": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rx_fail",
					Command: "MSP_SET_RXFAIL_CONFIG",
					Detail:  "receiver failsafe changed but not saved",
				})
				return env
			})
		},
	}
}

func parseRXFailTableJSON(data []byte) (bfcommands.RXFailTableSetConfig, error) {
	if isJSONArray(data) {
		var channels []bfcommands.RXFailChannel
		if err := json.Unmarshal(data, &channels); err != nil {
			return bfcommands.RXFailTableSetConfig{}, err
		}
		return bfcommands.RXFailTableSetConfig{Channels: channels}, nil
	}
	var wrapped struct {
		RXFailTable *bfcommands.RXFailTableSetConfig `json:"rx_fail_table"`
		RXFail      []bfcommands.RXFailChannel       `json:"rx_fail"`
		Failsafe    []bfcommands.RXFailChannel       `json:"failsafe"`
		Channels    []bfcommands.RXFailChannel       `json:"channels"`
		Receiver    *struct {
			Failsafe []bfcommands.RXFailChannel `json:"failsafe"`
		} `json:"receiver"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.RXFailTableSetConfig{}, err
	}
	switch {
	case wrapped.RXFailTable != nil:
		return *wrapped.RXFailTable, nil
	case wrapped.RXFail != nil:
		return bfcommands.RXFailTableSetConfig{Channels: wrapped.RXFail}, nil
	case wrapped.Failsafe != nil:
		return bfcommands.RXFailTableSetConfig{Channels: wrapped.Failsafe}, nil
	case wrapped.Channels != nil:
		return bfcommands.RXFailTableSetConfig{Channels: wrapped.Channels}, nil
	case wrapped.Receiver != nil && wrapped.Receiver.Failsafe != nil:
		return bfcommands.RXFailTableSetConfig{Channels: wrapped.Receiver.Failsafe}, nil
	}
	var config bfcommands.RXFailTableSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.RXFailTableSetConfig{}, err
	}
	return config, nil
}

func (a *app) vtxSetConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config BAND CHANNEL POWER PIT_MODE FREQUENCY_MHZ LOW_POWER_DISARM PIT_MODE_FREQUENCY_MHZ",
		Short: "Set VTX band, channel, power, pit mode, and frequencies over MSP",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			band, err := parseUint8Arg("band", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			channel, err := parseUint8Arg("channel", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			power, err := parseUint8Arg("power", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			pitMode, err := parseBoolArg("pit_mode", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			frequency, err := parseUint16Arg("frequency_mhz", args[4])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			lowPowerDisarm, err := parseUint8Arg("low_power_disarm", args[5])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			pitFrequency, err := parseUint16Arg("pit_mode_frequency_mhz", args[6])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "VTX configuration can change RF output; pass --yes"))
			}
			config := bfcommands.VTXConfigSetConfig{
				Band:             band,
				Channel:          channel,
				Power:            power,
				PitMode:          pitMode,
				FrequencyMHz:     frequency,
				LowPowerDisarm:   lowPowerDisarm,
				PitModeFrequency: pitFrequency,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetVTXConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"vtx_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "vtx_config",
					Command: "MSP_SET_VTX_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) vtxSetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set VTX band, channel, power, pit mode, and frequencies from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseVTXConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "VTX configuration can change RF output; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetVTXConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"vtx_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "vtx_config",
					Command: "MSP_SET_VTX_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseVTXConfigJSON(data []byte) (bfcommands.VTXConfigSetConfig, error) {
	var wrapped struct {
		VTXConfig *bfcommands.VTXConfigSetConfig `json:"vtx_config"`
		VTX       *bfcommands.VTXConfigSetConfig `json:"vtx"`
		Config    *bfcommands.VTXConfigSetConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.VTXConfigSetConfig{}, err
	}
	switch {
	case wrapped.VTXConfig != nil:
		return *wrapped.VTXConfig, nil
	case wrapped.VTX != nil:
		return *wrapped.VTX, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var config bfcommands.VTXConfigSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.VTXConfigSetConfig{}, err
	}
	return config, nil
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

func (a *app) receiverSetRXFailCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rxfail INDEX MODE VALUE",
		Short: "Set one receiver failsafe channel through MSP_SET_RXFAIL_CONFIG",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if index >= 18 {
				return validationFailureMessage(a, cmd, "index must be in [0..17]")
			}
			mode, err := parseUint8Arg("mode", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if mode > 2 {
				return validationFailureMessage(a, cmd, "mode must be 0, 1, or 2")
			}
			if index >= 4 && mode == 0 {
				return validationFailureMessage(a, cmd, "mode 0 is only valid for flight channels 0..3")
			}
			value, err := parseUint16Arg("value", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			channel := bfcommands.RXFailChannel{
				Index: int(index),
				Mode:  mode,
				Value: value,
			}
			if err := bfcommands.ValidateRXFailChannel(channel); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "receiver failsafe changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRXFailChannel(cmd.Context(), client, channel)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rx_fail": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rx_fail",
					Command: "MSP_SET_RXFAIL_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) receiverRSSIChannelCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rssi-channel CHANNEL",
		Short: "Set RSSI channel through MSP_SET_RSSI_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			channelValue, err := strconv.ParseUint(args[0], 10, 8)
			if err != nil || channelValue > 18 {
				return validationFailureMessage(a, cmd, "channel must be an integer in [0..18]")
			}
			channel := uint8(channelValue)
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "RSSI channel changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRSSIChannel(cmd.Context(), client, channel)
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

func (a *app) receiverRSSIChannelJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rssi-channel-json FILE",
		Short: "Set RSSI channel from JSON through MSP_SET_RSSI_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			channel, err := parseRSSIChannelJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "RSSI channel changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRSSIChannel(cmd.Context(), client, channel)
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

func parseRSSIChannelJSON(data []byte) (uint8, error) {
	var wrapped struct {
		Channel     *uint8 `json:"channel"`
		RSSIChannel *uint8 `json:"rssi_channel"`
		Value       *uint8 `json:"value"`
		Receiver    *struct {
			Channel     *uint8 `json:"channel"`
			RSSIChannel *uint8 `json:"rssi_channel"`
		} `json:"receiver"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return 0, err
	}
	var channel *uint8
	switch {
	case wrapped.Channel != nil:
		channel = wrapped.Channel
	case wrapped.RSSIChannel != nil:
		channel = wrapped.RSSIChannel
	case wrapped.Value != nil:
		channel = wrapped.Value
	case wrapped.Receiver != nil && wrapped.Receiver.Channel != nil:
		channel = wrapped.Receiver.Channel
	case wrapped.Receiver != nil && wrapped.Receiver.RSSIChannel != nil:
		channel = wrapped.Receiver.RSSIChannel
	default:
		return 0, fmt.Errorf("channel or rssi_channel is required")
	}
	if *channel > 18 {
		return 0, fmt.Errorf("channel must be an integer in [0..18]")
	}
	return *channel, nil
}

func (a *app) receiverMapCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-map ROLL PITCH YAW THROTTLE",
		Short: "Set receiver channel map through MSP_SET_RX_MAP",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			mapping := make([]uint8, 4)
			for i, arg := range args {
				value, err := parseUint8Arg("map", arg)
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				mapping[i] = value
			}
			if err := bfcommands.ValidateRCMap(mapping); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "receiver map changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRCMap(cmd.Context(), client, mapping)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rc_map": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rc_map",
					Command: "MSP_SET_RX_MAP",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) receiverMapJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-map-json FILE",
		Short: "Set receiver channel map from JSON through MSP_SET_RX_MAP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			mapping, err := parseRCMapJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if err := bfcommands.ValidateRCMap(mapping); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "receiver map changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRCMap(cmd.Context(), client, mapping)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rc_map": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rc_map",
					Command: "MSP_SET_RX_MAP",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseRCMapJSON(data []byte) ([]uint8, error) {
	if isJSONArray(data) {
		var mapping []uint8
		if err := json.Unmarshal(data, &mapping); err != nil {
			return nil, err
		}
		return mapping, nil
	}
	var wrapped struct {
		RCMap    []uint8 `json:"rc_map"`
		Map      []uint8 `json:"map"`
		Receiver *struct {
			RCMap []uint8 `json:"rc_map"`
		} `json:"receiver"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return nil, err
	}
	switch {
	case wrapped.RCMap != nil:
		return append([]uint8(nil), wrapped.RCMap...), nil
	case wrapped.Map != nil:
		return append([]uint8(nil), wrapped.Map...), nil
	case wrapped.Receiver != nil && wrapped.Receiver.RCMap != nil:
		return append([]uint8(nil), wrapped.Receiver.RCMap...), nil
	}
	return nil, fmt.Errorf("expected rc_map, map, receiver.rc_map, or a direct array of channel indexes")
}

func (a *app) receiverDeadbandCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-deadband DEADBAND YAW_DEADBAND POS_HOLD_DEADBAND DEADBAND_3D_THROTTLE",
		Short: "Set RC deadband values through MSP_SET_RC_DEADBAND",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			deadband, err := parseUint8Arg("deadband", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			yawDeadband, err := parseUint8Arg("yaw_deadband", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			posHoldDeadband, err := parseUint8Arg("pos_hold_deadband", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			deadband3DThrottle, err := parseUint16Arg("deadband_3d_throttle", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "RC deadband changes receiver configuration; pass --yes"))
			}
			config := bfcommands.RCDeadband{
				Deadband:           deadband,
				YawDeadband:        yawDeadband,
				PosHoldDeadband:    posHoldDeadband,
				Deadband3DThrottle: deadband3DThrottle,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRCDeadband(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rc_deadband": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rc_deadband",
					Command: "MSP_SET_RC_DEADBAND",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) receiverDeadbandJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-deadband-json FILE",
		Short: "Set RC deadband values from JSON through MSP_SET_RC_DEADBAND",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseRCDeadbandJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "RC deadband changes receiver configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRCDeadband(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rc_deadband": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rc_deadband",
					Command: "MSP_SET_RC_DEADBAND",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseRCDeadbandJSON(data []byte) (bfcommands.RCDeadband, error) {
	var config bfcommands.RCDeadband
	if err := json.Unmarshal(data, &config); err == nil && config != (bfcommands.RCDeadband{}) {
		return config, nil
	}
	var wrapped struct {
		RCDeadband *bfcommands.RCDeadband `json:"rc_deadband"`
		Deadband   *bfcommands.RCDeadband `json:"deadband"`
		Config     *bfcommands.RCDeadband `json:"config"`
		Receiver   *struct {
			Deadband *bfcommands.RCDeadband `json:"deadband"`
		} `json:"receiver"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.RCDeadband{}, err
	}
	switch {
	case wrapped.RCDeadband != nil:
		return *wrapped.RCDeadband, nil
	case wrapped.Deadband != nil:
		return *wrapped.Deadband, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case wrapped.Receiver != nil && wrapped.Receiver.Deadband != nil:
		return *wrapped.Receiver.Deadband, nil
	}
	return bfcommands.RCDeadband{}, fmt.Errorf("expected rc_deadband, deadband, config, receiver.deadband, or a direct deadband object")
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

func (a *app) gpsSetConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config PROVIDER SBAS_MODE AUTO_CONFIG AUTO_BAUD HOME_POINT_ONCE UBLOX_USE_GALILEO",
		Short: "Set GPS provider and auto-configuration through MSP_SET_GPS_CONFIG",
		Args:  cobra.ExactArgs(6),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := parseUint8Arg("provider", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			sbasMode, err := parseUint8Arg("sbas_mode", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			autoConfig, err := parseBoolFlagArg("auto_config", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			autoBaud, err := parseBoolFlagArg("auto_baud", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			homePointOnce, err := parseBoolFlagArg("home_point_once", args[4])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			useGalileo, err := parseBoolFlagArg("ublox_use_galileo", args[5])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "GPS configuration changes flight-controller configuration; pass --yes"))
			}
			config := bfcommands.GPSConfig{
				Provider:        provider,
				SBASMode:        sbasMode,
				AutoConfig:      autoConfig,
				AutoBaud:        autoBaud,
				HomePointOnce:   &homePointOnce,
				UBloxUseGalileo: &useGalileo,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetGPSConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"gps_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "gps_config",
					Command: "MSP_SET_GPS_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) gpsSetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set GPS provider and auto-configuration from JSON through MSP_SET_GPS_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseGPSConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "GPS configuration changes flight-controller configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetGPSConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"gps_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "gps_config",
					Command: "MSP_SET_GPS_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseGPSConfigJSON(data []byte) (bfcommands.GPSConfig, error) {
	var wrapped struct {
		GPSConfig *bfcommands.GPSConfig `json:"gps_config"`
		Config    *bfcommands.GPSConfig `json:"config"`
		GPS       *struct {
			Config *bfcommands.GPSConfig `json:"config"`
		} `json:"gps"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.GPSConfig{}, err
	}
	switch {
	case wrapped.GPSConfig != nil:
		return *wrapped.GPSConfig, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case wrapped.GPS != nil && wrapped.GPS.Config != nil:
		return *wrapped.GPS.Config, nil
	}
	var config bfcommands.GPSConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.GPSConfig{}, err
	}
	return config, nil
}

func (a *app) gpsSetRescueCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rescue MAX_ANGLE RETURN_ALTITUDE DESCENT_DISTANCE GROUND_SPEED THROTTLE_MIN THROTTLE_MAX THROTTLE_HOVER SANITY_CHECKS MIN_SATS ASCEND_RATE DESCEND_RATE ALLOW_ARMING_WITHOUT_FIX ALTITUDE_MODE MIN_START_DISTANCE INITIAL_CLIMB",
		Short: "Set GPS Rescue configuration through MSP_SET_GPS_RESCUE",
		Args:  cobra.ExactArgs(15),
		RunE: func(cmd *cobra.Command, args []string) error {
			values := make([]uint16, 0, 12)
			for i, name := range []string{"max_angle", "return_altitude", "descent_distance", "ground_speed", "throttle_min", "throttle_max", "throttle_hover"} {
				value, err := parseUint16Arg(name, args[i])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				values = append(values, value)
			}
			sanityChecks, err := parseUint8Arg("sanity_checks", args[7])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			minSats, err := parseUint8Arg("min_sats", args[8])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			for i, name := range []string{"ascend_rate", "descend_rate"} {
				value, err := parseUint16Arg(name, args[9+i])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				values = append(values, value)
			}
			allowArming, err := parseBoolFlagArg("allow_arming_without_fix", args[11])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			altitudeMode, err := parseUint8Arg("altitude_mode", args[12])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			for i, name := range []string{"min_start_distance", "initial_climb"} {
				value, err := parseUint16Arg(name, args[13+i])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				values = append(values, value)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "GPS Rescue configuration changes GPS settings; pass --yes"))
			}
			ascendRate := values[7]
			descendRate := values[8]
			minStartDistance := values[9]
			initialClimb := values[10]
			config := bfcommands.GPSRescue{
				MaxRescueAngle:        values[0],
				ReturnAltitudeM:       values[1],
				DescentDistanceM:      values[2],
				GroundSpeedCMS:        values[3],
				ThrottleMin:           values[4],
				ThrottleMax:           values[5],
				ThrottleHover:         values[6],
				SanityChecks:          sanityChecks,
				MinSats:               minSats,
				AscendRate:            &ascendRate,
				DescendRate:           &descendRate,
				AllowArmingWithoutFix: &allowArming,
				AltitudeMode:          &altitudeMode,
				MinStartDistanceM:     &minStartDistance,
				InitialClimbM:         &initialClimb,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetGPSRescue(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"gps_rescue": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "gps_rescue",
					Command: "MSP_SET_GPS_RESCUE",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) gpsSetRescueJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rescue-json FILE",
		Short: "Set GPS Rescue configuration from JSON through MSP_SET_GPS_RESCUE",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseGPSRescueJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "GPS Rescue configuration changes GPS settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetGPSRescue(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"gps_rescue": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "gps_rescue",
					Command: "MSP_SET_GPS_RESCUE",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseGPSRescueJSON(data []byte) (bfcommands.GPSRescue, error) {
	var wrapped struct {
		GPSRescue *bfcommands.GPSRescue `json:"gps_rescue"`
		Rescue    *bfcommands.GPSRescue `json:"rescue"`
		Config    *bfcommands.GPSRescue `json:"config"`
		GPS       *struct {
			Rescue *bfcommands.GPSRescue `json:"rescue"`
		} `json:"gps"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.GPSRescue{}, err
	}
	switch {
	case wrapped.GPSRescue != nil:
		return *wrapped.GPSRescue, nil
	case wrapped.Rescue != nil:
		return *wrapped.Rescue, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case wrapped.GPS != nil && wrapped.GPS.Rescue != nil:
		return *wrapped.GPS.Rescue, nil
	}
	var config bfcommands.GPSRescue
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.GPSRescue{}, err
	}
	return config, nil
}

func (a *app) gpsSetRescuePIDCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rescue-pids ALTITUDE_P ALTITUDE_I ALTITUDE_D VELOCITY_P VELOCITY_I VELOCITY_D YAW_P",
		Short: "Set GPS Rescue PID terms through MSP_SET_GPS_RESCUE_PIDS",
		Args:  cobra.ExactArgs(7),
		RunE: func(cmd *cobra.Command, args []string) error {
			values := make([]uint16, 0, 7)
			for i, name := range []string{"altitude_p", "altitude_i", "altitude_d", "velocity_p", "velocity_i", "velocity_d", "yaw_p"} {
				value, err := parseUint16Arg(name, args[i])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				values = append(values, value)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "GPS Rescue PID changes GPS settings; pass --yes"))
			}
			config := bfcommands.GPSRescuePID{
				AltitudeP: values[0],
				AltitudeI: values[1],
				AltitudeD: values[2],
				VelocityP: values[3],
				VelocityI: values[4],
				VelocityD: values[5],
				YawP:      values[6],
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetGPSRescuePID(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"gps_rescue_pids": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "gps_rescue_pids",
					Command: "MSP_SET_GPS_RESCUE_PIDS",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) gpsSetRescuePIDJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-rescue-pids-json FILE",
		Short: "Set GPS Rescue PID terms from JSON through MSP_SET_GPS_RESCUE_PIDS",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseGPSRescuePIDJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "GPS Rescue PID changes GPS settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetGPSRescuePID(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"gps_rescue_pids": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "gps_rescue_pids",
					Command: "MSP_SET_GPS_RESCUE_PIDS",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseGPSRescuePIDJSON(data []byte) (bfcommands.GPSRescuePID, error) {
	var wrapped struct {
		GPSRescuePIDs *bfcommands.GPSRescuePID `json:"gps_rescue_pids"`
		GPSRescuePID  *bfcommands.GPSRescuePID `json:"gps_rescue_pid"`
		RescuePID     *bfcommands.GPSRescuePID `json:"rescue_pid"`
		Config        *bfcommands.GPSRescuePID `json:"config"`
		GPS           *struct {
			RescuePID *bfcommands.GPSRescuePID `json:"rescue_pid"`
		} `json:"gps"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.GPSRescuePID{}, err
	}
	switch {
	case wrapped.GPSRescuePIDs != nil:
		return *wrapped.GPSRescuePIDs, nil
	case wrapped.GPSRescuePID != nil:
		return *wrapped.GPSRescuePID, nil
	case wrapped.RescuePID != nil:
		return *wrapped.RescuePID, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case wrapped.GPS != nil && wrapped.GPS.RescuePID != nil:
		return *wrapped.GPS.RescuePID, nil
	}
	var config bfcommands.GPSRescuePID
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.GPSRescuePID{}, err
	}
	return config, nil
}

func parseBoolFlagArg(name, value string) (bool, error) {
	v, err := parseUint8Arg(name, value)
	if err != nil {
		return false, err
	}
	if v > 1 {
		return false, fmt.Errorf("%s must be 0 or 1", name)
	}
	return v != 0, nil
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

func (a *app) osdSetCanvasCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-canvas COLUMNS ROWS",
		Short: "Set OSD canvas dimensions over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			columns, err := parseUint8Arg("columns", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			rows, err := parseUint8Arg("rows", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD canvas changes can trigger EEPROM write and reboot on HD targets; pass --yes"))
			}
			config := bfcommands.OSDCanvasSetConfig{Columns: columns, Rows: rows}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDCanvas(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_canvas": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "osd_canvas",
					Command: "MSP_SET_OSD_CANVAS",
					Detail:  "canvas changed; firmware may save and reboot when switching to HD MSP displayport",
				})
				return env
			})
		},
	}
}

func (a *app) osdSetGeneralJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-general-json FILE",
		Short: "Patch general OSD config fields from JSON through MSP_SET_OSD_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			patch, err := parseOSDGeneralConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD general config changes can alter display behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDGeneralConfig(cmd.Context(), client, patch)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_general_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "osd_general_config",
					Command: "MSP_SET_OSD_CONFIG",
					Detail:  "OSD general config changed but not saved",
				})
				return env
			})
		},
	}
}

func parseOSDGeneralConfigJSON(data []byte) (bfcommands.OSDGeneralSetConfig, error) {
	var wrapped struct {
		OSDGeneralConfig *bfcommands.OSDGeneralSetConfig `json:"osd_general_config"`
		OSD              *bfcommands.OSDGeneralSetConfig `json:"osd"`
		Config           *bfcommands.OSDGeneralSetConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.OSDGeneralSetConfig{}, err
	}
	if wrapped.OSDGeneralConfig != nil {
		return validateOSDGeneralConfigPatch(*wrapped.OSDGeneralConfig)
	}
	if wrapped.OSD != nil {
		return validateOSDGeneralConfigPatch(*wrapped.OSD)
	}
	if wrapped.Config != nil {
		return validateOSDGeneralConfigPatch(*wrapped.Config)
	}
	var patch bfcommands.OSDGeneralSetConfig
	if err := json.Unmarshal(data, &patch); err != nil {
		return bfcommands.OSDGeneralSetConfig{}, err
	}
	return validateOSDGeneralConfigPatch(patch)
}

func validateOSDGeneralConfigPatch(patch bfcommands.OSDGeneralSetConfig) (bfcommands.OSDGeneralSetConfig, error) {
	if err := bfcommands.ValidateOSDGeneralSetConfig(patch); err != nil {
		return bfcommands.OSDGeneralSetConfig{}, err
	}
	return patch, nil
}

func (a *app) osdSetVideoSystemCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-video-system VIDEO_SYSTEM",
		Short: "Set OSD video system over MSP while preserving the other general OSD settings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			videoSystem, err := parseUint8Arg("video_system", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if videoSystem > 3 {
				return validationFailure(a, cmd, fmt.Errorf("video_system must be 0 (AUTO), 1 (PAL), 2 (NTSC), or 3 (HD)"))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD video system changes can alter canvas and displayport behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDVideoSystem(cmd.Context(), client, videoSystem)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_video_system": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "osd_video_system",
					Command: "MSP_SET_OSD_CONFIG",
					Detail:  "OSD video system changed but not saved; firmware may resize canvas or change displayport mode when switching SD/HD",
				})
				return env
			})
		},
	}
}

func (a *app) osdSetVideoSystemJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-video-system-json FILE",
		Short: "Set OSD video system from JSON over MSP while preserving the other general OSD settings",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			videoSystem, err := parseOSDVideoSystemJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD video system changes can alter canvas and displayport behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDVideoSystem(cmd.Context(), client, videoSystem)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_video_system": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "osd_video_system",
					Command: "MSP_SET_OSD_CONFIG",
					Detail:  "OSD video system changed but not saved; firmware may resize canvas or change displayport mode when switching SD/HD",
				})
				return env
			})
		},
	}
}

func parseOSDVideoSystemJSON(data []byte) (uint8, error) {
	var wrapped struct {
		VideoSystem    *uint8 `json:"video_system"`
		OSDVideoSystem *uint8 `json:"osd_video_system"`
		Value          *uint8 `json:"value"`
		Config         *struct {
			VideoSystem *uint8 `json:"video_system"`
		} `json:"config"`
		OSD *struct {
			VideoSystem *uint8 `json:"video_system"`
		} `json:"osd"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return 0, err
	}
	var videoSystem *uint8
	switch {
	case wrapped.VideoSystem != nil:
		videoSystem = wrapped.VideoSystem
	case wrapped.OSDVideoSystem != nil:
		videoSystem = wrapped.OSDVideoSystem
	case wrapped.Value != nil:
		videoSystem = wrapped.Value
	case wrapped.Config != nil && wrapped.Config.VideoSystem != nil:
		videoSystem = wrapped.Config.VideoSystem
	case wrapped.OSD != nil && wrapped.OSD.VideoSystem != nil:
		videoSystem = wrapped.OSD.VideoSystem
	default:
		return 0, fmt.Errorf("video_system is required")
	}
	if *videoSystem > 3 {
		return 0, fmt.Errorf("video_system must be 0 (AUTO), 1 (PAL), 2 (NTSC), or 3 (HD)")
	}
	return *videoSystem, nil
}

func (a *app) osdCharGetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "char-get INDEX",
		Short: "Read one OSD font character over MSP_OSD_CHAR_READ",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				character, err := bfcommands.ReadOSDChar(cmd.Context(), client, index)
				if err != nil {
					// MSP_OSD_CHAR_READ is answered by external OSD devices only;
					// degrade to a warning when the target has no handler.
					return output.Success(commandPath(cmd), &target, map[string]any{
						"warnings": []string{err.Error()},
					})
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"osd_char": character,
				})
			})
		},
	}
}

func (a *app) osdCharSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "char-set INDEX FILE",
		Short: "Upload one OSD font character from JSON over MSP_OSD_CHAR_WRITE",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			data, err := a.readInput(args[1])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			bitmap, err := parseOSDCharJSON(data, index)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD character writes update the font immediately on the target; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDChar(cmd.Context(), client, index, bitmap)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_char": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "osd_char",
					Command: "MSP_OSD_CHAR_WRITE",
					Detail:  "font character replaced in volatile OSD memory; re-upload after reboot",
				})
				return env
			})
		},
	}
}

func (a *app) osdVideoConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "video-config",
		Short: "Read OSD video system and units over MSP_OSD_VIDEO_CONFIG",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				config, err := bfcommands.ReadOSDVideoConfig(cmd.Context(), client)
				if err != nil {
					// MSP_OSD_VIDEO_CONFIG is answered by external OSD devices
					// only; degrade to a warning when the target has no handler.
					return output.Success(commandPath(cmd), &target, map[string]any{
						"warnings": []string{err.Error()},
					})
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"osd_video_config": config,
				})
			})
		},
	}
}

func (a *app) osdSetVideoConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-video-config-json FILE",
		Short: "Set OSD video system and units from JSON through MSP_SET_OSD_VIDEO_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseOSDVideoConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD video config changes alter video system and unit rendering; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDVideoConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_video_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "osd_video_config",
					Command: "MSP_SET_OSD_VIDEO_CONFIG",
					Detail:  "OSD video config changed but not saved",
				})
				return env
			})
		},
	}
}

// parseOSDCharJSON accepts {"index":n,"rows":[[...18 rows x 12 pixels...]]} or
// {"index":n,"bitmap":[...54 bytes...]}; an explicit INDEX argument wins over
// the JSON index field. Pixel values are two-bit (0-3).
func parseOSDCharJSON(data []byte, index uint8) ([]byte, error) {
	var input struct {
		Index  *uint8    `json:"index"`
		Rows   [][]uint8 `json:"rows"`
		Bitmap []uint8   `json:"bitmap"`
	}
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, err
	}
	switch {
	case len(input.Rows) > 0:
		return bfcommands.EncodeOSDCharPixels(input.Rows)
	case len(input.Bitmap) > 0:
		bitmap := make([]byte, len(input.Bitmap))
		for i, value := range input.Bitmap {
			if value > 255 {
				return nil, fmt.Errorf("OSD char bitmap byte %d out of range", i)
			}
			bitmap[i] = value
		}
		return bitmap, nil
	default:
		return nil, fmt.Errorf("osd char requires rows or bitmap")
	}
}

func parseOSDVideoConfigJSON(data []byte) (bfcommands.OSDVideoConfigSetConfig, error) {
	var wrapped struct {
		OSDVideoConfig *bfcommands.OSDVideoConfigSetConfig `json:"osd_video_config"`
		VideoConfig    *bfcommands.OSDVideoConfigSetConfig `json:"video_config"`
		Config         *bfcommands.OSDVideoConfigSetConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.OSDVideoConfigSetConfig{}, err
	}
	var config bfcommands.OSDVideoConfigSetConfig
	switch {
	case wrapped.OSDVideoConfig != nil:
		config = *wrapped.OSDVideoConfig
	case wrapped.VideoConfig != nil:
		config = *wrapped.VideoConfig
	case wrapped.Config != nil:
		config = *wrapped.Config
	default:
		if err := json.Unmarshal(data, &config); err != nil {
			return bfcommands.OSDVideoConfigSetConfig{}, err
		}
	}
	if err := bfcommands.ValidateOSDVideoConfig(config); err != nil {
		return bfcommands.OSDVideoConfigSetConfig{}, err
	}
	return config, nil
}

func (a *app) osdSetPositionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-position INDEX RAW [SCREEN]",
		Short: "Set one OSD element position over MSP",
		Args:  cobra.RangeArgs(2, 3),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			raw, err := parseUint16Arg("raw", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			screen := uint8(1)
			if len(args) == 3 {
				screen, err = parseUint8Arg("screen", args[2])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD element position changes require --yes"))
			}
			config := bfcommands.OSDPositionSetConfig{Index: index, Raw: raw, Screen: screen}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDPosition(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_position": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "osd_position", Command: "MSP_SET_OSD_CONFIG", Detail: "OSD element position changed but not saved"})
				return env
			})
		},
	}
}

func (a *app) osdSetPositionJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-position-json FILE",
		Short: "Set one OSD element position from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseOSDPositionJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD element position changes require --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDPosition(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_position": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "osd_position", Command: "MSP_SET_OSD_CONFIG", Detail: "OSD element position changed but not saved"})
				return env
			})
		},
	}
}

func parseOSDPositionJSON(data []byte) (bfcommands.OSDPositionSetConfig, error) {
	var wrapped struct {
		OSDPosition *bfcommands.OSDPositionSetConfig `json:"osd_position"`
		Position    *bfcommands.OSDPositionSetConfig `json:"position"`
		Config      *bfcommands.OSDPositionSetConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.OSDPositionSetConfig{}, err
	}
	switch {
	case wrapped.OSDPosition != nil:
		return *wrapped.OSDPosition, nil
	case wrapped.Position != nil:
		return *wrapped.Position, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var config bfcommands.OSDPositionSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.OSDPositionSetConfig{}, err
	}
	return config, nil
}

func (a *app) osdSetStatCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-stat INDEX ENABLED",
		Short: "Set one post-flight OSD statistic flag over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			enabled, err := parseBoolArg("enabled", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD statistic changes require --yes"))
			}
			config := bfcommands.OSDStatSetConfig{Index: index, Enabled: enabled}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDStat(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_stat": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "osd_stat", Command: "MSP_SET_OSD_CONFIG", Detail: "OSD statistic changed but not saved"})
				return env
			})
		},
	}
}

func (a *app) osdSetStatJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-stat-json FILE",
		Short: "Set one post-flight OSD statistic flag from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseOSDStatJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD statistic changes require --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDStat(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_stat": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "osd_stat", Command: "MSP_SET_OSD_CONFIG", Detail: "OSD statistic changed but not saved"})
				return env
			})
		},
	}
}

func parseOSDStatJSON(data []byte) (bfcommands.OSDStatSetConfig, error) {
	var wrapped struct {
		OSDStat *bfcommands.OSDStatSetConfig `json:"osd_stat"`
		Stat    *bfcommands.OSDStatSetConfig `json:"stat"`
		Config  *bfcommands.OSDStatSetConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.OSDStatSetConfig{}, err
	}
	switch {
	case wrapped.OSDStat != nil:
		return *wrapped.OSDStat, nil
	case wrapped.Stat != nil:
		return *wrapped.Stat, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var config bfcommands.OSDStatSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.OSDStatSetConfig{}, err
	}
	return config, nil
}

func (a *app) osdSetTimerCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-timer INDEX VALUE",
		Short: "Set one OSD timer value over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			value, err := parseUint16Arg("value", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD timer changes require --yes"))
			}
			config := bfcommands.OSDTimerSetConfig{Index: index, Value: value}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDTimer(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_timer": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "osd_timer", Command: "MSP_SET_OSD_CONFIG", Detail: "OSD timer changed but not saved"})
				return env
			})
		},
	}
}

func (a *app) osdSetTimerJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-timer-json FILE",
		Short: "Set one OSD timer value from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseOSDTimerJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "OSD timer changes require --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetOSDTimer(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"osd_timer": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "osd_timer", Command: "MSP_SET_OSD_CONFIG", Detail: "OSD timer changed but not saved"})
				return env
			})
		},
	}
}

func parseOSDTimerJSON(data []byte) (bfcommands.OSDTimerSetConfig, error) {
	var wrapped struct {
		OSDTimer *bfcommands.OSDTimerSetConfig `json:"osd_timer"`
		Timer    *bfcommands.OSDTimerSetConfig `json:"timer"`
		Config   *bfcommands.OSDTimerSetConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.OSDTimerSetConfig{}, err
	}
	switch {
	case wrapped.OSDTimer != nil:
		return *wrapped.OSDTimer, nil
	case wrapped.Timer != nil:
		return *wrapped.Timer, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var config bfcommands.OSDTimerSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.OSDTimerSetConfig{}, err
	}
	return config, nil
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

func (a *app) pidSetGainsJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-gains-json FILE",
		Short: "Set complete PID gain triplets from JSON through MSP_SET_PID",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			gains, err := parsePIDGainsJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "PID gain changes affect flight tuning; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetPIDGains(cmd.Context(), client, gains)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"pid_gains": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "pid_gains",
					Command: "MSP_SET_PID",
					Detail:  "PID gains changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) pidSetAdvancedJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-advanced-json FILE",
		Short: "Set PID advanced profile fields from JSON through MSP_SET_PID_ADVANCED",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			advanced, err := parsePIDAdvancedJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "PID advanced changes affect flight behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetPIDAdvanced(cmd.Context(), client, advanced)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"pid_advanced": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "pid_advanced",
					Command: "MSP_SET_PID_ADVANCED",
					Detail:  "PID advanced profile changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) pidSetSimplifiedJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-simplified-json FILE",
		Short: "Set simplified tuning from JSON through MSP_SET_SIMPLIFIED_TUNING",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			tuning, err := parseSimplifiedTuningJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "simplified tuning changes affect flight behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetSimplifiedTuning(cmd.Context(), client, tuning)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"simplified_tuning": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "simplified_tuning",
					Command: "MSP_SET_SIMPLIFIED_TUNING",
					Detail:  "simplified tuning changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) pidPreviewSimplifiedJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "preview-simplified-json FILE",
		Short: "Preview simplified tuning calculations without changing configuration",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			tuning, err := parseSimplifiedTuningJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.PreviewSimplifiedTuning(cmd.Context(), client, tuning)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{"simplified_tuning_preview": result})
			})
		},
	}
}

func (a *app) pidValidateSimplifiedCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "validate-simplified",
		Short: "Validate current simplified tuning against applied tuning values",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.ValidateSimplifiedTuningState(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{"simplified_tuning_validation": result})
			})
		},
	}
}

func parsePIDGainsJSON(data []byte) ([]bfcommands.PIDGain, error) {
	var gains []bfcommands.PIDGain
	if err := json.Unmarshal(data, &gains); err != nil {
		var wrapped struct {
			Gains []bfcommands.PIDGain `json:"gains"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return nil, err
		}
		gains = wrapped.Gains
	}
	names := bfcommands.DefaultPIDNamesForCLI()
	if len(gains) != len(names) {
		return nil, fmt.Errorf("pid gains must contain exactly %d rows for Betaflight 2025.12", len(names))
	}
	for i := range gains {
		gains[i].Index = i
		if gains[i].Name == "" {
			gains[i].Name = names[i]
		}
	}
	return gains, nil
}

func parsePIDAdvancedJSON(data []byte) (bfcommands.PIDAdvanced, error) {
	var wrapped struct {
		PIDAdvanced *bfcommands.PIDAdvanced `json:"pid_advanced"`
		Advanced    *bfcommands.PIDAdvanced `json:"advanced"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.PIDAdvanced{}, err
	}
	var advanced bfcommands.PIDAdvanced
	if wrapped.PIDAdvanced != nil {
		advanced = *wrapped.PIDAdvanced
	} else if wrapped.Advanced != nil {
		advanced = *wrapped.Advanced
	} else if err := json.Unmarshal(data, &advanced); err != nil {
		return bfcommands.PIDAdvanced{}, err
	}
	if err := bfcommands.ValidatePIDAdvanced(advanced); err != nil {
		return bfcommands.PIDAdvanced{}, err
	}
	return advanced, nil
}

func parseSimplifiedTuningJSON(data []byte) (bfcommands.SimplifiedTuning, error) {
	var wrapped struct {
		SimplifiedTuning *bfcommands.SimplifiedTuning `json:"simplified_tuning"`
		Tuning           *bfcommands.SimplifiedTuning `json:"tuning"`
		PID              *struct {
			SimplifiedTuning *bfcommands.SimplifiedTuning `json:"simplified_tuning"`
		} `json:"pid"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.SimplifiedTuning{}, err
	}
	var tuning bfcommands.SimplifiedTuning
	switch {
	case wrapped.SimplifiedTuning != nil:
		tuning = *wrapped.SimplifiedTuning
	case wrapped.Tuning != nil:
		tuning = *wrapped.Tuning
	case wrapped.PID != nil && wrapped.PID.SimplifiedTuning != nil:
		tuning = *wrapped.PID.SimplifiedTuning
	default:
		if err := json.Unmarshal(data, &tuning); err != nil {
			return bfcommands.SimplifiedTuning{}, err
		}
	}
	if err := bfcommands.ValidateSimplifiedTuning(tuning); err != nil {
		return bfcommands.SimplifiedTuning{}, err
	}
	return tuning, nil
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

func (a *app) ratesSetProfileJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-profile-json FILE",
		Short: "Set the active rate profile from JSON through MSP_SET_RC_TUNING",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			profile, err := parseRateProfileJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "rate profile changes affect flight tuning; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRateProfile(cmd.Context(), client, profile)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rate_profile": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rate_profile",
					Command: "MSP_SET_RC_TUNING",
					Detail:  "rate profile changed but not saved",
				})
				return env
			})
		},
	}
}

func parseRateProfileJSON(data []byte) (bfcommands.RateProfile, error) {
	var wrapped struct {
		RateProfile *bfcommands.RateProfile `json:"rate_profile"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.RateProfile{}, err
	}
	var profile bfcommands.RateProfile
	if wrapped.RateProfile != nil {
		profile = *wrapped.RateProfile
	} else if err := json.Unmarshal(data, &profile); err != nil {
		return bfcommands.RateProfile{}, err
	}
	if len(profile.Axes) != 3 {
		return bfcommands.RateProfile{}, fmt.Errorf("rate profile must include exactly three axes")
	}
	seen := map[string]bool{}
	for _, axis := range profile.Axes {
		name := strings.ToLower(axis.Axis)
		if name != "roll" && name != "pitch" && name != "yaw" {
			return bfcommands.RateProfile{}, fmt.Errorf("rate profile axis %q must be roll, pitch, or yaw", axis.Axis)
		}
		if seen[name] {
			return bfcommands.RateProfile{}, fmt.Errorf("rate profile axis %q is duplicated", axis.Axis)
		}
		seen[name] = true
	}
	for _, name := range []string{"roll", "pitch", "yaw"} {
		if !seen[name] {
			return bfcommands.RateProfile{}, fmt.Errorf("rate profile missing %s axis", name)
		}
	}
	return profile, nil
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

func (a *app) filtersSetAdvancedJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-advanced-json FILE",
		Short: "Set loop and motor advanced config from JSON through MSP_SET_ADVANCED_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseAdvancedConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "advanced loop and motor config changes affect flight behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetAdvancedConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"advanced_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "advanced_config",
					Command: "MSP_SET_ADVANCED_CONFIG",
					Detail:  "advanced config changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) filtersSetFilterJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-filter-json FILE",
		Short: "Set gyro, D-term, dynamic notch, and RPM filter config from JSON through MSP_SET_FILTER_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseFilterConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "filter changes affect flight behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetFilterConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"filter_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "filter_config",
					Command: "MSP_SET_FILTER_CONFIG",
					Detail:  "filter config changed but not saved",
				})
				return env
			})
		},
	}
}

func parseAdvancedConfigJSON(data []byte) (bfcommands.AdvancedConfig, error) {
	var wrapped struct {
		AdvancedConfig *bfcommands.AdvancedConfig `json:"advanced_config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.AdvancedConfig{}, err
	}
	var config bfcommands.AdvancedConfig
	if wrapped.AdvancedConfig != nil {
		config = *wrapped.AdvancedConfig
	} else if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.AdvancedConfig{}, err
	}
	if config.DebugModeCount != 0 && config.DebugMode >= config.DebugModeCount {
		return bfcommands.AdvancedConfig{}, fmt.Errorf("debug_mode must be lower than debug_mode_count")
	}
	return config, nil
}

func parseFilterConfigJSON(data []byte) (bfcommands.FilterConfig, error) {
	var wrapped struct {
		FilterConfig *bfcommands.FilterConfig `json:"filter_config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.FilterConfig{}, err
	}
	var config bfcommands.FilterConfig
	if wrapped.FilterConfig != nil {
		config = *wrapped.FilterConfig
	} else if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.FilterConfig{}, err
	}
	if err := bfcommands.ValidateFilterConfig(config); err != nil {
		return bfcommands.FilterConfig{}, err
	}
	return config, nil
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

func (a *app) batterySetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set battery configuration from JSON through MSP_SET_BATTERY_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseBatteryConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "battery config changes affect safety behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetBatteryConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"battery_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "battery_config",
					Command: "MSP_SET_BATTERY_CONFIG",
					Detail:  "battery config changed but not saved",
				})
				return env
			})
		},
	}
}

func parseBatteryConfigJSON(data []byte) (bfcommands.BatteryConfig, error) {
	var wrapped struct {
		BatteryConfig *bfcommands.BatteryConfig `json:"battery_config"`
		Config        *bfcommands.BatteryConfig `json:"config"`
		Battery       *struct {
			Config *bfcommands.BatteryConfig `json:"config"`
		} `json:"battery"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.BatteryConfig{}, err
	}
	var config bfcommands.BatteryConfig
	switch {
	case wrapped.BatteryConfig != nil:
		config = *wrapped.BatteryConfig
	case wrapped.Config != nil:
		config = *wrapped.Config
	case wrapped.Battery != nil && wrapped.Battery.Config != nil:
		config = *wrapped.Battery.Config
	default:
		if err := json.Unmarshal(data, &config); err != nil {
			return bfcommands.BatteryConfig{}, err
		}
	}
	if err := bfcommands.ValidateBatteryConfig(config); err != nil {
		return bfcommands.BatteryConfig{}, err
	}
	return config, nil
}

func (a *app) batterySetProfileJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-profile-json FILE",
		Short: "Set battery profile from JSON through MSP2_SET_BATTERY_PROFILE",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			profile, err := parseBatteryProfileJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "battery profile changes affect safety behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetBatteryProfile(cmd.Context(), client, profile)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"battery_profile": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "battery_profile",
					Command: "MSP2_SET_BATTERY_PROFILE",
					Detail:  "battery profile changed but not saved",
				})
				return env
			})
		},
	}
}

func parseBatteryProfileJSON(data []byte) (bfcommands.BatteryProfile, error) {
	var wrapped struct {
		BatteryProfile *bfcommands.BatteryProfile `json:"battery_profile"`
		Profile        *bfcommands.BatteryProfile `json:"profile"`
		Battery        *struct {
			Profile *bfcommands.BatteryProfile `json:"profile"`
		} `json:"battery"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.BatteryProfile{}, err
	}
	var profile bfcommands.BatteryProfile
	switch {
	case wrapped.BatteryProfile != nil:
		profile = *wrapped.BatteryProfile
	case wrapped.Profile != nil:
		profile = *wrapped.Profile
	case wrapped.Battery != nil && wrapped.Battery.Profile != nil:
		profile = *wrapped.Battery.Profile
	default:
		if err := json.Unmarshal(data, &profile); err != nil {
			return bfcommands.BatteryProfile{}, err
		}
	}
	if err := bfcommands.ValidateBatteryProfile(profile); err != nil {
		return bfcommands.BatteryProfile{}, err
	}
	return profile, nil
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

func (a *app) batteryVoltageMeterJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-voltage-meter-json FILE",
		Short: "Set voltage meter calibration from JSON through MSP_SET_VOLTAGE_METER_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseVoltageMeterConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "voltage meter calibration changes configuration; pass --yes"))
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

func parseVoltageMeterConfigJSON(data []byte) (bfcommands.VoltageMeterConfig, error) {
	var wrapped struct {
		VoltageMeterConfig  *bfcommands.VoltageMeterConfig  `json:"voltage_meter_config"`
		Config              *bfcommands.VoltageMeterConfig  `json:"config"`
		VoltageMeterConfigs []bfcommands.VoltageMeterConfig `json:"voltage_meter_configs"`
		Battery             *struct {
			VoltageMeterConfigs []bfcommands.VoltageMeterConfig `json:"voltage_meter_configs"`
		} `json:"battery"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.VoltageMeterConfig{}, err
	}
	switch {
	case wrapped.VoltageMeterConfig != nil:
		return *wrapped.VoltageMeterConfig, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case len(wrapped.VoltageMeterConfigs) > 0:
		return wrapped.VoltageMeterConfigs[0], nil
	case wrapped.Battery != nil && len(wrapped.Battery.VoltageMeterConfigs) > 0:
		return wrapped.Battery.VoltageMeterConfigs[0], nil
	}
	var config bfcommands.VoltageMeterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.VoltageMeterConfig{}, err
	}
	return config, nil
}

func (a *app) batteryCurrentMeterCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-current-meter ID SCALE OFFSET",
		Short: "Set current meter calibration through MSP_SET_CURRENT_METER_CONFIG",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			id, err := parseUint8Arg("id", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			scale, err := parseInt16Arg("scale", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			offset, err := parseInt16Arg("offset", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "current meter calibration changes configuration; pass --yes"))
			}
			config := bfcommands.CurrentMeterConfig{
				ID:         id,
				SensorType: 1,
				Scale:      scale,
				Offset:     offset,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetCurrentMeterConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"current_meter_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "current_meter_config",
					Command: "MSP_SET_CURRENT_METER_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) batteryCurrentMeterJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-current-meter-json FILE",
		Short: "Set current meter calibration from JSON through MSP_SET_CURRENT_METER_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseCurrentMeterConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "current meter calibration changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetCurrentMeterConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"current_meter_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "current_meter_config",
					Command: "MSP_SET_CURRENT_METER_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseCurrentMeterConfigJSON(data []byte) (bfcommands.CurrentMeterConfig, error) {
	var wrapped struct {
		CurrentMeterConfig  *bfcommands.CurrentMeterConfig  `json:"current_meter_config"`
		Config              *bfcommands.CurrentMeterConfig  `json:"config"`
		CurrentMeterConfigs []bfcommands.CurrentMeterConfig `json:"current_meter_configs"`
		Battery             *struct {
			CurrentMeterConfigs []bfcommands.CurrentMeterConfig `json:"current_meter_configs"`
		} `json:"battery"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.CurrentMeterConfig{}, err
	}
	switch {
	case wrapped.CurrentMeterConfig != nil:
		return *wrapped.CurrentMeterConfig, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case len(wrapped.CurrentMeterConfigs) > 0:
		return wrapped.CurrentMeterConfigs[0], nil
	case wrapped.Battery != nil && len(wrapped.Battery.CurrentMeterConfigs) > 0:
		return wrapped.Battery.CurrentMeterConfigs[0], nil
	}
	var config bfcommands.CurrentMeterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.CurrentMeterConfig{}, err
	}
	return config, nil
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

func (a *app) failsafeSetArmingJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-arming-json FILE",
		Short: "Set arming configuration from JSON through MSP_SET_ARMING_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseArmingConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "arming changes affect safety behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetArmingConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"arming_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "arming_config",
					Command: "MSP_SET_ARMING_CONFIG",
					Detail:  "arming config changed but not saved",
				})
				return env
			})
		},
	}
}

func parseArmingConfigJSON(data []byte) (bfcommands.ArmingConfig, error) {
	var wrapped struct {
		ArmingConfig *bfcommands.ArmingConfig `json:"arming_config"`
		Config       *bfcommands.ArmingConfig `json:"config"`
		Failsafe     *struct {
			ArmingConfig *bfcommands.ArmingConfig `json:"arming_config"`
		} `json:"failsafe"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.ArmingConfig{}, err
	}
	if wrapped.ArmingConfig != nil {
		return *wrapped.ArmingConfig, nil
	}
	if wrapped.Config != nil {
		return *wrapped.Config, nil
	}
	if wrapped.Failsafe != nil && wrapped.Failsafe.ArmingConfig != nil {
		return *wrapped.Failsafe.ArmingConfig, nil
	}
	var config bfcommands.ArmingConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.ArmingConfig{}, err
	}
	return config, nil
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

func (a *app) failsafeBoardAlignmentJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-board-alignment-json FILE",
		Short: "Set board alignment from JSON through MSP_SET_BOARD_ALIGNMENT_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			alignment, err := parseBoardAlignmentJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "board alignment changes configuration; pass --yes"))
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

func parseBoardAlignmentJSON(data []byte) (bfcommands.BoardAlignment, error) {
	var wrapped struct {
		BoardAlignment *bfcommands.BoardAlignment `json:"board_alignment"`
		Alignment      *bfcommands.BoardAlignment `json:"alignment"`
		Config         *bfcommands.BoardAlignment `json:"config"`
		Failsafe       *struct {
			BoardAlignment *bfcommands.BoardAlignment `json:"board_alignment"`
		} `json:"failsafe"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.BoardAlignment{}, err
	}
	switch {
	case wrapped.BoardAlignment != nil:
		return *wrapped.BoardAlignment, nil
	case wrapped.Alignment != nil:
		return *wrapped.Alignment, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case wrapped.Failsafe != nil && wrapped.Failsafe.BoardAlignment != nil:
		return *wrapped.Failsafe.BoardAlignment, nil
	}
	var alignment bfcommands.BoardAlignment
	if err := json.Unmarshal(data, &alignment); err != nil {
		return bfcommands.BoardAlignment{}, err
	}
	return alignment, nil
}

func (a *app) failsafeSetConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set failsafe configuration from JSON through MSP_SET_FAILSAFE_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseFailsafeConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "failsafe changes affect safety behavior; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetFailsafeConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"failsafe_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "failsafe_config",
					Command: "MSP_SET_FAILSAFE_CONFIG",
					Detail:  "failsafe config changed but not saved",
				})
				return env
			})
		},
	}
}

func parseFailsafeConfigJSON(data []byte) (bfcommands.FailsafeConfig, error) {
	var wrapped struct {
		FailsafeConfig *bfcommands.FailsafeConfig `json:"failsafe_config"`
		Failsafe       *bfcommands.FailsafeConfig `json:"failsafe"`
		Config         *bfcommands.FailsafeConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.FailsafeConfig{}, err
	}
	if wrapped.FailsafeConfig != nil {
		return *wrapped.FailsafeConfig, nil
	}
	if wrapped.Failsafe != nil {
		return *wrapped.Failsafe, nil
	}
	if wrapped.Config != nil {
		return *wrapped.Config, nil
	}
	var config bfcommands.FailsafeConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.FailsafeConfig{}, err
	}
	return config, nil
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
