package cli

import (
	"encoding/binary"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type mspPayloadDecoder func([]byte) (any, error)

var mspDecodeRegistry = map[uint16]mspPayloadDecoder{
	msp.MSPAPIVersion:         decodeMSPAPIVersionPayload,
	msp.MSPFCVariant:          decodeMSPFCVariantPayload,
	msp.MSPFCVersion:          decodeMSPFCVersionPayload,
	msp.MSPBoardInfo:          decodeVia(commands.DecodeBoardInfo),
	msp.MSPBuildInfo:          decodeVia(commands.DecodeBuildInfo),
	msp.MSPName:               decodeString(commands.DecodeName),
	msp.MSPUID:                decodeVia(commands.DecodeDeviceUID),
	msp.MSP2McuInfo:           decodeVia(commands.DecodeMCUInfo),
	msp.MSPStatus:             decodeVia(commands.DecodeStatus),
	msp.MSPStatusEx:           decodeVia(commands.DecodeStatusEx),
	msp.MSPAttitude:           decodeVia(commands.DecodeAttitude),
	msp.MSPAltitude:           decodeVia(commands.DecodeAltitude),
	msp.MSPSonarAltitude:      decodeVia(commands.DecodeRangefinderAltitude),
	msp.MSPBatteryState:       decodeVia(commands.DecodeBatteryRuntimeState),
	msp.MSPAnalog:             decodeVia(commands.DecodeAnalog),
	msp.MSPBatteryConfig:       decodeVia(commands.DecodeBatteryConfig),
	msp.MSPVoltageMeters:      decodeVia(commands.DecodeVoltageMeters),
	msp.MSPCurrentMeters:      decodeVia(commands.DecodeCurrentMeters),
	msp.MSPVoltageMeterConfig: decodeVia(commands.DecodeVoltageMeterConfigs),
	msp.MSPCurrentMeterConfig: decodeVia(commands.DecodeCurrentMeterConfigs),
	msp.MSPRC:                 decodeVia(commands.DecodeRC),
	msp.MSPRtc:                decodeVia(commands.DecodeRTC),
	msp.MSPDataflashSummary:    decodeVia(commands.DecodeDataflashSummary),
	msp.MSPDataflashRead:      decodeVia(commands.DecodeDataflashReadChunk),
	msp.MSP2BatteryProfile:     decodeVia(commands.DecodeBatteryProfile),
	msp.MSPSdcardSummary:      decodeVia(commands.DecodeSDCardSummary),
	msp.MSPBoxnames:           decodeStringSlice(commands.DecodeBoxNames),
	msp.MSPBlackboxConfig:      decodeVia(commands.DecodeBlackboxConfig),
	msp.MSPBoxids:             decodeByteSlice(commands.DecodeBoxIDs),
	msp.MSPModeRanges:         decodeVia(commands.DecodeModeRanges),
	msp.MSPModeRangesExtra:    decodeVia(commands.DecodeModeRangeExtras),
	msp.MSPAdjustmentRanges:   decodeVia(commands.DecodeAdjustmentRanges),
	msp.MSPAdvancedConfig:     decodeVia(commands.DecodeAdvancedConfig),
	msp.MSPFilterConfig:       decodeVia(commands.DecodeFilterConfig),
	msp.MSPPID:               decodeVia(decodeMSPPIDPayload),
	msp.MSPPidnames:          decodeStringSlice(commands.DecodePIDNames),
	msp.MSPPIDController:      decodeVia(commands.DecodePIDController),
	msp.MSPRCTuning:           decodeVia(commands.DecodeRateProfile),
	msp.MSPPIDAdvanced:        decodeVia(commands.DecodePIDAdvanced),
	msp.MSPFeatureConfig:      decodeVia(commands.DecodeFeatureStatus),
	msp.MSPArmingConfig:       decodeVia(commands.DecodeArmingConfig),
	msp.MSPFailsafeConfig:     decodeVia(commands.DecodeFailsafeConfig),
	msp.MSPBoardAlignmentConfig: decodeVia(commands.DecodeBoardAlignment),
	msp.MSPAccTrim:            decodeRawAccTrim,
	msp.MSP2GetText:           decodeMSP2Text,
	msp.MSP2SensorConfigActive: decodeVia(func(payload []byte) ([]any, error) {
		out := make([]any, len(payload))
		for i, raw := range payload {
			out[i] = float64(raw)
		}
		return out, nil
	}),
	msp.MSP2GyroSensorActive:  decodeVia(commands.DecodeActiveGyros),
	msp.MSPRXConfig:           decodeVia(commands.DecodeReceiverConfig),
	msp.MSPRSSIConfig:         decodeRawBytes,
	msp.MSPRXMap:              decodeByteSlice(commands.DecodeBoxIDs),
	msp.MSPRxfailConfig:       decodeVia(commands.DecodeRXFailConfig),
	msp.MSPServo:              decodeVia(commands.DecodeU16Array),
	msp.MSP2GetOSDWarnings:    decodeVia(commands.DecodeOSDWarnings),
	msp.MSPMotor:              decodeVia(commands.DecodeU16Array),
	msp.MSP2MotorOutputReordering: decodeVia(commands.DecodeMotorOutputOrder),
	msp.MSPGPSConfig:          decodeVia(commands.DecodeGPSConfig),
	msp.MSPRawGPS:             decodeVia(commands.DecodeGPSPosition),
	msp.MSPGPSRescue:          decodeVia(commands.DecodeGPSRescue),
	msp.MSPGPSRescuePids:      decodeVia(commands.DecodeGPSRescuePID),
	msp.MSPCompGPS:            decodeVia(commands.DecodeGPSHome),
	msp.MSPGpssvinfo:          decodeVia(commands.DecodeGPSSatellites),
	msp.MSPSensorConfig:       decodeVia(commands.DecodeActiveGyros),
	msp.MSPRawImu:             decodeVia(commands.DecodeRawIMU),
	msp.MSPSensorAlignment:     decodeVia(commands.DecodeSensorAlignment),
	msp.MSPCompassConfig:      decodeVia(commands.DecodeCompassConfig),
	msp.MSPMotorConfig:        decodeVia(commands.DecodeMotorConfig),
	msp.MSPMotorTelemetry:     decodeVia(commands.DecodeMotorTelemetry),
	msp.MSPMotor3dConfig:      decodeVia(commands.DecodeMotor3DConfig),
	msp.MSPServoMixRules:      decodeVia(commands.DecodeServoMixRules),
	msp.MSPServoConfigurations: decodeVia(commands.DecodeServoConfigurations),
	msp.MSP2CommonSerialConfig: decodeVia(commands.DecodeSerialPortConfigV2),
	msp.MSPCFSerialConfig:     decodeVia(commands.DecodeSerialPortConfigV1),
	msp.MSPVTXConfig:          decodeVia(commands.DecodeVTXConfig),
	msp.MSPOSDConfig:          decodeVia(commands.DecodeOSDConfig),
	msp.MSPOSDCanvas:          decodeVia(commands.DecodeOSDCanvas),
	msp.MSPOSDWarnings:        decodeVia(commands.DecodeOSDWarnings),
	msp.MSPMixerConfig:        decodeVia(commands.DecodeMixerStatus),
	msp.MSPBeeperConfig:       decodeVia(commands.DecodeBeeperConfig),
	msp.MSPTransponderConfig:  decodeVia(commands.DecodeTransponderConfig),
	msp.MSPDebug:              decodeVia(commands.DecodeDebugValues),
	msp.MSPLedStripConfig:     decodeVia(commands.DecodeLEDStripConfig),
	msp.MSPLedColors:          decodeVia(commands.DecodeLEDColors),
	msp.MSPLedStripModecolor:  decodeVia(commands.DecodeLEDModeColors),
	msp.MSP2GetLedStripConfigValues: decodeVia(commands.DecodeLEDConfigValues),
	msp.MSPReboot:             decodeVia(func(payload []byte) (map[string]any, error) {
		out := map[string]any{
			"payload_bytes": bytesAsJSONNumbers(payload),
		}
		if len(payload) > 0 {
			out["boot_mode"] = float64(payload[0])
		}
		if len(payload) > 1 {
			out["state"] = float64(payload[1])
		}
		return out, nil
	}),
	msp.MSPDataflashErase:     decodeVia(func(payload []byte) (map[string]any, error) {
		return map[string]any{
			"payload_bytes": bytesAsJSONNumbers(payload),
		}, nil
	}),
	msp.MSPDisplayport:        decodeRawBytes,
	msp.MSPTxInfo:             decodeRawBytes,
}

var pidFallbackNames = []string{"ROLL", "PITCH", "YAW", "LEVEL", "MAG"}

func decodeStringSlice(fn func([]byte) []string) mspPayloadDecoder {
	return func(payload []byte) (any, error) {
		return fn(payload), nil
	}
}

func decodeByteSlice(fn func([]byte) []uint8) mspPayloadDecoder {
	return func(payload []byte) (any, error) {
		return fn(payload), nil
	}
}

func decodeVia[T any](fn func([]byte) (T, error)) mspPayloadDecoder {
	return func(payload []byte) (any, error) {
		decoded, err := fn(payload)
		if err != nil {
			return nil, err
		}
		return any(decoded), nil
	}
}

func decodeString(fn func([]byte) string) mspPayloadDecoder {
	return func(payload []byte) (any, error) {
		return fn(payload), nil
	}
}

func decodeMSPPIDPayload(payload []byte) (any, error) {
	gains, err := commands.DecodePIDGains(payload, pidFallbackNames)
	if err != nil {
		return nil, err
	}
	type pidPayload struct {
		Names []string          `json:"names"`
		Gains []commands.PIDGain `json:"gains"`
	}
	out := pidPayload{
		Names: make([]string, len(pidFallbackNames)),
		Gains: gains,
	}
	copy(out.Names, pidFallbackNames)
	return out, nil
}

func decodeRawBytes(payload []byte) (any, error) {
	return append([]byte(nil), payload...)
}

func decodeRawAccTrim(payload []byte) (any, error) {
	if len(payload) != 4 {
		return nil, fmt.Errorf("payload is not 4 bytes: %d", len(payload))
	}
	roll := int16(binary.LittleEndian.Uint16(payload[:2]))
	pitch := int16(binary.LittleEndian.Uint16(payload[2:4]))
	return map[string]any{"roll": roll, "pitch": pitch}, nil
}

func decodeMSP2Text(payload []byte) (any, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("payload is shorter than MSP2_GET_TEXT")
	}
	r := msp.NewPayloadReader(payload)
	textType, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "MSP2_GET_TEXT text type")
	}
	text, err := r.PString()
	if err != nil {
		return nil, msp.RequireNoShort(err, "MSP2_GET_TEXT value")
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP2_GET_TEXT returned %d trailing byte(s)", r.Remaining())
	}
	decoded := map[string]any{
		"type":        float64(textType),
		"value":       text,
		"raw":         append([]byte(nil), payload...),
		"text_length": float64(len(text)),
	}
	return decoded, nil
}

