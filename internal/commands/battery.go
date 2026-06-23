package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const (
	batteryConfigLength    = 13
	batteryProfileLength   = 13
	batteryStateLength     = 11
	voltageMeterSize       = 2
	currentMeterSize       = 5
	voltageMeterConfigSize = 5
	currentMeterConfigSize = 6
)

var (
	voltageMeterSourceNames = []string{"NONE", "ADC", "ESC"}
	currentMeterSourceNames = []string{"NONE", "ADC", "VIRTUAL", "ESC", "MSP"}
	voltageSensorTypeNames  = []string{"ADC_RESISTOR_DIVIDER", "ESC"}
	currentSensorTypeNames  = []string{"VIRTUAL", "ADC", "ESC", "MSP"}
	batteryStateNames       = []string{"OK", "WARNING", "CRITICAL", "NOT_PRESENT", "INIT"}
)

type BatteryStatus struct {
	Config              *BatteryConfig       `json:"config,omitempty"`
	Profile             *BatteryProfile      `json:"profile,omitempty"`
	State               *BatteryRuntimeState `json:"state,omitempty"`
	VoltageMeters       []VoltageMeter       `json:"voltage_meters,omitempty"`
	CurrentMeters       []CurrentMeter       `json:"current_meters,omitempty"`
	VoltageMeterConfigs []VoltageMeterConfig `json:"voltage_meter_configs,omitempty"`
	CurrentMeterConfigs []CurrentMeterConfig `json:"current_meter_configs,omitempty"`
	Sources             map[string]string    `json:"sources,omitempty"`
}

type BatteryConfig struct {
	LegacyMinCellVoltageV     float64 `json:"legacy_min_cell_voltage_v"`
	LegacyMaxCellVoltageV     float64 `json:"legacy_max_cell_voltage_v"`
	LegacyWarningCellVoltageV float64 `json:"legacy_warning_cell_voltage_v"`
	CapacityMAh               uint16  `json:"capacity_mah"`
	VoltageMeterSource        uint8   `json:"voltage_meter_source"`
	VoltageMeterSourceName    string  `json:"voltage_meter_source_name,omitempty"`
	CurrentMeterSource        uint8   `json:"current_meter_source"`
	CurrentMeterSourceName    string  `json:"current_meter_source_name,omitempty"`
	MinCellVoltageV           float64 `json:"min_cell_voltage_v"`
	MaxCellVoltageV           float64 `json:"max_cell_voltage_v"`
	WarningCellVoltageV       float64 `json:"warning_cell_voltage_v"`
	TrailingBytesIgnored      int     `json:"trailing_bytes_ignored,omitempty"`
}

type BatteryConfigSetResult struct {
	Config       BatteryConfig `json:"config"`
	MSPCode      uint16        `json:"msp_code"`
	MSPName      string        `json:"msp_name"`
	Acknowledged bool          `json:"acknowledged"`
	SaveRequired bool          `json:"save_required"`
}

type BatteryProfile struct {
	Index                     uint8   `json:"index"`
	MinCellVoltageV           float64 `json:"min_cell_voltage_v"`
	MaxCellVoltageV           float64 `json:"max_cell_voltage_v"`
	WarningCellVoltageV       float64 `json:"warning_cell_voltage_v"`
	FullCellVoltageV          float64 `json:"full_cell_voltage_v"`
	CapacityMAh               uint16  `json:"capacity_mah"`
	ForceCellCount            uint8   `json:"force_cell_count"`
	ConsumptionWarningPercent uint8   `json:"consumption_warning_percent"`
	TrailingBytesIgnored      int     `json:"trailing_bytes_ignored,omitempty"`
}

type BatteryProfileSetResult struct {
	Profile      BatteryProfile `json:"profile"`
	MSPCode      uint16         `json:"msp_code"`
	MSPName      string         `json:"msp_name"`
	Acknowledged bool           `json:"acknowledged"`
	SaveRequired bool           `json:"save_required"`
}

