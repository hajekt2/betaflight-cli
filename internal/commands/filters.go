package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const (
	advancedConfigModernLength = 20
	filterConfigModernLength   = 56
)

var (
	lowpassTypeNames       = []string{"PT1", "BIQUAD", "PT2", "PT3"}
	gyroHardwareLPFNames   = []string{"NORMAL", "OPTION_1", "OPTION_2", "EXPERIMENTAL"}
	motorProtocolNames     = []string{"PWM", "ONESHOT125", "ONESHOT42", "MULTISHOT", "BRUSHED", "DSHOT150", "DSHOT300", "DSHOT600", "PROSHOT1000", "DISABLED"}
	gyroOverflowCheckNames = []string{"OFF", "YAW", "ALL"}
)

type FilterStatus struct {
	AdvancedConfig *AdvancedConfig   `json:"advanced_config,omitempty"`
	FilterConfig   *FilterConfig     `json:"filter_config,omitempty"`
	Sources        map[string]string `json:"sources,omitempty"`
}

type AdvancedConfig struct {
	GyroSyncDenom              uint8   `json:"gyro_sync_denom"`
	PIDProcessDenom            uint8   `json:"pid_process_denom"`
	UseUnsyncedPWM             uint8   `json:"use_unsynced_pwm"`
	MotorProtocol              uint8   `json:"motor_protocol"`
	MotorProtocolName          string  `json:"motor_protocol_name,omitempty"`
	MotorPWMRate               uint16  `json:"motor_pwm_rate"`
	MotorIdle                  uint16  `json:"motor_idle"`
	MotorIdlePercent           float64 `json:"motor_idle_percent"`
	GyroUse32KHzDeprecated     uint8   `json:"gyro_use_32khz_deprecated"`
	MotorPWMInversion          uint8   `json:"motor_pwm_inversion"`
	GyroToUseDeprecated        uint8   `json:"gyro_to_use_deprecated"`
	GyroHighFSR                uint8   `json:"gyro_high_fsr"`
	GyroMovementCalibThreshold uint8   `json:"gyro_movement_calib_threshold"`
	GyroCalibDuration          uint16  `json:"gyro_calib_duration"`
	GyroOffsetYaw              uint16  `json:"gyro_offset_yaw"`
	GyroCheckOverflow          uint8   `json:"gyro_check_overflow"`
	GyroCheckOverflowName      string  `json:"gyro_check_overflow_name,omitempty"`
	DebugMode                  uint8   `json:"debug_mode"`
	DebugModeCount             uint8   `json:"debug_mode_count"`
	TrailingBytesIgnored       int     `json:"trailing_bytes_ignored,omitempty"`
}

type AdvancedConfigSetResult struct {
	Config       AdvancedConfig `json:"advanced_config"`
	MSPCode      uint16         `json:"msp_code"`
	MSPName      string         `json:"msp_name"`
	Acknowledged bool           `json:"acknowledged"`
	SaveRequired bool           `json:"save_required"`
}

type FilterConfig struct {
	LegacyGyroLowpassHz  uint8          `json:"legacy_gyro_lowpass_hz"`
	GyroLPF1StaticHz     uint16         `json:"gyro_lpf1_static_hz"`
	GyroLPF2StaticHz     uint16         `json:"gyro_lpf2_static_hz"`
	GyroLPF1Type         uint8          `json:"gyro_lpf1_type"`
	GyroLPF1TypeName     string         `json:"gyro_lpf1_type_name,omitempty"`
	GyroLPF2Type         uint8          `json:"gyro_lpf2_type"`
	GyroLPF2TypeName     string         `json:"gyro_lpf2_type_name,omitempty"`
	GyroHardwareLPF      uint8          `json:"gyro_hardware_lpf"`
	GyroHardwareLPFName  string         `json:"gyro_hardware_lpf_name,omitempty"`
	Gyro32KHzHardwareLPF uint8          `json:"gyro_32khz_hardware_lpf_deprecated"`
	GyroSoftNotchHz1     uint16         `json:"gyro_soft_notch_hz_1"`
	GyroSoftNotchCutoff1 uint16         `json:"gyro_soft_notch_cutoff_1"`
	GyroSoftNotchHz2     uint16         `json:"gyro_soft_notch_hz_2"`
	GyroSoftNotchCutoff2 uint16         `json:"gyro_soft_notch_cutoff_2"`
	DtermLPF1StaticHz    uint16         `json:"dterm_lpf1_static_hz"`
	DtermLPF2StaticHz    uint16         `json:"dterm_lpf2_static_hz"`
	DtermLPF1Type        uint8          `json:"dterm_lpf1_type"`
	DtermLPF1TypeName    string         `json:"dterm_lpf1_type_name,omitempty"`
	DtermLPF2Type        uint8          `json:"dterm_lpf2_type"`
	DtermLPF2TypeName    string         `json:"dterm_lpf2_type_name,omitempty"`
	DtermNotchHz         uint16         `json:"dterm_notch_hz"`
	DtermNotchCutoff     uint16         `json:"dterm_notch_cutoff"`
	YawLowpassHz         uint16         `json:"yaw_lowpass_hz"`
	DynamicLowpass       DynamicLowpass `json:"dynamic_lowpass"`
	DynamicNotch         DynamicNotch   `json:"dynamic_notch"`
	RPMFilter            RPMFilter      `json:"rpm_filter"`
	TrailingBytesIgnored int            `json:"trailing_bytes_ignored,omitempty"`
}