func bytesAsJSONNumbers(payload []byte) []any {
	out := make([]any, len(payload))
	for i, raw := range payload {
		out[i] = float64(raw)
	}
	return out
}

func decodeMSPPayload(code uint16, payload []byte) (any, bool, error) {
	decode, ok := mspDecodeRegistry[code]
	if !ok {
		return nil, false, nil
	}
	decoded, err := decode(payload)
	if err != nil {
		return nil, true, err
	}
	return decoded, true, nil
}

func decodeMSPAPIVersionPayload(payload []byte) (any, error) {
	if len(payload) < 3 {
		return nil, fmt.Errorf("payload is shorter than MSP_API_VERSION size %d", len(payload))
	}
	return map[string]any{
		"msp_protocol": payload[0],
		"major":        payload[1],
		"minor":        payload[2],
	}, nil
}

func decodeMSPFCVariantPayload(payload []byte) (any, error) {
	if len(payload) < 4 {
		return nil, fmt.Errorf("payload is shorter than MSP_FC_VARIANT size %d", len(payload))
	}
	return map[string]any{
		"text":  string(payload[:4]),
		"bytes": payload[:4],
	}, nil
}

func decodeMSPFCVersionPayload(payload []byte) (any, error) {
	if len(payload) < 3 {
		return nil, fmt.Errorf("payload is shorter than MSP_FC_VERSION size %d", len(payload))
	}
	out := map[string]any{
		"major": payload[0],
		"minor": payload[1],
		"patch": payload[2],
	}
	if len(payload) >= 4 {
		out["format"] = strings.TrimSpace(string(payload[3:]))
	}
	if format, ok := out["format"].(string); ok && format != "" {
		out["version"] = format
	} else {
		out["version"] = fmt.Sprintf("%d.%d.%d", out["major"], out["minor"], out["patch"])
	}
	return out, nil
}