type BatteryRuntimeState struct {
	CellCount            uint8   `json:"cell_count"`
	CapacityMAh          uint16  `json:"capacity_mah"`
	VoltageLegacyV       float64 `json:"voltage_legacy_v"`
	DrawnMAh             uint16  `json:"drawn_mah"`
	AmperageA            float64 `json:"amperage_a"`
	State                uint8   `json:"state"`
	StateName            string  `json:"state_name,omitempty"`
	VoltageV             float64 `json:"voltage_v"`
	TrailingBytesIgnored int     `json:"trailing_bytes_ignored,omitempty"`
}

type VoltageMeter struct {
	ID       uint8   `json:"id"`
	IDName   string  `json:"id_name,omitempty"`
	VoltageV float64 `json:"voltage_v"`
}

type CurrentMeter struct {
	ID        uint8   `json:"id"`
	IDName    string  `json:"id_name,omitempty"`
	DrawnMAh  uint16  `json:"drawn_mah"`
	AmperageA float64 `json:"amperage_a"`
}

type VoltageMeterConfig struct {
	ID                   uint8  `json:"id"`
	IDName               string `json:"id_name,omitempty"`
	SensorType           uint8  `json:"sensor_type"`
	SensorTypeName       string `json:"sensor_type_name,omitempty"`
	VBATScale            uint8  `json:"vbat_scale"`
	VBATResDivVal        uint8  `json:"vbat_res_div_val"`
	VBATResDivMultiplier uint8  `json:"vbat_res_div_multiplier"`
}

type VoltageMeterConfigSetResult struct {
	Config       VoltageMeterConfig `json:"config"`
	MSPCode      uint16             `json:"msp_code"`
	MSPName      string             `json:"msp_name"`
	Acknowledged bool               `json:"acknowledged"`
	SaveRequired bool               `json:"save_required"`
}

type CurrentMeterConfig struct {
	ID             uint8  `json:"id"`
	IDName         string `json:"id_name,omitempty"`
	SensorType     uint8  `json:"sensor_type"`
	SensorTypeName string `json:"sensor_type_name,omitempty"`
	Scale          int16  `json:"scale"`
	Offset         int16  `json:"offset"`
}

type CurrentMeterConfigSetResult struct {
	Config       CurrentMeterConfig `json:"config"`
	MSPCode      uint16             `json:"msp_code"`
	MSPName      string             `json:"msp_name"`
	Acknowledged bool               `json:"acknowledged"`
	SaveRequired bool               `json:"save_required"`
}

func ReadBatteryStatus(ctx context.Context, client *connection.Client) (*BatteryStatus, []string, error) {
	status := &BatteryStatus{Sources: map[string]string{}}
	warnings := []string{}

	if config, err := readBatteryConfig(ctx, client); err == nil {
		status.Config = config
		status.Sources["config"] = "MSP_BATTERY_CONFIG"
	} else {
		warnings = append(warnings, err.Error())
	}
	if profile, err := readBatteryProfile(ctx, client); err == nil {
		status.Profile = profile
		status.Sources["profile"] = "MSP2_BATTERY_PROFILE"
	} else {
		warnings = append(warnings, err.Error())
	}
	if state, err := readBatteryState(ctx, client); err == nil {
		status.State = state
		status.Sources["state"] = "MSP_BATTERY_STATE"
	} else {
		warnings = append(warnings, err.Error())
	}
	if meters, err := readVoltageMeters(ctx, client); err == nil {
		status.VoltageMeters = meters
		status.Sources["voltage_meters"] = "MSP_VOLTAGE_METERS"
	} else {
		warnings = append(warnings, err.Error())
	}
	if meters, err := readCurrentMeters(ctx, client); err == nil {
		status.CurrentMeters = meters
		status.Sources["current_meters"] = "MSP_CURRENT_METERS"
	} else {
		warnings = append(warnings, err.Error())
	}
	if configs, err := readVoltageMeterConfigs(ctx, client); err == nil {
		status.VoltageMeterConfigs = configs
		status.Sources["voltage_meter_configs"] = "MSP_VOLTAGE_METER_CONFIG"
	} else {
		warnings = append(warnings, err.Error())
	}
	if configs, err := readCurrentMeterConfigs(ctx, client); err == nil {
		status.CurrentMeterConfigs = configs
		status.Sources["current_meter_configs"] = "MSP_CURRENT_METER_CONFIG"
	} else {
		warnings = append(warnings, err.Error())
	}
	if status.Config == nil && status.Profile == nil && status.State == nil {
		return nil, warnings, fmt.Errorf("battery status unavailable")
	}
	return status, warnings, nil
}

