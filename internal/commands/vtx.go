package commands

import (
	"context"
	"errors"
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

type VTXTableStatus struct {
	Supported         bool                 `json:"supported"`
	UnsupportedReason string               `json:"unsupported_reason,omitempty"`
	Summary           *VTXTableSummary     `json:"summary,omitempty"`
	Bands             []VTXTableBandStatus `json:"bands,omitempty"`
	Powers            []VTXTablePowerLevel `json:"powers,omitempty"`
}

type VTXTableBandStatus struct {
	Band           uint8    `json:"band"`
	Name           string   `json:"name"`
	Letter         string   `json:"letter"`
	Factory        bool     `json:"factory"`
	FrequenciesMHz []uint16 `json:"frequencies_mhz"`
}

type VTXTablePowerLevel struct {
	Level uint8  `json:"level"`
	Value uint16 `json:"value"`
	Label string `json:"label"`
}

type VTXDeviceStatus struct {
	Supported             bool                  `json:"supported"`
	UnsupportedReason     string                `json:"unsupported_reason,omitempty"`
	DevicePresent         bool                  `json:"device_present"`
	Type                  uint8                 `json:"type,omitempty"`
	TypeName              string                `json:"type_name,omitempty"`
	Ready                 bool                  `json:"ready"`
	BandChannelAvailable  bool                  `json:"band_channel_available"`
	Band                  uint8                 `json:"band,omitempty"`
	Channel               uint8                 `json:"channel,omitempty"`
	PowerIndexAvailable   bool                  `json:"power_index_available"`
	PowerIndex            uint8                 `json:"power_index,omitempty"`
	FrequencyAvailable    bool                  `json:"frequency_available"`
	FrequencyMHz          uint16                `json:"frequency_mhz,omitempty"`
	StatusAvailable       bool                  `json:"status_available"`
	StatusRaw             uint32                `json:"status_raw,omitempty"`
	PitMode               bool                  `json:"pit_mode,omitempty"`
	Locked                bool                  `json:"locked,omitempty"`
	PowerLevels           []VTXDevicePowerLevel `json:"power_levels,omitempty"`
	CustomStatusByteCount uint8                 `json:"custom_status_byte_count,omitempty"`
	CustomStatusBytes     []int                 `json:"custom_status_bytes,omitempty"`
}

type VTXDevicePowerLevel struct {
	Level uint16 `json:"level"`
	Power uint16 `json:"power"`
}

type VTXConfigSetConfig struct {
	Band             uint8  `json:"band"`
	Channel          uint8  `json:"channel"`
	Power            uint8  `json:"power"`
	PitMode          bool   `json:"pit_mode"`
	FrequencyMHz     uint16 `json:"frequency_mhz"`
	LowPowerDisarm   uint8  `json:"low_power_disarm"`
	PitModeFrequency uint16 `json:"pit_mode_frequency_mhz"`
}

type VTXConfigSetResult struct {
	Config       VTXConfigSetConfig `json:"config"`
	MSPCode      uint16             `json:"msp_code"`
	MSPName      string             `json:"msp_name"`
	Acknowledged bool               `json:"acknowledged"`
	SaveRequired bool               `json:"save_required"`
}

type VTXTableBandSetConfig struct {
	Band           uint8    `json:"band"`
	Name           string   `json:"name"`
	Letter         string   `json:"letter"`
	Factory        bool     `json:"factory"`
	FrequenciesMHz []uint16 `json:"frequencies_mhz"`
}

type VTXTableBandSetResult struct {
	Config       VTXTableBandSetConfig `json:"config"`
	MSPCode      uint16                `json:"msp_code"`
	MSPName      string                `json:"msp_name"`
	Acknowledged bool                  `json:"acknowledged"`
	SaveRequired bool                  `json:"save_required"`
}

type VTXTablePowerSetConfig struct {
	Level uint8  `json:"level"`
	Value uint16 `json:"value"`
	Label string `json:"label"`
}

type VTXTablePowerSetResult struct {
	Config       VTXTablePowerSetConfig `json:"config"`
	MSPCode      uint16                 `json:"msp_code"`
	MSPName      string                 `json:"msp_name"`
	Acknowledged bool                   `json:"acknowledged"`
	SaveRequired bool                   `json:"save_required"`
}

type VTXTableSetConfig struct {
	Bands  []VTXTableBandSetConfig  `json:"bands,omitempty"`
	Powers []VTXTablePowerSetConfig `json:"powers,omitempty"`
}

type VTXTableSetResult struct {
	Bands        []VTXTableBandSetConfig  `json:"bands,omitempty"`
	Powers       []VTXTablePowerSetConfig `json:"powers,omitempty"`
	BandCount    int                      `json:"band_count"`
	PowerCount   int                      `json:"power_count"`
	MSPNames     []string                 `json:"msp_names"`
	Acknowledged bool                     `json:"acknowledged"`
	SaveRequired bool                     `json:"save_required"`
}

func ReadVTXConfig(ctx context.Context, client *connection.Client) (*VTXConfig, error) {
	frame, err := client.Request(ctx, msp.MSPVTXConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("vtx config unavailable: %w", err)
	}
	return DecodeVTXConfig(frame.Payload)
}

func ReadVTXTableStatus(ctx context.Context, client *connection.Client) (*VTXTableStatus, error) {
	config, err := ReadVTXConfig(ctx, client)
	if err != nil {
		var coded *connection.CodedError
		if errors.As(err, &coded) && coded.Code == "unsupported_msp" {
			return &VTXTableStatus{Supported: false, UnsupportedReason: coded.Message}, nil
		}
		return nil, err
	}
	status := &VTXTableStatus{Supported: true, Summary: config.Table}
	if config.Table == nil || !config.Table.Available {
		return status, nil
	}
	for band := uint8(1); band <= config.Table.Bands; band++ {
		frame, err := client.Request(ctx, msp.MSPVtxtableBand, []byte{band})
		if err != nil {
			return nil, fmt.Errorf("vtx table band %d unavailable: %w", band, err)
		}
		row, err := DecodeVTXTableBand(frame.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode vtx table band %d: %w", band, err)
		}
		status.Bands = append(status.Bands, *row)
	}
	for level := uint8(1); level <= config.Table.PowerLevels; level++ {
		frame, err := client.Request(ctx, msp.MSPVtxtablePowerlevel, []byte{level})
		if err != nil {
			return nil, fmt.Errorf("vtx table power level %d unavailable: %w", level, err)
		}
		row, err := DecodeVTXTablePowerLevel(frame.Payload)
		if err != nil {
			return nil, fmt.Errorf("decode vtx table power level %d: %w", level, err)
		}
		status.Powers = append(status.Powers, *row)
	}
	return status, nil
}

func ReadVTXDeviceStatus(ctx context.Context, client *connection.Client) (*VTXDeviceStatus, error) {
	frame, err := client.Request(ctx, msp.MSP2GetVTXDeviceStatus, nil)
	if err != nil {
		var coded *connection.CodedError
		if errors.As(err, &coded) && coded.Code == "unsupported_msp" {
			return &VTXDeviceStatus{Supported: false, UnsupportedReason: coded.Message}, nil
		}
		return nil, fmt.Errorf("vtx device status unavailable: %w", err)
	}
	status, err := DecodeVTXDeviceStatus(frame.Payload)
	if err != nil {
		return nil, err
	}
	status.Supported = true
	return status, nil
}

func SetVTXConfig(ctx context.Context, client *connection.Client, config VTXConfigSetConfig) (*VTXConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetVTXConfig, EncodeVTXConfig(config)); err != nil {
		return nil, fmt.Errorf("vtx config request failed: %w", err)
	}
	return &VTXConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetVTXConfig,
		MSPName:      "MSP_SET_VTX_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeVTXConfig(config VTXConfigSetConfig) []byte {
	payload := appendU16Payload(nil, config.FrequencyMHz)
	payload = append(payload, config.Power, boolByte(config.PitMode), config.LowPowerDisarm)
	payload = appendU16Payload(payload, config.PitModeFrequency)
	payload = append(payload, config.Band, config.Channel)
	payload = appendU16Payload(payload, config.FrequencyMHz)
	return payload
}