type FilterConfigSetResult struct {
	Config       FilterConfig `json:"filter_config"`
	MSPCode      uint16       `json:"msp_code"`
	MSPName      string       `json:"msp_name"`
	Acknowledged bool         `json:"acknowledged"`
	SaveRequired bool         `json:"save_required"`
}

type DynamicLowpass struct {
	GyroMinHz  uint16 `json:"gyro_min_hz"`
	GyroMaxHz  uint16 `json:"gyro_max_hz"`
	DtermMinHz uint16 `json:"dterm_min_hz"`
	DtermMaxHz uint16 `json:"dterm_max_hz"`
	DtermExpo  uint8  `json:"dterm_expo"`
}

type DynamicNotch struct {
	RangeDeprecated        uint8  `json:"range_deprecated"`
	WidthPercentDeprecated uint8  `json:"width_percent_deprecated"`
	Q                      uint16 `json:"q"`
	MinHz                  uint16 `json:"min_hz"`
	MaxHz                  uint16 `json:"max_hz"`
	Count                  uint8  `json:"count"`
}

type RPMFilter struct {
	Harmonics   uint8  `json:"harmonics"`
	MinHz       uint8  `json:"min_hz"`
	FadeRangeHz uint16 `json:"fade_range_hz"`
	Q           uint16 `json:"q"`
	Weights     []int  `json:"weights,omitempty"`
}

func ReadFilterStatus(ctx context.Context, client *connection.Client) (*FilterStatus, []string, error) {
	status := &FilterStatus{Sources: map[string]string{}}
	warnings := []string{}

	if advanced, err := readAdvancedConfig(ctx, client); err == nil {
		status.AdvancedConfig = advanced
		status.Sources["advanced_config"] = "MSP_ADVANCED_CONFIG"
	} else {
		warnings = append(warnings, err.Error())
	}
	filterConfig, err := readFilterConfig(ctx, client)
	if err != nil {
		return nil, warnings, err
	}
	status.FilterConfig = filterConfig
	status.Sources["filter_config"] = "MSP_FILTER_CONFIG"
	return status, warnings, nil
}

func SetAdvancedConfig(ctx context.Context, client *connection.Client, config AdvancedConfig) (*AdvancedConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetAdvancedConfig, EncodeAdvancedConfig(config)); err != nil {
		return nil, fmt.Errorf("advanced config request failed: %w", err)
	}
	return &AdvancedConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetAdvancedConfig,
		MSPName:      "MSP_SET_ADVANCED_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetFilterConfig(ctx context.Context, client *connection.Client, config FilterConfig) (*FilterConfigSetResult, error) {
	if err := ValidateFilterConfig(config); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetFilterConfig, EncodeFilterConfig(config)); err != nil {
		return nil, fmt.Errorf("filter config request failed: %w", err)
	}
	return &FilterConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetFilterConfig,
		MSPName:      "MSP_SET_FILTER_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func ValidateFilterConfig(config FilterConfig) error {
	if config.DynamicNotch.Count > 5 {
		return fmt.Errorf("dynamic_notch.count must be 0-5")
	}
	if config.RPMFilter.FadeRangeHz > 1000 {
		return fmt.Errorf("rpm_filter.fade_range_hz must be 0-1000")
	}
	if config.RPMFilter.Q != 0 && (config.RPMFilter.Q < 250 || config.RPMFilter.Q > 3000) {
		return fmt.Errorf("rpm_filter.q must be 250-3000 or 0")
	}
	if len(config.RPMFilter.Weights) != 0 && len(config.RPMFilter.Weights) != 3 {
		return fmt.Errorf("rpm_filter.weights must contain exactly 3 entries when provided")
	}
	for i, weight := range config.RPMFilter.Weights {
		if weight < 0 || weight > 100 {
			return fmt.Errorf("rpm_filter.weights[%d] must be 0-100", i)
		}
	}
	return nil
}