func SetBatteryConfig(ctx context.Context, client *connection.Client, config BatteryConfig) (*BatteryConfigSetResult, error) {
	if err := ValidateBatteryConfig(config); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetBatteryConfig, EncodeBatteryConfig(config)); err != nil {
		return nil, fmt.Errorf("battery config request failed: %w", err)
	}
	config.VoltageMeterSourceName = indexedName(voltageMeterSourceNames, config.VoltageMeterSource)
	config.CurrentMeterSourceName = indexedName(currentMeterSourceNames, config.CurrentMeterSource)
	return &BatteryConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetBatteryConfig,
		MSPName:      "MSP_SET_BATTERY_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func ValidateBatteryConfig(config BatteryConfig) error {
	if config.MinCellVoltageV > config.WarningCellVoltageV || config.WarningCellVoltageV > config.MaxCellVoltageV {
		return fmt.Errorf("cell voltages must satisfy min_cell_voltage_v <= warning_cell_voltage_v <= max_cell_voltage_v")
	}
	return nil
}

func EncodeBatteryConfig(config BatteryConfig) []byte {
	minCell := centivolts(config.MinCellVoltageV)
	maxCell := centivolts(config.MaxCellVoltageV)
	warnCell := centivolts(config.WarningCellVoltageV)
	payload := []byte{
		decivoltsByte(config.LegacyMinCellVoltageV, minCell),
		decivoltsByte(config.LegacyMaxCellVoltageV, maxCell),
		decivoltsByte(config.LegacyWarningCellVoltageV, warnCell),
	}
	payload = appendU16Payload(payload, config.CapacityMAh)
	payload = append(payload, config.VoltageMeterSource, config.CurrentMeterSource)
	payload = appendU16Payload(payload, minCell)
	payload = appendU16Payload(payload, maxCell)
	payload = appendU16Payload(payload, warnCell)
	return payload
}