func SetVTXTableBand(ctx context.Context, client *connection.Client, config VTXTableBandSetConfig) (*VTXTableBandSetResult, error) {
	if err := ValidateVTXTableBand(config); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetVtxtableBand, EncodeVTXTableBand(config)); err != nil {
		return nil, fmt.Errorf("vtx table band request failed: %w", err)
	}
	return &VTXTableBandSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetVtxtableBand,
		MSPName:      "MSP_SET_VTXTABLE_BAND",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeVTXTableBand(config VTXTableBandSetConfig) []byte {
	name := []byte(config.Name)
	payload := []byte{config.Band, uint8(len(name))}
	payload = append(payload, name...)
	payload = append(payload, config.Letter[0], boolByte(config.Factory), uint8(len(config.FrequenciesMHz)))
	for _, frequency := range config.FrequenciesMHz {
		payload = appendU16Payload(payload, frequency)
	}
	return payload
}

func SetVTXTablePower(ctx context.Context, client *connection.Client, config VTXTablePowerSetConfig) (*VTXTablePowerSetResult, error) {
	if err := ValidateVTXTablePower(config); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetVtxtablePowerlevel, EncodeVTXTablePower(config)); err != nil {
		return nil, fmt.Errorf("vtx table power request failed: %w", err)
	}
	return &VTXTablePowerSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetVtxtablePowerlevel,
		MSPName:      "MSP_SET_VTXTABLE_POWERLEVEL",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeVTXTablePower(config VTXTablePowerSetConfig) []byte {
	label := []byte(config.Label)
	payload := appendU16Payload([]byte{config.Level}, config.Value)
	payload = append(payload, uint8(len(label)))
	payload = append(payload, label...)
	return payload
}

