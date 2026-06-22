package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type VTXConfig struct {
	Type             uint8            `json:"type"`
	TypeName         string           `json:"type_name,omitempty"`
	Band             uint8            `json:"band"`
	Channel          uint8            `json:"channel"`
	Power            uint8            `json:"power"`
	PitMode          bool             `json:"pit_mode"`
	FrequencyMHz     uint16           `json:"frequency_mhz"`
	DeviceReady      bool             `json:"device_ready"`
	LowPowerDisarm   uint8            `json:"low_power_disarm"`
	PitModeFrequency *uint16          `json:"pit_mode_frequency_mhz,omitempty"`
	Table            *VTXTableSummary `json:"table,omitempty"`
}

type VTXTableSummary struct {
	Available   bool  `json:"available"`
	Bands       uint8 `json:"bands"`
	Channels    uint8 `json:"channels"`
	PowerLevels uint8 `json:"power_levels"`
}

func ReadVTXConfig(ctx context.Context, client *connection.Client) (*VTXConfig, error) {
	frame, err := client.Request(ctx, msp.MSPVTXConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("vtx config unavailable: %w", err)
	}
	return DecodeVTXConfig(frame.Payload)
}

func DecodeVTXConfig(payload []byte) (*VTXConfig, error) {
	r := msp.NewPayloadReader(payload)
	vtxType, err := r.U8()
	if err != nil {
		return nil, err
	}
	band, err := r.U8()
	if err != nil {
		return nil, err
	}
	channel, err := r.U8()
	if err != nil {
		return nil, err
	}
	power, err := r.U8()
	if err != nil {
		return nil, err
	}
	pitMode, err := r.U8()
	if err != nil {
		return nil, err
	}
	frequency, err := r.U16()
	if err != nil {
		return nil, err
	}
	ready, err := r.U8()
	if err != nil {
		return nil, err
	}
	lowPowerDisarm, err := r.U8()
	if err != nil {
		return nil, err
	}
	config := &VTXConfig{
		Type:           vtxType,
		TypeName:       lookupVTXType(vtxType),
		Band:           band,
		Channel:        channel,
		Power:          power,
		PitMode:        pitMode != 0,
		FrequencyMHz:   frequency,
		DeviceReady:    ready != 0,
		LowPowerDisarm: lowPowerDisarm,
	}
	if r.Remaining() >= 2 {
		pitFrequency, err := r.U16()
		if err != nil {
			return nil, err
		}
		config.PitModeFrequency = &pitFrequency
	}
	if r.Remaining() >= 4 {
		available, err := r.U8()
		if err != nil {
			return nil, err
		}
		bands, err := r.U8()
		if err != nil {
			return nil, err
		}
		channels, err := r.U8()
		if err != nil {
			return nil, err
		}
		powerLevels, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.Table = &VTXTableSummary{
			Available:   available != 0,
			Bands:       bands,
			Channels:    channels,
			PowerLevels: powerLevels,
		}
	}
	return config, nil
}

func lookupVTXType(v uint8) string {
	switch v {
	case 0:
		return "UNSUPPORTED"
	case 1:
		return "RTC6705"
	case 3:
		return "SMARTAUDIO"
	case 4:
		return "TRAMP"
	case 5:
		return "MSP"
	case 255:
		return "UNKNOWN"
	default:
		return ""
	}
}