func SetBatteryProfile(ctx context.Context, client *connection.Client, profile BatteryProfile) (*BatteryProfileSetResult, error) {
	if err := ValidateBatteryProfile(profile); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSP2SetBatteryProfile, EncodeBatteryProfile(profile)); err != nil {
		return nil, fmt.Errorf("battery profile request failed: %w", err)
	}
	return &BatteryProfileSetResult{
		Profile:      profile,
		MSPCode:      msp.MSP2SetBatteryProfile,
		MSPName:      "MSP2_SET_BATTERY_PROFILE",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func ValidateBatteryProfile(profile BatteryProfile) error {
	if profile.MinCellVoltageV > profile.WarningCellVoltageV || profile.WarningCellVoltageV > profile.FullCellVoltageV || profile.FullCellVoltageV > profile.MaxCellVoltageV {
		return fmt.Errorf("cell voltages must satisfy min_cell_voltage_v <= warning_cell_voltage_v <= full_cell_voltage_v <= max_cell_voltage_v")
	}
	if profile.ForceCellCount > 24 {
		return fmt.Errorf("force_cell_count must be <= 24")
	}
	if profile.ConsumptionWarningPercent > 100 {
		return fmt.Errorf("consumption_warning_percent must be <= 100")
	}
	return nil
}

func EncodeBatteryProfile(profile BatteryProfile) []byte {
	payload := []byte{profile.Index}
	payload = appendU16Payload(payload, centivolts(profile.MinCellVoltageV))
	payload = appendU16Payload(payload, centivolts(profile.MaxCellVoltageV))
	payload = appendU16Payload(payload, centivolts(profile.WarningCellVoltageV))
	payload = appendU16Payload(payload, centivolts(profile.FullCellVoltageV))
	payload = appendU16Payload(payload, profile.CapacityMAh)
	payload = append(payload, profile.ForceCellCount, profile.ConsumptionWarningPercent)
	return payload
}

func centivolts(value float64) uint16 {
	if value <= 0 {
		return 0
	}
	return uint16(value*100 + 0.5)
}

func decivoltsByte(value float64, centivoltFallback uint16) uint8 {
	if value <= 0 {
		return uint8((centivoltFallback + 5) / 10)
	}
	return uint8(value*10 + 0.5)
}

func SetVoltageMeterConfig(ctx context.Context, client *connection.Client, config VoltageMeterConfig) (*VoltageMeterConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetVoltageMeterConfig, EncodeVoltageMeterConfig(config)); err != nil {
		return nil, fmt.Errorf("voltage meter config request failed: %w", err)
	}
	config.IDName = voltageMeterIDName(config.ID)
	config.SensorTypeName = indexedName(voltageSensorTypeNames, config.SensorType)
	return &VoltageMeterConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetVoltageMeterConfig,
		MSPName:      "MSP_SET_VOLTAGE_METER_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeVoltageMeterConfig(config VoltageMeterConfig) []byte {
	return []byte{config.ID, config.VBATScale, config.VBATResDivVal, config.VBATResDivMultiplier}
}

func SetCurrentMeterConfig(ctx context.Context, client *connection.Client, config CurrentMeterConfig) (*CurrentMeterConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetCurrentMeterConfig, EncodeCurrentMeterConfig(config)); err != nil {
		return nil, fmt.Errorf("current meter config request failed: %w", err)
	}
	config.IDName = currentMeterIDName(config.ID)
	config.SensorTypeName = indexedName(currentSensorTypeNames, config.SensorType)
	return &CurrentMeterConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetCurrentMeterConfig,
		MSPName:      "MSP_SET_CURRENT_METER_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeCurrentMeterConfig(config CurrentMeterConfig) []byte {
	payload := []byte{config.ID}
	payload = append(payload, byte(config.Scale), byte(uint16(config.Scale)>>8))
	payload = append(payload, byte(config.Offset), byte(uint16(config.Offset)>>8))
	return payload
}

func DecodeBatteryConfig(payload []byte) (*BatteryConfig, error) {
	if len(payload) < batteryConfigLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_BATTERY_CONFIG size %d", len(payload), batteryConfigLength)
	}
	r := msp.NewPayloadReader(payload)
	legacyMin, err := r.U8()
	if err != nil {
		return nil, err
	}
	legacyMax, err := r.U8()
	if err != nil {
		return nil, err
	}
	legacyWarn, err := r.U8()
	if err != nil {
		return nil, err
	}
	capacity, err := r.U16()
	if err != nil {
		return nil, err
	}
	voltageSource, err := r.U8()
	if err != nil {
		return nil, err
	}
	currentSource, err := r.U8()
	if err != nil {
		return nil, err
	}
	minCell, err := r.U16()
	if err != nil {
		return nil, err
	}
	maxCell, err := r.U16()
	if err != nil {
		return nil, err
	}
	warnCell, err := r.U16()
	if err != nil {
		return nil, err
	}
	return &BatteryConfig{
		LegacyMinCellVoltageV:     float64(legacyMin) / 10,
		LegacyMaxCellVoltageV:     float64(legacyMax) / 10,
		LegacyWarningCellVoltageV: float64(legacyWarn) / 10,
		CapacityMAh:               capacity,
		VoltageMeterSource:        voltageSource,
		VoltageMeterSourceName:    indexedName(voltageMeterSourceNames, voltageSource),
		CurrentMeterSource:        currentSource,
		CurrentMeterSourceName:    indexedName(currentMeterSourceNames, currentSource),
		MinCellVoltageV:           float64(minCell) / 100,
		MaxCellVoltageV:           float64(maxCell) / 100,
		WarningCellVoltageV:       float64(warnCell) / 100,
		TrailingBytesIgnored:      r.Remaining(),
	}, nil
}

