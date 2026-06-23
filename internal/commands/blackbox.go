package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type BlackboxConfig struct {
	Supported          bool     `json:"supported"`
	Device             uint8    `json:"device"`
	DeviceName         string   `json:"device_name,omitempty"`
	RateNumerator      uint8    `json:"rate_numerator"`
	RateDenominator    uint8    `json:"rate_denominator"`
	PRatio             uint16   `json:"p_ratio"`
	SampleRate         *uint8   `json:"sample_rate,omitempty"`
	SampleRateName     string   `json:"sample_rate_name,omitempty"`
	FieldsDisabledMask *uint32  `json:"fields_disabled_mask,omitempty"`
	DisabledFields     []string `json:"disabled_fields,omitempty"`
	EnabledFields      []string `json:"enabled_fields,omitempty"`
}

type BlackboxConfigSetResult struct {
	Config       BlackboxConfig `json:"config"`
	MSPCode      uint16         `json:"msp_code"`
	MSPName      string         `json:"msp_name"`
	Acknowledged bool           `json:"acknowledged"`
	SaveRequired bool           `json:"save_required"`
}

func ReadBlackboxConfig(ctx context.Context, client *connection.Client) (*BlackboxConfig, error) {
	frame, err := client.Request(ctx, msp.MSPBlackboxConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("blackbox config unavailable: %w", err)
	}
	return DecodeBlackboxConfig(frame.Payload)
}

func SetBlackboxConfig(ctx context.Context, client *connection.Client, config BlackboxConfig) (*BlackboxConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetBlackboxConfig, EncodeBlackboxConfig(config)); err != nil {
		return nil, fmt.Errorf("blackbox config request failed: %w", err)
	}
	return &BlackboxConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetBlackboxConfig,
		MSPName:      "MSP_SET_BLACKBOX_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeBlackboxConfig(config BlackboxConfig) []byte {
	payload := []byte{config.Device, config.RateNumerator, config.RateDenominator}
	payload = appendU16Payload(payload, config.PRatio)
	if config.SampleRate != nil {
		payload = append(payload, *config.SampleRate)
	} else {
		payload = append(payload, 0)
	}
	if config.FieldsDisabledMask != nil {
		payload = appendU32Payload(payload, *config.FieldsDisabledMask)
	} else {
		payload = appendU32Payload(payload, 0)
	}
	return payload
}

func DecodeBlackboxConfig(payload []byte) (*BlackboxConfig, error) {
	r := msp.NewPayloadReader(payload)
	supported, err := r.U8()
	if err != nil {
		return nil, err
	}
	device, err := r.U8()
	if err != nil {
		return nil, err
	}
	rateNum, err := r.U8()
	if err != nil {
		return nil, err
	}
	rateDenom, err := r.U8()
	if err != nil {
		return nil, err
	}
	pRatio, err := r.U16()
	if err != nil {
		return nil, err
	}
	config := &BlackboxConfig{
		Supported:       supported&1 != 0,
		Device:          device,
		DeviceName:      lookupBlackboxDevice(device),
		RateNumerator:   rateNum,
		RateDenominator: rateDenom,
		PRatio:          pRatio,
	}
	if r.Remaining() > 0 {
		sampleRate, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.SampleRate = &sampleRate
		config.SampleRateName = lookupBlackboxSampleRate(sampleRate)
	}
	if r.Remaining() >= 4 {
		mask, err := r.U32()
		if err != nil {
			return nil, err
		}
		config.FieldsDisabledMask = &mask
		config.DisabledFields, config.EnabledFields = blackboxFieldSelection(mask)
	}
	return config, nil
}

func lookupBlackboxDevice(v uint8) string {
	return lookupByIndex(v, []string{"NONE", "SPIFLASH", "SDCARD", "SERIAL", "VIRTUAL"})
}

func lookupBlackboxSampleRate(v uint8) string {
	return lookupByIndex(v, []string{"1/1", "1/2", "1/4", "1/8", "1/16"})
}

func lookupByIndex(v uint8, values []string) string {
	if int(v) >= len(values) {
		return ""
	}
	return values[v]
}

func blackboxFieldSelection(mask uint32) ([]string, []string) {
	var disabled []string
	var enabled []string
	for bit, name := range blackboxSelectableFields {
		if mask&(1<<bit) != 0 {
			disabled = append(disabled, name)
		} else {
			enabled = append(enabled, name)
		}
	}
	return disabled, enabled
}

var blackboxSelectableFields = []string{
	"PID",
	"RC_COMMANDS",
	"SETPOINT",
	"BATTERY",
	"MAG",
	"ALTITUDE",
	"RSSI",
	"GYRO",
	"ATTITUDE",
	"ACC",
	"DEBUG_LOG",
	"MOTOR",
	"GPS",
	"RPM",
	"GYROUNFILT",
	"SERVO",
}
