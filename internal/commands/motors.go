package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type MotorStatus struct {
	Config      *MotorConfig     `json:"config,omitempty"`
	Outputs     []uint16         `json:"outputs,omitempty"`
	Telemetry   []MotorTelemetry `json:"telemetry,omitempty"`
	Config3D    *Motor3DConfig   `json:"config_3d,omitempty"`
	OutputOrder []uint8          `json:"output_order,omitempty"`
}

type MotorConfig struct {
	MinThrottleDeprecated uint16 `json:"min_throttle_deprecated"`
	MaxThrottle           uint16 `json:"max_throttle"`
	MinCommand            uint16 `json:"min_command"`
	MotorCount            *uint8 `json:"motor_count,omitempty"`
	MotorPoles            *uint8 `json:"motor_poles,omitempty"`
	UseDShotTelemetry     *bool  `json:"use_dshot_telemetry,omitempty"`
	UseESCSensor          *bool  `json:"use_esc_sensor,omitempty"`
}

type MotorTelemetry struct {
	Index             int     `json:"index"`
	RPM               uint32  `json:"rpm"`
	InvalidPercentRaw uint16  `json:"invalid_percent_raw"`
	InvalidPercent    float64 `json:"invalid_percent"`
	TemperatureC      uint8   `json:"temperature_c"`
	VoltageRaw        uint16  `json:"voltage_raw"`
	VoltageV          float64 `json:"voltage_v"`
	CurrentRaw        uint16  `json:"current_raw"`
	CurrentA          float64 `json:"current_a"`
	ConsumptionMAh    uint16  `json:"consumption_mah"`
}

type Motor3DConfig struct {
	DeadbandLow  uint16 `json:"deadband_low"`
	DeadbandHigh uint16 `json:"deadband_high"`
	Neutral      uint16 `json:"neutral"`
}

type ServoStatus struct {
	Outputs        []uint16             `json:"outputs,omitempty"`
	Configurations []ServoConfiguration `json:"configurations,omitempty"`
	MixRules       []ServoMixRule       `json:"mix_rules,omitempty"`
}

type ServoConfiguration struct {
	Index               int    `json:"index"`
	Min                 uint16 `json:"min"`
	Max                 uint16 `json:"max"`
	Middle              uint16 `json:"middle"`
	Rate                int8   `json:"rate"`
	ForwardFromChannel  uint8  `json:"forward_from_channel"`
	ReversedSourcesMask uint32 `json:"reversed_sources_mask"`
}

type ServoMixRule struct {
	Index         int   `json:"index"`
	TargetChannel uint8 `json:"target_channel"`
	InputSource   uint8 `json:"input_source"`
	Rate          int8  `json:"rate"`
	Speed         uint8 `json:"speed"`
	Min           uint8 `json:"min"`
	Max           uint8 `json:"max"`
	Box           uint8 `json:"box"`
	Active        bool  `json:"active"`
}

