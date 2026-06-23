package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type ReceiverStatus struct {
	Config      *ReceiverConfig `json:"config,omitempty"`
	RCMap       []uint8         `json:"rc_map,omitempty"`
	RCMapNames  []string        `json:"rc_map_names,omitempty"`
	RSSIChannel *uint8          `json:"rssi_channel,omitempty"`
	Channels    []uint16        `json:"channels,omitempty"`
	Failsafe    []RXFailChannel `json:"failsafe,omitempty"`
}

type ReceiverConfig struct {
	SerialProvider                  uint8   `json:"serial_provider"`
	StickMax                        uint16  `json:"stick_max"`
	StickCenter                     uint16  `json:"stick_center"`
	StickMin                        uint16  `json:"stick_min"`
	SpektrumSatelliteBind           uint8   `json:"spektrum_satellite_bind"`
	RXMinUsec                       uint16  `json:"rx_min_usec"`
	RXMaxUsec                       uint16  `json:"rx_max_usec"`
	AirModeActivateThreshold        uint16  `json:"airmode_activate_threshold"`
	RXSPIProtocol                   uint8   `json:"rx_spi_protocol"`
	RXSPIID                         uint32  `json:"rx_spi_id"`
	RXSPIRFChannelCount             uint8   `json:"rx_spi_rf_channel_count"`
	FPVCamAngleDegrees              uint8   `json:"fpv_cam_angle_degrees"`
	RCSmoothingSetpointCutoff       *uint8  `json:"rc_smoothing_setpoint_cutoff,omitempty"`
	RCSmoothingThrottleCutoff       *uint8  `json:"rc_smoothing_throttle_cutoff,omitempty"`
	RCSmoothingAutoFactorThrottle   *uint8  `json:"rc_smoothing_auto_factor_throttle,omitempty"`
	USBCdcHidType                   *uint8  `json:"usb_cdc_hid_type,omitempty"`
	RCSmoothingAutoFactor           *uint8  `json:"rc_smoothing_auto_factor,omitempty"`
	RCSmoothing                     *uint8  `json:"rc_smoothing,omitempty"`
	ELRSUID                         []uint8 `json:"elrs_uid,omitempty"`
	ELRSModelID                     *uint8  `json:"elrs_model_id,omitempty"`
	DeprecatedRCInterpolation       uint8   `json:"deprecated_rc_interpolation"`
	DeprecatedRCInterpolationIntv   uint8   `json:"deprecated_rc_interpolation_interval"`
	DeprecatedRCInterpolationChans  uint8   `json:"deprecated_rc_interpolation_channels"`
	DeprecatedRCSmoothingType       uint8   `json:"deprecated_rc_smoothing_type"`
	DeprecatedRCSmoothingDerivative uint8   `json:"deprecated_rc_smoothing_derivative_type"`
}

type RXFailChannel struct {
	Index         int    `json:"index"`
	Name          string `json:"name"`
	Mode          uint8  `json:"mode"`
	ModeName      string `json:"mode_name,omitempty"`
	ModeChar      string `json:"mode_char,omitempty"`
	Value         uint16 `json:"value"`
	RequiresValue bool   `json:"requires_value"`
	CLICommand    string `json:"cli_command,omitempty"`
}

type RSSIChannelSetResult struct {
	Channel      uint8  `json:"channel"`
	MSPCode      uint16 `json:"msp_code"`
	MSPName      string `json:"msp_name"`
	Acknowledged bool   `json:"acknowledged"`
	SaveRequired bool   `json:"save_required"`
}