func DecodeBatteryProfile(payload []byte) (*BatteryProfile, error) {
	if len(payload) < batteryProfileLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP2_BATTERY_PROFILE size %d", len(payload), batteryProfileLength)
	}
	r := msp.NewPayloadReader(payload)
	index, err := r.U8()
	if err != nil {
		return nil, err
	}
	minCell, err := r.U16()
	if err != nil {
		return nil, err
	}
	maxCell, err := r.U16()
	if err != nil {
		return nil, err
	}
	warnCell, err := r.U16()
	if err != nil {
		return nil, err
	}
	fullCell, err := r.U16()
	if err != nil {
		return nil, err
	}
	capacity, err := r.U16()
	if err != nil {
		return nil, err
	}
	forceCells, err := r.U8()
	if err != nil {
		return nil, err
	}
	warnPercent, err := r.U8()
	if err != nil {
		return nil, err
	}
	return &BatteryProfile{
		Index:                     index,
		MinCellVoltageV:           float64(minCell) / 100,
		MaxCellVoltageV:           float64(maxCell) / 100,
		WarningCellVoltageV:       float64(warnCell) / 100,
		FullCellVoltageV:          float64(fullCell) / 100,
		CapacityMAh:               capacity,
		ForceCellCount:            forceCells,
		ConsumptionWarningPercent: warnPercent,
		TrailingBytesIgnored:      r.Remaining(),
	}, nil
}

func DecodeBatteryRuntimeState(payload []byte) (*BatteryRuntimeState, error) {
	if len(payload) < batteryStateLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_BATTERY_STATE size %d", len(payload), batteryStateLength)
	}
	r := msp.NewPayloadReader(payload)
	cellCount, err := r.U8()
	if err != nil {
		return nil, err
	}
	capacity, err := r.U16()
	if err != nil {
		return nil, err
	}
	legacyVoltage, err := r.U8()
	if err != nil {
		return nil, err
	}
	drawn, err := r.U16()
	if err != nil {
		return nil, err
	}
	amps, err := r.S16()
	if err != nil {
		return nil, err
	}
	state, err := r.U8()
	if err != nil {
		return nil, err
	}
	voltage, err := r.U16()
	if err != nil {
		return nil, err
	}
	return &BatteryRuntimeState{
		CellCount:            cellCount,
		CapacityMAh:          capacity,
		VoltageLegacyV:       float64(legacyVoltage) / 10,
		DrawnMAh:             drawn,
		AmperageA:            float64(amps) / 100,
		State:                state,
		StateName:            indexedName(batteryStateNames, state),
		VoltageV:             float64(voltage) / 100,
		TrailingBytesIgnored: r.Remaining(),
	}, nil
}

func DecodeVoltageMeters(payload []byte) ([]VoltageMeter, error) {
	if len(payload)%voltageMeterSize != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of voltage meter size %d", len(payload), voltageMeterSize)
	}
	r := msp.NewPayloadReader(payload)
	meters := make([]VoltageMeter, 0, len(payload)/voltageMeterSize)
	for r.Remaining() > 0 {
		id, err := r.U8()
		if err != nil {
			return nil, err
		}
		voltage, err := r.U8()
		if err != nil {
			return nil, err
		}
		meters = append(meters, VoltageMeter{ID: id, IDName: voltageMeterIDName(id), VoltageV: float64(voltage) / 10})
	}
	return meters, nil
}

