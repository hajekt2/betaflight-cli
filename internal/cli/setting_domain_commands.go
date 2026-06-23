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
		cmd.AddCommand(a.vtxSetConfigCommand())
	}
	if domain.use == "receiver" {
		cmd.AddCommand(a.receiverStatusCommand())
		cmd.AddCommand(a.receiverSetConfigJSONCommand())
		cmd.AddCommand(a.receiverRXFailCommand())
		cmd.AddCommand(a.receiverSetRXFailCommand())
		cmd.AddCommand(a.receiverRSSIChannelCommand())
		cmd.AddCommand(a.receiverMapCommand())
		cmd.AddCommand(a.receiverDeadbandCommand())
	}
	if domain.use == "gps" {
		cmd.AddCommand(a.gpsStatusCommand())
		cmd.AddCommand(a.gpsSetConfigCommand())
		cmd.AddCommand(a.gpsSetRescueCommand())
		cmd.AddCommand(a.gpsSetRescuePIDCommand())
	}
	if domain.use == "osd" {
		cmd.AddCommand(a.osdStatusCommand())
		cmd.AddCommand(a.osdSetCanvasCommand())
		cmd.AddCommand(a.osdSetPositionCommand())
		cmd.AddCommand(a.osdSetStatCommand())
		cmd.AddCommand(a.osdSetTimerCommand())
	}
	if domain.use == "pid" {
		cmd.AddCommand(a.pidStatusCommand())
		cmd.AddCommand(a.pidSetGainsJSONCommand())
		cmd.AddCommand(a.pidSetAdvancedJSONCommand())
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
		cmd.AddCommand(a.batteryVoltageMeterCommand())
		cmd.AddCommand(a.batteryCurrentMeterCommand())
	}
	if domain.use == "failsafe" {
		cmd.AddCommand(a.failsafeStatusCommand())
		cmd.AddCommand(a.failsafeSetConfigJSONCommand())
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
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "receiver failsafe changes receiver configuration; pass --yes"))
			}
			channel := bfcommands.RXFailChannel{
				Index: int(index),
				Mode:  mode,
				Value: value,
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
				if value > 3 {
					return validationFailureMessage(a, cmd, "receiver map values must be in [0..3]")
				}
				mapping[i] = value
			}
			seen := map[uint8]bool{}
			for _, value := range mapping {
				if seen[value] {
					return validationFailureMessage(a, cmd, "receiver map values must be a permutation of 0,1,2,3")
				}
				seen[value] = true
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
	var profile bfcommands.RateProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		var wrapped struct {
			RateProfile bfcommands.RateProfile `json:"rate_profile"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return bfcommands.RateProfile{}, err
		}
		profile = wrapped.RateProfile
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
	var config bfcommands.AdvancedConfig
	if err := json.Unmarshal(data, &config); err != nil {
		var wrapped struct {
			AdvancedConfig bfcommands.AdvancedConfig `json:"advanced_config"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return bfcommands.AdvancedConfig{}, err
		}
		config = wrapped.AdvancedConfig
	}
	if config.DebugModeCount != 0 && config.DebugMode >= config.DebugModeCount {
		return bfcommands.AdvancedConfig{}, fmt.Errorf("debug_mode must be lower than debug_mode_count")
	}
	return config, nil
}

func parseFilterConfigJSON(data []byte) (bfcommands.FilterConfig, error) {
	var config bfcommands.FilterConfig
	if err := json.Unmarshal(data, &config); err != nil {
		var wrapped struct {
			FilterConfig bfcommands.FilterConfig `json:"filter_config"`
		}
		if wrappedErr := json.Unmarshal(data, &wrapped); wrappedErr != nil {
			return bfcommands.FilterConfig{}, err
		}
		config = wrapped.FilterConfig
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
