package cli

import (
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
	msp.MSPBatteryState:       decodeVia(commands.DecodeBatteryRuntimeState),
	msp.MSPSonarAltitude:      decodeVia(commands.DecodeRangefinderAltitude),
	msp.MSPAnalog:             decodeVia(commands.DecodeAnalog),
	msp.MSPRC:                 decodeVia(commands.DecodeRC),
	msp.MSPRtc:                decodeVia(commands.DecodeRTC),
	msp.MSPVTXConfig:          decodeVia(commands.DecodeVTXConfig),
	msp.MSPOSDConfig:          decodeVia(commands.DecodeOSDConfig),
	msp.MSPMixerConfig:        decodeVia(commands.DecodeMixerStatus),
	msp.MSPArmingConfig:       decodeVia(commands.DecodeArmingConfig),
	msp.MSPFailsafeConfig:     decodeVia(commands.DecodeFailsafeConfig),
	msp.MSPBoardAlignmentConfig: decodeVia(commands.DecodeBoardAlignment),
	msp.MSPTransponderConfig:  decodeVia(commands.DecodeTransponderConfig),
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