func ReadReceiverStatus(ctx context.Context, client *connection.Client) (*ReceiverStatus, []string, error) {
	status := &ReceiverStatus{}
	warnings := []string{}
	frame, err := client.Request(ctx, msp.MSPRXConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("receiver config unavailable: %w", err)
	}
	config, err := DecodeReceiverConfig(frame.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("receiver config decode failed: %w", err)
	}
	status.Config = config
	if rcMap, err := readRCMap(ctx, client); err == nil {
		status.RCMap = rcMap
		status.RCMapNames = rcMapNames(rcMap)
	} else {
		warnings = append(warnings, err.Error())
	}
	if rssiChannel, err := readRSSIChannel(ctx, client); err == nil {
		status.RSSIChannel = &rssiChannel
	} else {
		warnings = append(warnings, err.Error())
	}
	if channels, err := readRC(ctx, client); err == nil {
		status.Channels = channels
	} else {
		warnings = append(warnings, err.Error())
	}
	if failsafe, err := readRXFailConfig(ctx, client); err == nil {
		status.Failsafe = failsafe
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func SetRSSIChannel(ctx context.Context, client *connection.Client, channel uint8) (*RSSIChannelSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetRSSIConfig, EncodeRSSIChannel(channel)); err != nil {
		return nil, fmt.Errorf("rssi channel request failed: %w", err)
	}
	return &RSSIChannelSetResult{
		Channel:      channel,
		MSPCode:      msp.MSPSetRSSIConfig,
		MSPName:      "MSP_SET_RSSI_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeRSSIChannel(channel uint8) []byte {
	return []byte{channel}
}

func DecodeReceiverConfig(payload []byte) (*ReceiverConfig, error) {
	r := msp.NewPayloadReader(payload)
	serialProvider, err := r.U8()
	if err != nil {
		return nil, err
	}
	stickMax, err := r.U16()
	if err != nil {
		return nil, err
	}
	stickCenter, err := r.U16()
	if err != nil {
		return nil, err
	}
	stickMin, err := r.U16()
	if err != nil {
		return nil, err
	}
	spektrumBind, err := r.U8()
	if err != nil {
		return nil, err
	}
	rxMinUsec, err := r.U16()
	if err != nil {
		return nil, err
	}
	rxMaxUsec, err := r.U16()
	if err != nil {
		return nil, err
	}
	deprecatedInterpolation, err := r.U8()
	if err != nil {
		return nil, err
	}
	deprecatedInterpolationInterval, err := r.U8()
	if err != nil {
		return nil, err
	}
	airModeThreshold, err := r.U16()
	if err != nil {
		return nil, err
	}
	rxSPIProtocol, err := r.U8()
	if err != nil {
		return nil, err
	}
	rxSPIID, err := r.U32()
	if err != nil {
		return nil, err
	}
	rxSPIRFChannelCount, err := r.U8()
	if err != nil {
		return nil, err
	}
	fpvCamAngle, err := r.U8()
	if err != nil {
		return nil, err
	}
	deprecatedInterpolationChannels, err := r.U8()
	if err != nil {
		return nil, err
	}
	config := &ReceiverConfig{
		SerialProvider:                 serialProvider,
		StickMax:                       stickMax,
		StickCenter:                    stickCenter,
		StickMin:                       stickMin,
		SpektrumSatelliteBind:          spektrumBind,
		RXMinUsec:                      rxMinUsec,
		RXMaxUsec:                      rxMaxUsec,
		AirModeActivateThreshold:       airModeThreshold,
		RXSPIProtocol:                  rxSPIProtocol,
		RXSPIID:                        rxSPIID,
		RXSPIRFChannelCount:            rxSPIRFChannelCount,
		FPVCamAngleDegrees:             fpvCamAngle,
		DeprecatedRCInterpolation:      deprecatedInterpolation,
		DeprecatedRCInterpolationIntv:  deprecatedInterpolationInterval,
		DeprecatedRCInterpolationChans: deprecatedInterpolationChannels,
	}
	if r.Remaining() == 0 {
		return config, nil
	}
	deprecatedSmoothingType, err := r.U8()
	if err != nil {
		return nil, err
	}
	config.DeprecatedRCSmoothingType = deprecatedSmoothingType
	setpointCutoff, err := r.U8()
	if err != nil {
		return nil, err
	}
	config.RCSmoothingSetpointCutoff = &setpointCutoff
	throttleCutoff, err := r.U8()
	if err != nil {
		return nil, err
	}
	config.RCSmoothingThrottleCutoff = &throttleCutoff
	autoFactorThrottle, err := r.U8()
	if err != nil {
		return nil, err
	}
	config.RCSmoothingAutoFactorThrottle = &autoFactorThrottle
	deprecatedDerivativeType, err := r.U8()
	if err != nil {
		return nil, err
	}
	config.DeprecatedRCSmoothingDerivative = deprecatedDerivativeType
	if r.Remaining() >= 1 {
		usb, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.USBCdcHidType = &usb
	}
	if r.Remaining() >= 1 {
		autoFactor, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.RCSmoothingAutoFactor = &autoFactor
	}
	if r.Remaining() >= 1 {
		smoothing, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.RCSmoothing = &smoothing
	}
	if r.Remaining() >= 6 {
		uid, err := r.Bytes(6)
		if err != nil {
			return nil, err
		}
		config.ELRSUID = append([]uint8(nil), uid...)
	}
	if r.Remaining() >= 1 {
		modelID, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.ELRSModelID = &modelID
	}
	return config, nil
}

func readRCMap(ctx context.Context, client *connection.Client) ([]uint8, error) {
	frame, err := client.Request(ctx, msp.MSPRXMap, nil)
	if err != nil {
		return nil, fmt.Errorf("receiver map unavailable: %w", err)
	}
	return DecodeBoxIDs(frame.Payload), nil
}

func readRSSIChannel(ctx context.Context, client *connection.Client) (uint8, error) {
	frame, err := client.Request(ctx, msp.MSPRSSIConfig, nil)
	if err != nil {
		return 0, fmt.Errorf("rssi config unavailable: %w", err)
	}
	r := msp.NewPayloadReader(frame.Payload)
	channel, err := r.U8()
	if err != nil {
		return 0, err
	}
	return channel, nil
}

func DecodeRXFailConfig(payload []byte) ([]RXFailChannel, error) {
	const rowSize = 3
	if len(payload)%rowSize != 0 {
		return nil, fmt.Errorf("payload length %d is not divisible by MSP_RXFAIL_CONFIG row size %d", len(payload), rowSize)
	}
	r := msp.NewPayloadReader(payload)
	rows := make([]RXFailChannel, 0, len(payload)/rowSize)
	for index := 0; r.Remaining() > 0; index++ {
		mode, err := r.U8()
		if err != nil {
			return nil, err
		}
		value, err := r.U16()
		if err != nil {
			return nil, err
		}
		rows = append(rows, rxFailChannel(index, mode, value))
	}
	return rows, nil
}

func readRXFailConfig(ctx context.Context, client *connection.Client) ([]RXFailChannel, error) {
	frame, err := client.Request(ctx, msp.MSPRxfailConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("receiver failsafe unavailable: %w", err)
	}
	return DecodeRXFailConfig(frame.Payload)
}

func rcMapNames(mapping []uint8) []string {
	names := []string{"ROLL", "PITCH", "YAW", "THROTTLE"}
	out := make([]string, len(mapping))
	for i, mapped := range mapping {
		if int(mapped) < len(names) {
			out[i] = names[mapped]
			continue
		}
		out[i] = fmt.Sprintf("AUX%d", int(mapped)-len(names)+1)
	}
	return out
}

func rxFailChannel(index int, mode uint8, value uint16) RXFailChannel {
	modeName, modeChar := rxFailMode(mode)
	row := RXFailChannel{
		Index:         index,
		Name:          rcChannelName(index),
		Mode:          mode,
		ModeName:      modeName,
		ModeChar:      modeChar,
		Value:         value,
		RequiresValue: mode == 2,
	}
	if modeChar != "" {
		if row.RequiresValue {
			row.CLICommand = fmt.Sprintf("rxfail %d %s %d", index, modeChar, value)
		} else {
			row.CLICommand = fmt.Sprintf("rxfail %d %s", index, modeChar)
		}
	}
	return row
}

func rxFailMode(mode uint8) (string, string) {
	switch mode {
	case 0:
		return "AUTO", "a"
	case 1:
		return "HOLD", "h"
	case 2:
		return "SET", "s"
	default:
		return "", ""
	}
}

func rcChannelName(index int) string {
	names := []string{"ROLL", "PITCH", "YAW", "THROTTLE"}
	if index < len(names) {
		return names[index]
	}
	return fmt.Sprintf("AUX%d", index-len(names)+1)
}