func SetVTXTable(ctx context.Context, client *connection.Client, config VTXTableSetConfig) (*VTXTableSetResult, error) {
	if err := ValidateVTXTable(config); err != nil {
		return nil, err
	}
	mspNames := make([]string, 0, 2)
	for _, band := range config.Bands {
		if _, err := SetVTXTableBand(ctx, client, band); err != nil {
			return nil, err
		}
	}
	if len(config.Bands) > 0 {
		mspNames = append(mspNames, "MSP_SET_VTXTABLE_BAND")
	}
	for _, power := range config.Powers {
		if _, err := SetVTXTablePower(ctx, client, power); err != nil {
			return nil, err
		}
	}
	if len(config.Powers) > 0 {
		mspNames = append(mspNames, "MSP_SET_VTXTABLE_POWERLEVEL")
	}
	return &VTXTableSetResult{
		Bands:        config.Bands,
		Powers:       config.Powers,
		BandCount:    len(config.Bands),
		PowerCount:   len(config.Powers),
		MSPNames:     mspNames,
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func ValidateVTXTable(config VTXTableSetConfig) error {
	if len(config.Bands) == 0 && len(config.Powers) == 0 {
		return fmt.Errorf("at least one band or power row is required")
	}
	for i, band := range config.Bands {
		if err := ValidateVTXTableBand(band); err != nil {
			return fmt.Errorf("bands[%d]: %w", i, err)
		}
	}
	for i, power := range config.Powers {
		if err := ValidateVTXTablePower(power); err != nil {
			return fmt.Errorf("powers[%d]: %w", i, err)
		}
	}
	return nil
}

func ValidateVTXTableBand(config VTXTableBandSetConfig) error {
	if config.Band == 0 {
		return fmt.Errorf("band must be greater than 0")
	}
	if len(config.Name) == 0 || len(config.Name) > 8 {
		return fmt.Errorf("name must be 1-8 bytes")
	}
	if len(config.Letter) != 1 {
		return fmt.Errorf("letter must be exactly 1 byte")
	}
	if len(config.FrequenciesMHz) == 0 || len(config.FrequenciesMHz) > 8 {
		return fmt.Errorf("frequencies_mhz must contain 1-8 values")
	}
	return nil
}

func ValidateVTXTablePower(config VTXTablePowerSetConfig) error {
	if config.Level == 0 {
		return fmt.Errorf("level must be greater than 0")
	}
	if len(config.Label) == 0 || len(config.Label) > 3 {
		return fmt.Errorf("label must be 1-3 bytes")
	}
	return nil
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

func DecodeVTXTableBand(payload []byte) (*VTXTableBandStatus, error) {
	r := msp.NewPayloadReader(payload)
	band, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table band")
	}
	nameLen, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table band name length")
	}
	nameBytes, err := r.Bytes(int(nameLen))
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table band name")
	}
	letter, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table band letter")
	}
	factory, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table band factory flag")
	}
	channelCount, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table band channel count")
	}
	frequencies := make([]uint16, 0, channelCount)
	for i := uint8(0); i < channelCount; i++ {
		frequency, err := r.U16()
		if err != nil {
			return nil, msp.RequireNoShort(err, "VTX table band frequency")
		}
		frequencies = append(frequencies, frequency)
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("VTX table band returned %d trailing byte(s)", r.Remaining())
	}
	return &VTXTableBandStatus{
		Band:           band,
		Name:           string(nameBytes),
		Letter:         string([]byte{letter}),
		Factory:        factory != 0,
		FrequenciesMHz: frequencies,
	}, nil
}