func DecodeCurrentMeters(payload []byte) ([]CurrentMeter, error) {
	if len(payload)%currentMeterSize != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of current meter size %d", len(payload), currentMeterSize)
	}
	r := msp.NewPayloadReader(payload)
	meters := make([]CurrentMeter, 0, len(payload)/currentMeterSize)
	for r.Remaining() > 0 {
		id, err := r.U8()
		if err != nil {
			return nil, err
		}
		drawn, err := r.U16()
		if err != nil {
			return nil, err
		}
		amps, err := r.U16()
		if err != nil {
			return nil, err
		}
		meters = append(meters, CurrentMeter{ID: id, IDName: currentMeterIDName(id), DrawnMAh: drawn, AmperageA: float64(amps) / 1000})
	}
	return meters, nil
}

func DecodeVoltageMeterConfigs(payload []byte) ([]VoltageMeterConfig, error) {
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, err
	}
	configs := []VoltageMeterConfig{}
	for i := 0; i < int(count); i++ {
		size, err := r.U8()
		if err != nil {
			return nil, err
		}
		if size != voltageMeterConfigSize {
			if _, err := r.Bytes(int(size)); err != nil {
				return nil, err
			}
			continue
		}
		id, err := r.U8()
		if err != nil {
			return nil, err
		}
		sensorType, err := r.U8()
		if err != nil {
			return nil, err
		}
		scale, err := r.U8()
		if err != nil {
			return nil, err
		}
		resDiv, err := r.U8()
		if err != nil {
			return nil, err
		}
		multiplier, err := r.U8()
		if err != nil {
			return nil, err
		}
		configs = append(configs, VoltageMeterConfig{
			ID:                   id,
			IDName:               voltageMeterIDName(id),
			SensorType:           sensorType,
			SensorTypeName:       indexedName(voltageSensorTypeNames, sensorType),
			VBATScale:            scale,
			VBATResDivVal:        resDiv,
			VBATResDivMultiplier: multiplier,
		})
	}
	return configs, nil
}

func DecodeCurrentMeterConfigs(payload []byte) ([]CurrentMeterConfig, error) {
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, err
	}
	configs := []CurrentMeterConfig{}
	for i := 0; i < int(count); i++ {
		size, err := r.U8()
		if err != nil {
			return nil, err
		}
		if size != currentMeterConfigSize {
			if _, err := r.Bytes(int(size)); err != nil {
				return nil, err
			}
			continue
		}
		id, err := r.U8()
		if err != nil {
			return nil, err
		}
		sensorType, err := r.U8()
		if err != nil {
			return nil, err
		}
		scale, err := r.S16()
		if err != nil {
			return nil, err
		}
		offset, err := r.S16()
		if err != nil {
			return nil, err
		}
		configs = append(configs, CurrentMeterConfig{
			ID:             id,
			IDName:         currentMeterIDName(id),
			SensorType:     sensorType,
			SensorTypeName: indexedName(currentSensorTypeNames, sensorType),
			Scale:          scale,
			Offset:         offset,
		})
	}
	return configs, nil
}

func readBatteryConfig(ctx context.Context, client *connection.Client) (*BatteryConfig, error) {
	frame, err := client.Request(ctx, msp.MSPBatteryConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("battery config unavailable: %w", err)
	}
	config, err := DecodeBatteryConfig(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("battery config decode failed: %w", err)
	}
	return config, nil
}

func readBatteryProfile(ctx context.Context, client *connection.Client) (*BatteryProfile, error) {
	frame, err := client.Request(ctx, msp.MSP2BatteryProfile, nil)
	if err != nil {
		return nil, fmt.Errorf("battery profile unavailable: %w", err)
	}
	profile, err := DecodeBatteryProfile(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("battery profile decode failed: %w", err)
	}
	return profile, nil
}