func EncodeAdvancedConfig(config AdvancedConfig) []byte {
	payload := []byte{
		config.GyroSyncDenom,
		config.PIDProcessDenom,
		config.UseUnsyncedPWM,
		config.MotorProtocol,
	}
	payload = appendU16Payload(payload, config.MotorPWMRate)
	payload = appendU16Payload(payload, config.MotorIdle)
	payload = append(payload,
		config.GyroUse32KHzDeprecated,
		config.MotorPWMInversion,
		config.GyroToUseDeprecated,
		config.GyroHighFSR,
		config.GyroMovementCalibThreshold,
	)
	payload = appendU16Payload(payload, config.GyroCalibDuration)
	payload = appendU16Payload(payload, config.GyroOffsetYaw)
	payload = append(payload, config.GyroCheckOverflow, config.DebugMode, config.DebugModeCount)
	return payload
}

func EncodeFilterConfig(config FilterConfig) []byte {
	payload := []byte{config.LegacyGyroLowpassHz}
	payload = appendU16Payload(payload, config.DtermLPF1StaticHz)
	payload = appendU16Payload(payload, config.YawLowpassHz)
	payload = appendU16Payload(payload, config.GyroSoftNotchHz1)
	payload = appendU16Payload(payload, config.GyroSoftNotchCutoff1)
	payload = appendU16Payload(payload, config.DtermNotchHz)
	payload = appendU16Payload(payload, config.DtermNotchCutoff)
	payload = appendU16Payload(payload, config.GyroSoftNotchHz2)
	payload = appendU16Payload(payload, config.GyroSoftNotchCutoff2)
	payload = append(payload, config.DtermLPF1Type, config.GyroHardwareLPF, config.Gyro32KHzHardwareLPF)
	payload = appendU16Payload(payload, config.GyroLPF1StaticHz)
	payload = appendU16Payload(payload, config.GyroLPF2StaticHz)
	payload = append(payload, config.GyroLPF1Type, config.GyroLPF2Type)
	payload = appendU16Payload(payload, config.DtermLPF2StaticHz)
	payload = append(payload, config.DtermLPF2Type)
	payload = appendU16Payload(payload, config.DynamicLowpass.GyroMinHz)
	payload = appendU16Payload(payload, config.DynamicLowpass.GyroMaxHz)
	payload = appendU16Payload(payload, config.DynamicLowpass.DtermMinHz)
	payload = appendU16Payload(payload, config.DynamicLowpass.DtermMaxHz)
	payload = append(payload, config.DynamicNotch.RangeDeprecated, config.DynamicNotch.WidthPercentDeprecated)
	payload = appendU16Payload(payload, config.DynamicNotch.Q)
	payload = appendU16Payload(payload, config.DynamicNotch.MinHz)
	payload = append(payload, config.RPMFilter.Harmonics, config.RPMFilter.MinHz)
	payload = appendU16Payload(payload, config.DynamicNotch.MaxHz)
	payload = append(payload, config.DynamicLowpass.DtermExpo, config.DynamicNotch.Count)
	payload = appendU16Payload(payload, config.RPMFilter.FadeRangeHz)
	payload = appendU16Payload(payload, config.RPMFilter.Q)
	weights := config.RPMFilter.Weights
	for i := 0; i < 3; i++ {
		weight := uint8(0)
		if i < len(weights) {
			weight = uint8(weights[i])
		}
		payload = append(payload, weight)
	}
	return payload
}

func DecodeAdvancedConfig(payload []byte) (*AdvancedConfig, error) {
	if len(payload) < advancedConfigModernLength {
		return nil, fmt.Errorf("payload length %d is shorter than modern MSP_ADVANCED_CONFIG size %d", len(payload), advancedConfigModernLength)
	}
	r := msp.NewPayloadReader(payload)
	config := &AdvancedConfig{}
	var err error
	if config.GyroSyncDenom, err = r.U8(); err != nil {
		return nil, err
	}
	if config.PIDProcessDenom, err = r.U8(); err != nil {
		return nil, err
	}
	if config.UseUnsyncedPWM, err = r.U8(); err != nil {
		return nil, err
	}
	if config.MotorProtocol, err = r.U8(); err != nil {
		return nil, err
	}
	config.MotorProtocolName = indexedName(motorProtocolNames, config.MotorProtocol)
	if config.MotorPWMRate, err = r.U16(); err != nil {
		return nil, err
	}
	if config.MotorIdle, err = r.U16(); err != nil {
		return nil, err
	}
	config.MotorIdlePercent = float64(config.MotorIdle) / 100
	if config.GyroUse32KHzDeprecated, err = r.U8(); err != nil {
		return nil, err
	}
	if config.MotorPWMInversion, err = r.U8(); err != nil {
		return nil, err
	}
	if config.GyroToUseDeprecated, err = r.U8(); err != nil {
		return nil, err
	}
	if config.GyroHighFSR, err = r.U8(); err != nil {
		return nil, err
	}
	if config.GyroMovementCalibThreshold, err = r.U8(); err != nil {
		return nil, err
	}
	if config.GyroCalibDuration, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroOffsetYaw, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroCheckOverflow, err = r.U8(); err != nil {
		return nil, err
	}
	config.GyroCheckOverflowName = indexedName(gyroOverflowCheckNames, config.GyroCheckOverflow)
	if config.DebugMode, err = r.U8(); err != nil {
		return nil, err
	}
	if config.DebugModeCount, err = r.U8(); err != nil {
		return nil, err
	}
	config.TrailingBytesIgnored = r.Remaining()
	return config, nil
}