func DecodeVTXTablePowerLevel(payload []byte) (*VTXTablePowerLevel, error) {
	r := msp.NewPayloadReader(payload)
	level, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table power level")
	}
	value, err := r.U16()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table power value")
	}
	labelLen, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table power label length")
	}
	labelBytes, err := r.Bytes(int(labelLen))
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX table power label")
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("VTX table power level returned %d trailing byte(s)", r.Remaining())
	}
	return &VTXTablePowerLevel{
		Level: level,
		Value: value,
		Label: string(labelBytes),
	}, nil
}

func DecodeVTXDeviceStatus(payload []byte) (*VTXDeviceStatus, error) {
	if len(payload) == 0 {
		return &VTXDeviceStatus{Supported: true, DevicePresent: false}, nil
	}
	r := msp.NewPayloadReader(payload)
	vtxType, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX device type")
	}
	ready, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX device readiness")
	}
	bandChannelAvailable, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX band/channel availability")
	}
	band, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX band")
	}
	channel, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX channel")
	}
	powerIndexAvailable, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX power index availability")
	}
	powerIndex, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX power index")
	}
	frequencyAvailable, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX frequency availability")
	}
	frequency, err := r.U16()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX frequency")
	}
	statusAvailable, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX status availability")
	}
	statusRaw, err := r.U32()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX status")
	}
	powerLevelCount, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX power level count")
	}
	powerLevels := make([]VTXDevicePowerLevel, 0, powerLevelCount)
	for i := uint8(0); i < powerLevelCount; i++ {
		level, err := r.U16()
		if err != nil {
			return nil, msp.RequireNoShort(err, "VTX power level")
		}
		power, err := r.U16()
		if err != nil {
			return nil, msp.RequireNoShort(err, "VTX power value")
		}
		powerLevels = append(powerLevels, VTXDevicePowerLevel{Level: level, Power: power})
	}
	customCount, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX custom status byte count")
	}
	customBytesRaw, err := r.Bytes(int(customCount))
	if err != nil {
		return nil, msp.RequireNoShort(err, "VTX custom status bytes")
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("VTX device status returned %d trailing byte(s)", r.Remaining())
	}
	customBytes := make([]int, len(customBytesRaw))
	for i, value := range customBytesRaw {
		customBytes[i] = int(value)
	}
	return &VTXDeviceStatus{
		Supported:             true,
		DevicePresent:         true,
		Type:                  vtxType,
		TypeName:              lookupVTXType(vtxType),
		Ready:                 ready != 0,
		BandChannelAvailable:  bandChannelAvailable != 0,
		Band:                  band,
		Channel:               channel,
		PowerIndexAvailable:   powerIndexAvailable != 0,
		PowerIndex:            powerIndex,
		FrequencyAvailable:    frequencyAvailable != 0,
		FrequencyMHz:          frequency,
		StatusAvailable:       statusAvailable != 0,
		StatusRaw:             statusRaw,
		PitMode:               statusRaw&1 != 0,
		Locked:                statusRaw&2 != 0,
		PowerLevels:           powerLevels,
		CustomStatusByteCount: customCount,
		CustomStatusBytes:     customBytes,
	}, nil
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