func readBatteryState(ctx context.Context, client *connection.Client) (*BatteryRuntimeState, error) {
	frame, err := client.Request(ctx, msp.MSPBatteryState, nil)
	if err != nil {
		return nil, fmt.Errorf("battery state unavailable: %w", err)
	}
	state, err := DecodeBatteryRuntimeState(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("battery state decode failed: %w", err)
	}
	return state, nil
}

func readVoltageMeters(ctx context.Context, client *connection.Client) ([]VoltageMeter, error) {
	frame, err := client.Request(ctx, msp.MSPVoltageMeters, nil)
	if err != nil {
		return nil, fmt.Errorf("voltage meters unavailable: %w", err)
	}
	meters, err := DecodeVoltageMeters(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("voltage meters decode failed: %w", err)
	}
	return meters, nil
}

func readCurrentMeters(ctx context.Context, client *connection.Client) ([]CurrentMeter, error) {
	frame, err := client.Request(ctx, msp.MSPCurrentMeters, nil)
	if err != nil {
		return nil, fmt.Errorf("current meters unavailable: %w", err)
	}
	meters, err := DecodeCurrentMeters(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("current meters decode failed: %w", err)
	}
	return meters, nil
}

func readVoltageMeterConfigs(ctx context.Context, client *connection.Client) ([]VoltageMeterConfig, error) {
	frame, err := client.Request(ctx, msp.MSPVoltageMeterConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("voltage meter config unavailable: %w", err)
	}
	configs, err := DecodeVoltageMeterConfigs(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("voltage meter config decode failed: %w", err)
	}
	return configs, nil
}

func readCurrentMeterConfigs(ctx context.Context, client *connection.Client) ([]CurrentMeterConfig, error) {
	frame, err := client.Request(ctx, msp.MSPCurrentMeterConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("current meter config unavailable: %w", err)
	}
	configs, err := DecodeCurrentMeterConfigs(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("current meter config decode failed: %w", err)
	}
	return configs, nil
}

func voltageMeterIDName(id uint8) string {
	switch {
	case id == 0:
		return "NONE"
	case id >= 10 && id <= 19:
		return fmt.Sprintf("BATTERY_%d", id-9)
	case id >= 20 && id <= 29:
		return fmt.Sprintf("5V_%d", id-19)
	case id >= 30 && id <= 39:
		return fmt.Sprintf("9V_%d", id-29)
	case id >= 40 && id <= 49:
		return fmt.Sprintf("12V_%d", id-39)
	case id >= 50 && id <= 59:
		return fmt.Sprintf("ESC_COMBINED_%d", id-49)
	case id >= 60 && id <= 79:
		return fmt.Sprintf("ESC_MOTOR_%d", id-59)
	case id >= 80 && id <= 119:
		return fmt.Sprintf("CELL_%d", id-79)
	default:
		return ""
	}
}

func currentMeterIDName(id uint8) string {
	switch {
	case id == 0:
		return "NONE"
	case id >= 10 && id <= 19:
		return fmt.Sprintf("BATTERY_%d", id-9)
	case id >= 20 && id <= 29:
		return fmt.Sprintf("5V_%d", id-19)
	case id >= 30 && id <= 39:
		return fmt.Sprintf("9V_%d", id-29)
	case id >= 40 && id <= 49:
		return fmt.Sprintf("12V_%d", id-39)
	case id >= 50 && id <= 59:
		return fmt.Sprintf("ESC_COMBINED_%d", id-49)
	case id >= 60 && id <= 79:
		return fmt.Sprintf("ESC_MOTOR_%d", id-59)
	case id >= 80 && id <= 89:
		return fmt.Sprintf("VIRTUAL_%d", id-79)
	case id >= 90 && id <= 99:
		return fmt.Sprintf("MSP_%d", id-89)
	default:
		return ""
	}
}