func DecodeFilterConfig(payload []byte) (*FilterConfig, error) {
	if len(payload) < filterConfigModernLength {
		return nil, fmt.Errorf("payload length %d is shorter than modern MSP_FILTER_CONFIG size %d", len(payload), filterConfigModernLength)
	}
	r := msp.NewPayloadReader(payload)
	config := &FilterConfig{}
	var err error
	if config.LegacyGyroLowpassHz, err = r.U8(); err != nil {
		return nil, err
	}
	if config.DtermLPF1StaticHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.YawLowpassHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroSoftNotchHz1, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroSoftNotchCutoff1, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DtermNotchHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DtermNotchCutoff, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroSoftNotchHz2, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroSoftNotchCutoff2, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DtermLPF1Type, err = r.U8(); err != nil {
		return nil, err
	}
	config.DtermLPF1TypeName = indexedName(lowpassTypeNames, config.DtermLPF1Type)
	if config.GyroHardwareLPF, err = r.U8(); err != nil {
		return nil, err
	}
	config.GyroHardwareLPFName = indexedName(gyroHardwareLPFNames, config.GyroHardwareLPF)
	if config.Gyro32KHzHardwareLPF, err = r.U8(); err != nil {
		return nil, err
	}
	if config.GyroLPF1StaticHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroLPF2StaticHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.GyroLPF1Type, err = r.U8(); err != nil {
		return nil, err
	}
	config.GyroLPF1TypeName = indexedName(lowpassTypeNames, config.GyroLPF1Type)
	if config.GyroLPF2Type, err = r.U8(); err != nil {
		return nil, err
	}
	config.GyroLPF2TypeName = indexedName(lowpassTypeNames, config.GyroLPF2Type)
	if config.DtermLPF2StaticHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DtermLPF2Type, err = r.U8(); err != nil {
		return nil, err
	}
	config.DtermLPF2TypeName = indexedName(lowpassTypeNames, config.DtermLPF2Type)
	if config.DynamicLowpass.GyroMinHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DynamicLowpass.GyroMaxHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DynamicLowpass.DtermMinHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DynamicLowpass.DtermMaxHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DynamicNotch.RangeDeprecated, err = r.U8(); err != nil {
		return nil, err
	}
	if config.DynamicNotch.WidthPercentDeprecated, err = r.U8(); err != nil {
		return nil, err
	}
	if config.DynamicNotch.Q, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DynamicNotch.MinHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.RPMFilter.Harmonics, err = r.U8(); err != nil {
		return nil, err
	}
	if config.RPMFilter.MinHz, err = r.U8(); err != nil {
		return nil, err
	}
	if config.DynamicNotch.MaxHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.DynamicLowpass.DtermExpo, err = r.U8(); err != nil {
		return nil, err
	}
	if config.DynamicNotch.Count, err = r.U8(); err != nil {
		return nil, err
	}
	if config.RPMFilter.FadeRangeHz, err = r.U16(); err != nil {
		return nil, err
	}
	if config.RPMFilter.Q, err = r.U16(); err != nil {
		return nil, err
	}
	weights, err := r.Bytes(3)
	if err != nil {
		return nil, err
	}
	config.RPMFilter.Weights = make([]int, 0, len(weights))
	for _, weight := range weights {
		config.RPMFilter.Weights = append(config.RPMFilter.Weights, int(weight))
	}
	config.TrailingBytesIgnored = r.Remaining()
	return config, nil
}

func readAdvancedConfig(ctx context.Context, client *connection.Client) (*AdvancedConfig, error) {
	frame, err := client.Request(ctx, msp.MSPAdvancedConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("advanced config unavailable: %w", err)
	}
	config, err := DecodeAdvancedConfig(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("advanced config decode failed: %w", err)
	}
	return config, nil
}

func readFilterConfig(ctx context.Context, client *connection.Client) (*FilterConfig, error) {
	frame, err := client.Request(ctx, msp.MSPFilterConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("filter config unavailable: %w", err)
	}
	config, err := DecodeFilterConfig(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("filter config decode failed: %w", err)
	}
	return config, nil
}
