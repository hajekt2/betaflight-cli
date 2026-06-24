package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

var rssiSourceNames = []string{
	"NONE",
	"ADC",
	"RX_CHANNEL",
	"RX_PROTOCOL",
	"MSP",
	"FRAME_ERRORS",
	"RX_PROTOCOL_CRSF",
	"RX_PROTOCOL_MAVLINK",
}

type ReceiverStatus struct {
	Config      *ReceiverConfig `json:"config,omitempty"`
	RCMap       []uint8         `json:"rc_map,omitempty"`
	RCMapNames  []string        `json:"rc_map_names,omitempty"`
	RSSIChannel *uint8          `json:"rssi_channel,omitempty"`
	TXInfo      *TXInfo         `json:"tx_info,omitempty"`
	Deadband    *RCDeadband     `json:"deadband,omitempty"`
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

type ReceiverConfigSetResult struct {
	Config       ReceiverConfig `json:"config"`
	MSPCode      uint16         `json:"msp_code"`
	MSPName      string         `json:"msp_name"`
	Acknowledged bool           `json:"acknowledged"`
	SaveRequired bool           `json:"save_required"`
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

type RXFailChannelSetResult struct {
	Channel      RXFailChannel `json:"channel"`
	MSPCode      uint16        `json:"msp_code"`
	MSPName      string        `json:"msp_name"`
	Acknowledged bool          `json:"acknowledged"`
	SaveRequired bool          `json:"save_required"`
}

type RXFailTableSetConfig struct {
	Channels []RXFailChannel `json:"channels"`
}

type RXFailTableSetResult struct {
	Channels     []RXFailChannel `json:"channels"`
	ChannelCount int             `json:"channel_count"`
	MSPCode      uint16          `json:"msp_code"`
	MSPName      string          `json:"msp_name"`
	Acknowledged bool            `json:"acknowledged"`
	SaveRequired bool            `json:"save_required"`
}

type RSSIChannelSetResult struct {
	Channel      uint8  `json:"channel"`
	MSPCode      uint16 `json:"msp_code"`
	MSPName      string `json:"msp_name"`
	Acknowledged bool   `json:"acknowledged"`
	SaveRequired bool   `json:"save_required"`
}

type RSSIConfig struct {
	Channel uint8  `json:"channel"`
	Source  string `json:"source"`
}

type TXInfo struct {
	RSSISource     uint8  `json:"rssi_source"`
	RSSISourceName string `json:"rssi_source_name,omitempty"`
	RTCStatus      uint8  `json:"rtc_status"`
	RTCStatusName  string `json:"rtc_status_name"`
	RTCIsSet       *bool  `json:"rtc_is_set,omitempty"`
	RTCSupported   bool   `json:"rtc_supported"`
	Source         string `json:"source"`
}

type RCMapSetResult struct {
	Map          []uint8  `json:"map"`
	Names        []string `json:"names"`
	MSPCode      uint16   `json:"msp_code"`
	MSPName      string   `json:"msp_name"`
	Acknowledged bool     `json:"acknowledged"`
	SaveRequired bool     `json:"save_required"`
}

type RCDeadband struct {
	Deadband           uint8  `json:"deadband"`
	YawDeadband        uint8  `json:"yaw_deadband"`
	PosHoldDeadband    uint8  `json:"pos_hold_deadband"`
	Deadband3DThrottle uint16 `json:"deadband_3d_throttle"`
}

type RCDeadbandSetResult struct {
	Config       RCDeadband `json:"config"`
	MSPCode      uint16     `json:"msp_code"`
	MSPName      string     `json:"msp_name"`
	Acknowledged bool       `json:"acknowledged"`
	SaveRequired bool       `json:"save_required"`
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
	if txInfo, err := readTXInfo(ctx, client); err == nil {
		status.TXInfo = txInfo
	} else {
		warnings = append(warnings, err.Error())
	}
	if deadband, err := readRCDeadband(ctx, client); err == nil {
		status.Deadband = deadband
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

func SetReceiverConfig(ctx context.Context, client *connection.Client, config ReceiverConfig) (*ReceiverConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetRXConfig, EncodeReceiverConfig(config)); err != nil {
		return nil, fmt.Errorf("receiver config request failed: %w", err)
	}
	return &ReceiverConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetRXConfig,
		MSPName:      "MSP_SET_RX_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeReceiverConfig(config ReceiverConfig) []byte {
	payload := []byte{config.SerialProvider}
	payload = appendU16Payload(payload, config.StickMax)
	payload = appendU16Payload(payload, config.StickCenter)
	payload = appendU16Payload(payload, config.StickMin)
	payload = append(payload, config.SpektrumSatelliteBind)
	payload = appendU16Payload(payload, config.RXMinUsec)
	payload = appendU16Payload(payload, config.RXMaxUsec)
	payload = append(payload, config.DeprecatedRCInterpolation, config.DeprecatedRCInterpolationIntv)
	payload = appendU16Payload(payload, config.AirModeActivateThreshold)
	payload = append(payload, config.RXSPIProtocol)
	payload = appendU32Payload(payload, config.RXSPIID)
	payload = append(payload,
		config.RXSPIRFChannelCount,
		config.FPVCamAngleDegrees,
		config.DeprecatedRCInterpolationChans,
		config.DeprecatedRCSmoothingType,
		optionalU8(config.RCSmoothingSetpointCutoff),
		optionalU8(config.RCSmoothingThrottleCutoff),
		optionalU8(config.RCSmoothingAutoFactorThrottle),
		config.DeprecatedRCSmoothingDerivative,
		optionalU8(config.USBCdcHidType),
		optionalU8(config.RCSmoothingAutoFactor),
		optionalU8(config.RCSmoothing),
	)
	uid := make([]uint8, 6)
	copy(uid, config.ELRSUID)
	payload = append(payload, uid...)
	payload = append(payload, optionalU8(config.ELRSModelID))
	return payload
}

func optionalU8(value *uint8) uint8 {
	if value == nil {
		return 0
	}
	return *value
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

func DecodeRSSIConfig(payload []byte) (*RSSIConfig, error) {
	r := msp.NewPayloadReader(payload)
	channel, err := r.U8()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_RSSI_CONFIG returned %d trailing byte(s)", r.Remaining())
	}
	return &RSSIConfig{Channel: channel, Source: "MSP_RSSI_CONFIG"}, nil
}

func DecodeTXInfo(payload []byte) (*TXInfo, error) {
	r := msp.NewPayloadReader(payload)
	rssiSource, err := r.U8()
	if err != nil {
		return nil, err
	}
	rtcStatus, err := r.U8()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_TX_INFO returned %d trailing byte(s)", r.Remaining())
	}
	info := &TXInfo{
		RSSISource:     rssiSource,
		RSSISourceName: indexedName(rssiSourceNames, rssiSource),
		RTCStatus:      rtcStatus,
		RTCStatusName:  rtcStatusName(rtcStatus),
		RTCSupported:   rtcStatus != 0xff,
		Source:         "MSP_TX_INFO",
	}
	if rtcStatus != 0xff {
		value := rtcStatus != 0
		info.RTCIsSet = &value
	}
	return info, nil
}

func SetRCMap(ctx context.Context, client *connection.Client, mapping []uint8) (*RCMapSetResult, error) {
	if err := ValidateRCMap(mapping); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetRXMap, EncodeRCMap(mapping)); err != nil {
		return nil, fmt.Errorf("receiver map request failed: %w", err)
	}
	copied := append([]uint8(nil), mapping...)
	return &RCMapSetResult{
		Map:          copied,
		Names:        rcMapNames(copied),
		MSPCode:      msp.MSPSetRXMap,
		MSPName:      "MSP_SET_RX_MAP",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeRCMap(mapping []uint8) []byte {
	return append([]byte(nil), mapping...)
}

func ValidateRCMap(mapping []uint8) error {
	if len(mapping) != 4 {
		return fmt.Errorf("receiver map must contain exactly 4 values")
	}
	seen := map[uint8]bool{}
	for _, value := range mapping {
		if value > 3 {
			return fmt.Errorf("receiver map values must be in [0..3]")
		}
		if seen[value] {
			return fmt.Errorf("receiver map values must be a permutation of 0,1,2,3")
		}
		seen[value] = true
	}
	return nil
}

func SetRXFailChannel(ctx context.Context, client *connection.Client, channel RXFailChannel) (*RXFailChannelSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetRxfailConfig, EncodeRXFailChannel(channel)); err != nil {
		return nil, fmt.Errorf("receiver failsafe request failed: %w", err)
	}
	normalized := rxFailChannel(channel.Index, channel.Mode, channel.Value)
	return &RXFailChannelSetResult{
		Channel:      normalized,
		MSPCode:      msp.MSPSetRxfailConfig,
		MSPName:      "MSP_SET_RXFAIL_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetRXFailTable(ctx context.Context, client *connection.Client, config RXFailTableSetConfig) (*RXFailTableSetResult, error) {
	if err := ValidateRXFailTable(config); err != nil {
		return nil, err
	}
	channels := make([]RXFailChannel, 0, len(config.Channels))
	for _, channel := range config.Channels {
		result, err := SetRXFailChannel(ctx, client, channel)
		if err != nil {
			return nil, fmt.Errorf("channel %d: %w", channel.Index, err)
		}
		channels = append(channels, result.Channel)
	}
	return &RXFailTableSetResult{
		Channels:     channels,
		ChannelCount: len(channels),
		MSPCode:      msp.MSPSetRxfailConfig,
		MSPName:      "MSP_SET_RXFAIL_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func ValidateRXFailTable(config RXFailTableSetConfig) error {
	if len(config.Channels) == 0 {
		return fmt.Errorf("channels must contain at least one receiver failsafe row")
	}
	seen := map[int]bool{}
	for _, channel := range config.Channels {
		if err := ValidateRXFailChannel(channel); err != nil {
			return err
		}
		if seen[channel.Index] {
			return fmt.Errorf("duplicate channel index %d", channel.Index)
		}
		seen[channel.Index] = true
	}
	return nil
}

func ValidateRXFailChannel(channel RXFailChannel) error {
	if channel.Index < 0 || channel.Index >= 18 {
		return fmt.Errorf("index must be in [0..17]")
	}
	if channel.Mode > 2 {
		return fmt.Errorf("mode must be 0, 1, or 2")
	}
	if channel.Index >= 4 && channel.Mode == 0 {
		return fmt.Errorf("mode 0 is only valid for flight channels 0..3")
	}
	return nil
}

func EncodeRXFailChannel(channel RXFailChannel) []byte {
	payload := []byte{byte(channel.Index), channel.Mode}
	return appendU16Payload(payload, channel.Value)
}

func SetRCDeadband(ctx context.Context, client *connection.Client, config RCDeadband) (*RCDeadbandSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetRCDeadband, EncodeRCDeadband(config)); err != nil {
		return nil, fmt.Errorf("rc deadband request failed: %w", err)
	}
	return &RCDeadbandSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetRCDeadband,
		MSPName:      "MSP_SET_RC_DEADBAND",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeRCDeadband(config RCDeadband) []byte {
	payload := []byte{config.Deadband, config.YawDeadband, config.PosHoldDeadband}
	return append(payload, byte(config.Deadband3DThrottle), byte(config.Deadband3DThrottle>>8))
}

func DecodeRCDeadband(payload []byte) (*RCDeadband, error) {
	r := msp.NewPayloadReader(payload)
	deadband, err := r.U8()
	if err != nil {
		return nil, err
	}
	yawDeadband, err := r.U8()
	if err != nil {
		return nil, err
	}
	posHoldDeadband, err := r.U8()
	if err != nil {
		return nil, err
	}
	deadband3DThrottle, err := r.U16()
	if err != nil {
		return nil, err
	}
	return &RCDeadband{
		Deadband:           deadband,
		YawDeadband:        yawDeadband,
		PosHoldDeadband:    posHoldDeadband,
		Deadband3DThrottle: deadband3DThrottle,
	}, nil
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
	config, err := DecodeRSSIConfig(frame.Payload)
	if err != nil {
		return 0, err
	}
	return config.Channel, nil
}

func readTXInfo(ctx context.Context, client *connection.Client) (*TXInfo, error) {
	frame, err := client.Request(ctx, msp.MSPTxInfo, nil)
	if err != nil {
		return nil, fmt.Errorf("tx info unavailable: %w", err)
	}
	info, err := DecodeTXInfo(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("tx info decode failed: %w", err)
	}
	return info, nil
}

func readRCDeadband(ctx context.Context, client *connection.Client) (*RCDeadband, error) {
	frame, err := client.Request(ctx, msp.MSPRCDeadband, nil)
	if err != nil {
		return nil, fmt.Errorf("rc deadband unavailable: %w", err)
	}
	return DecodeRCDeadband(frame.Payload)
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

func rtcStatusName(status uint8) string {
	switch status {
	case 0:
		return "NOT_SET"
	case 1:
		return "SET"
	case 0xff:
		return "NOT_SUPPORTED"
	default:
		return "UNKNOWN"
	}
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