func ReadMotorStatus(ctx context.Context, client *connection.Client) (*MotorStatus, []string, error) {
	status := &MotorStatus{}
	warnings := []string{}
	frame, err := client.Request(ctx, msp.MSPMotorConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("motor config unavailable: %w", err)
	}
	config, err := DecodeMotorConfig(frame.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("motor config decode failed: %w", err)
	}
	status.Config = config
	if outputs, err := readMotorOutputs(ctx, client); err == nil {
		status.Outputs = outputs
	} else {
		warnings = append(warnings, err.Error())
	}
	if telemetry, err := readMotorTelemetry(ctx, client); err == nil {
		status.Telemetry = telemetry
	} else {
		warnings = append(warnings, err.Error())
	}
	if config3D, err := readMotor3DConfig(ctx, client); err == nil {
		status.Config3D = config3D
	} else {
		warnings = append(warnings, err.Error())
	}
	if order, err := readMotorOutputOrder(ctx, client); err == nil {
		status.OutputOrder = order
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func ReadServoStatus(ctx context.Context, client *connection.Client) (*ServoStatus, []string, error) {
	status := &ServoStatus{}
	warnings := []string{}
	frame, err := client.Request(ctx, msp.MSPServo, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("servo outputs unavailable: %w", err)
	}
	outputs, err := DecodeU16Array(frame.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("servo outputs decode failed: %w", err)
	}
	status.Outputs = outputs
	if configs, err := readServoConfigurations(ctx, client); err == nil {
		status.Configurations = configs
	} else {
		warnings = append(warnings, err.Error())
	}
	if rules, err := readServoMixRules(ctx, client); err == nil {
		status.MixRules = rules
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func DecodeMotorConfig(payload []byte) (*MotorConfig, error) {
	r := msp.NewPayloadReader(payload)
	minThrottle, err := r.U16()
	if err != nil {
		return nil, err
	}
	maxThrottle, err := r.U16()
	if err != nil {
		return nil, err
	}
	minCommand, err := r.U16()
	if err != nil {
		return nil, err
	}
	config := &MotorConfig{
		MinThrottleDeprecated: minThrottle,
		MaxThrottle:           maxThrottle,
		MinCommand:            minCommand,
	}
	if r.Remaining() >= 1 {
		v, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.MotorCount = &v
	}
	if r.Remaining() >= 1 {
		v, err := r.U8()
		if err != nil {
			return nil, err
		}
		config.MotorPoles = &v
	}
	if r.Remaining() >= 1 {
		raw, err := r.U8()
		if err != nil {
			return nil, err
		}
		v := raw != 0
		config.UseDShotTelemetry = &v
	}
	if r.Remaining() >= 1 {
		raw, err := r.U8()
		if err != nil {
			return nil, err
		}
		v := raw != 0
		config.UseESCSensor = &v
	}
	return config, nil
}

func DecodeU16Array(payload []byte) ([]uint16, error) {
	if len(payload)%2 != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of 2", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	values := make([]uint16, 0, len(payload)/2)
	for r.Remaining() > 0 {
		value, err := r.U16()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func DecodeMotorTelemetry(payload []byte) ([]MotorTelemetry, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, err
	}
	telemetry := make([]MotorTelemetry, 0, count)
	for i := 0; i < int(count); i++ {
		rpm, err := r.U32()
		if err != nil {
			return nil, err
		}
		invalid, err := r.U16()
		if err != nil {
			return nil, err
		}
		temp, err := r.U8()
		if err != nil {
			return nil, err
		}
		voltage, err := r.U16()
		if err != nil {
			return nil, err
		}
		current, err := r.U16()
		if err != nil {
			return nil, err
		}
		consumption, err := r.U16()
		if err != nil {
			return nil, err
		}
		telemetry = append(telemetry, MotorTelemetry{
			Index:             i,
			RPM:               rpm,
			InvalidPercentRaw: invalid,
			InvalidPercent:    float64(invalid) / 100,
			TemperatureC:      temp,
			VoltageRaw:        voltage,
			VoltageV:          float64(voltage) / 100,
			CurrentRaw:        current,
			CurrentA:          float64(current) / 100,
			ConsumptionMAh:    consumption,
		})
	}
	return telemetry, nil
}

func DecodeMotor3DConfig(payload []byte) (*Motor3DConfig, error) {
	r := msp.NewPayloadReader(payload)
	low, err := r.U16()
	if err != nil {
		return nil, err
	}
	high, err := r.U16()
	if err != nil {
		return nil, err
	}
	neutral, err := r.U16()
	if err != nil {
		return nil, err
	}
	return &Motor3DConfig{DeadbandLow: low, DeadbandHigh: high, Neutral: neutral}, nil
}

func DecodeMotorOutputOrder(payload []byte) ([]uint8, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, err
	}
	order, err := r.Bytes(int(count))
	if err != nil {
		return nil, err
	}
	return append([]uint8(nil), order...), nil
}

func DecodeServoConfigurations(payload []byte) ([]ServoConfiguration, error) {
	if len(payload)%12 != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of 12", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	configs := make([]ServoConfiguration, 0, len(payload)/12)
	for i := 0; r.Remaining() > 0; i++ {
		min, err := r.U16()
		if err != nil {
			return nil, err
		}
		max, err := r.U16()
		if err != nil {
			return nil, err
		}
		middle, err := r.U16()
		if err != nil {
			return nil, err
		}
		rateRaw, err := r.U8()
		if err != nil {
			return nil, err
		}
		forward, err := r.U8()
		if err != nil {
			return nil, err
		}
		reversed, err := r.U32()
		if err != nil {
			return nil, err
		}
		configs = append(configs, ServoConfiguration{
			Index:               i,
			Min:                 min,
			Max:                 max,
			Middle:              middle,
			Rate:                int8(rateRaw),
			ForwardFromChannel:  forward,
			ReversedSourcesMask: reversed,
		})
	}
	return configs, nil
}

func DecodeServoMixRules(payload []byte) ([]ServoMixRule, error) {
	if len(payload)%7 != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of 7", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	rules := make([]ServoMixRule, 0, len(payload)/7)
	for i := 0; r.Remaining() > 0; i++ {
		target, err := r.U8()
		if err != nil {
			return nil, err
		}
		source, err := r.U8()
		if err != nil {
			return nil, err
		}
		rate, err := r.U8()
		if err != nil {
			return nil, err
		}
		speed, err := r.U8()
		if err != nil {
			return nil, err
		}
		min, err := r.U8()
		if err != nil {
			return nil, err
		}
		max, err := r.U8()
		if err != nil {
			return nil, err
		}
		box, err := r.U8()
		if err != nil {
			return nil, err
		}
		rules = append(rules, ServoMixRule{
			Index:         i,
			TargetChannel: target,
			InputSource:   source,
			Rate:          int8(rate),
			Speed:         speed,
			Min:           min,
			Max:           max,
			Box:           box,
			Active:        target != 0 || source != 0 || rate != 0 || speed != 0 || min != 0 || max != 0 || box != 0,
		})
	}
	return rules, nil
}

func readMotorOutputs(ctx context.Context, client *connection.Client) ([]uint16, error) {
	frame, err := client.Request(ctx, msp.MSPMotor, nil)
	if err != nil {
		return nil, fmt.Errorf("motor outputs unavailable: %w", err)
	}
	return DecodeU16Array(frame.Payload)
}

func readMotorTelemetry(ctx context.Context, client *connection.Client) ([]MotorTelemetry, error) {
	frame, err := client.Request(ctx, msp.MSPMotorTelemetry, nil)
	if err != nil {
		return nil, fmt.Errorf("motor telemetry unavailable: %w", err)
	}
	return DecodeMotorTelemetry(frame.Payload)
}

func readMotor3DConfig(ctx context.Context, client *connection.Client) (*Motor3DConfig, error) {
	frame, err := client.Request(ctx, msp.MSPMotor3dConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("motor 3d config unavailable: %w", err)
	}
	return DecodeMotor3DConfig(frame.Payload)
}

func readMotorOutputOrder(ctx context.Context, client *connection.Client) ([]uint8, error) {
	frame, err := client.Request(ctx, msp.MSP2MotorOutputReordering, nil)
	if err != nil {
		return nil, fmt.Errorf("motor output order unavailable: %w", err)
	}
	return DecodeMotorOutputOrder(frame.Payload)
}

func readServoConfigurations(ctx context.Context, client *connection.Client) ([]ServoConfiguration, error) {
	frame, err := client.Request(ctx, msp.MSPServoConfigurations, nil)
	if err != nil {
		return nil, fmt.Errorf("servo configurations unavailable: %w", err)
	}
	return DecodeServoConfigurations(frame.Payload)
}

func readServoMixRules(ctx context.Context, client *connection.Client) ([]ServoMixRule, error) {
	frame, err := client.Request(ctx, msp.MSPServoMixRules, nil)
	if err != nil {
		return nil, fmt.Errorf("servo mix rules unavailable: %w", err)
	}
	return DecodeServoMixRules(frame.Payload)
}
