package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type Telemetry struct {
	Sources            map[string]string   `json:"sources"`
	Status             *Status             `json:"status,omitempty"`
	Attitude           *Attitude           `json:"attitude,omitempty"`
	AttitudeQuaternion *AttitudeQuaternion `json:"attitude_quaternion,omitempty"`
	Battery            *Battery            `json:"battery,omitempty"`
	RC                 []uint16            `json:"rc,omitempty"`
}

type Status struct {
	Source              string   `json:"source"`
	CycleTimeUS         uint16   `json:"cycle_time_us"`
	I2CErrors           uint16   `json:"i2c_errors"`
	ActiveSensors       uint16   `json:"active_sensors"`
	ModeFlags           uint32   `json:"mode_flags"`
	ModeFlagsBytes      []int    `json:"mode_flags_bytes,omitempty"`
	ModeFlagByteCount   *uint8   `json:"mode_flag_byte_count,omitempty"`
	Profile             uint8    `json:"profile"`
	CPULoad             *uint16  `json:"cpu_load,omitempty"`
	ProfileCount        *uint8   `json:"profile_count,omitempty"`
	RateProfile         *uint8   `json:"rate_profile,omitempty"`
	RateProfileCount    *uint8   `json:"rate_profile_count,omitempty"`
	BatteryProfileCount *uint8   `json:"battery_profile_count,omitempty"`
	BatteryProfile      *uint8   `json:"battery_profile,omitempty"`
	ArmingDisableCount  *uint8   `json:"arming_disable_count,omitempty"`
	ArmingDisableFlags  *uint32  `json:"arming_disable_flags,omitempty"`
	ConfigStateFlag     *uint8   `json:"config_state_flag,omitempty"`
	CPUTemperatureC     *float64 `json:"cpu_temperature_c,omitempty"`
}

type Attitude struct {
	RollDegrees  float64 `json:"roll_degrees"`
	PitchDegrees float64 `json:"pitch_degrees"`
	YawDegrees   int16   `json:"yaw_degrees"`
}

type AttitudeQuaternion struct {
	W float64 `json:"w"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type Battery struct {
	CellCount      uint8   `json:"cell_count"`
	CapacityMAh    uint16  `json:"capacity_mah"`
	VoltageLegacyV float64 `json:"voltage_legacy_v"`
	DrawnMAh       uint16  `json:"drawn_mah"`
	AmperageA      float64 `json:"amperage_a"`
	State          uint8   `json:"state"`
	VoltageV       float64 `json:"voltage_v"`
}

func ReadTelemetry(ctx context.Context, client *connection.Client) (Telemetry, []string) {
	var out Telemetry
	out.Sources = map[string]string{}
	var warnings []string
	if status, err := readStatus(ctx, client); err == nil {
		out.Status = status
		out.Sources["status"] = status.Source
	} else {
		out.Sources["status"] = "MSP_STATUS"
		warnings = append(warnings, err.Error())
	}
	if attitude, err := readAttitude(ctx, client); err == nil {
		out.Attitude = attitude
		out.Sources["attitude"] = "MSP_ATTITUDE"
	} else {
		out.Sources["attitude"] = ""
		warnings = append(warnings, err.Error())
	}
	if quaternion, err := readAttitudeQuaternion(ctx, client); err == nil {
		out.AttitudeQuaternion = quaternion
		out.Sources["attitude_quaternion"] = "MSP_ATTITUDE_QUATERNION"
	} else {
		out.Sources["attitude_quaternion"] = ""
		warnings = append(warnings, err.Error())
	}
	if battery, err := readBattery(ctx, client); err == nil {
		out.Battery = battery
		out.Sources["battery"] = "MSP_BATTERY_STATE"
	} else {
		out.Sources["battery"] = ""
		warnings = append(warnings, err.Error())
	}
	if rc, err := readRC(ctx, client); err == nil {
		out.RC = rc
		out.Sources["rc"] = "MSP_RC"
	} else {
		out.Sources["rc"] = ""
		warnings = append(warnings, err.Error())
	}
	return out, warnings
}

func readStatus(ctx context.Context, client *connection.Client) (*Status, error) {
	frame, err := client.Request(ctx, msp.MSPStatusEx, nil)
	if err == nil {
		return decodeStatus(frame.Payload, true)
	}
	frame, fallbackErr := client.Request(ctx, msp.MSPStatus, nil)
	if fallbackErr != nil {
		return nil, fmt.Errorf("status unavailable: %v; fallback: %v", err, fallbackErr)
	}
	return decodeStatus(frame.Payload, false)
}

func decodeStatus(payload []byte, extended bool) (*Status, error) {
	r := msp.NewPayloadReader(payload)
	cycle, err := r.U16()
	if err != nil {
		return nil, err
	}
	i2c, err := r.U16()
	if err != nil {
		return nil, err
	}
	sensors, err := r.U16()
	if err != nil {
		return nil, err
	}
	mode, err := r.U32()
	if err != nil {
		return nil, err
	}
	profile, err := r.U8()
	if err != nil {
		return nil, err
	}
	status := &Status{
		Source:        "MSP_STATUS",
		CycleTimeUS:   cycle,
		I2CErrors:     i2c,
		ActiveSensors: sensors,
		ModeFlags:     mode,
		ModeFlagsBytes: []int{
			int(byte(mode)),
			int(byte(mode >> 8)),
			int(byte(mode >> 16)),
			int(byte(mode >> 24)),
		},
		Profile: profile,
	}
	if !extended {
		return status, nil
	}
	status.Source = "MSP_STATUS_EX"
	if cpu, err := r.U16(); err == nil {
		status.CPULoad = &cpu
	}
	if count, err := r.U8(); err == nil {
		status.ProfileCount = &count
	}
	if rate, err := r.U8(); err == nil {
		status.RateProfile = &rate
	}
	if byteCount, err := r.U8(); err == nil && r.Remaining() >= int(byteCount) {
		status.ModeFlagByteCount = &byteCount
		if extra, err := r.Bytes(int(byteCount)); err == nil {
			for _, value := range extra {
				status.ModeFlagsBytes = append(status.ModeFlagsBytes, int(value))
			}
		}
	}
	if count, err := r.U8(); err == nil {
		status.ArmingDisableCount = &count
	}
	if flags, err := r.U32(); err == nil {
		status.ArmingDisableFlags = &flags
	}
	if config, err := r.U8(); err == nil {
		status.ConfigStateFlag = &config
	}
	if temp, err := r.U16(); err == nil {
		v := float64(temp) / 10
		status.CPUTemperatureC = &v
	}
	if count, err := r.U8(); err == nil {
		status.RateProfileCount = &count
	}
	if count, err := r.U8(); err == nil {
		status.BatteryProfileCount = &count
	}
	if battery, err := r.U8(); err == nil {
		status.BatteryProfile = &battery
	}
	return status, nil
}

func DecodeStatus(payload []byte) (*Status, error) {
	return decodeStatus(payload, false)
}

func DecodeStatusEx(payload []byte) (*Status, error) {
	return decodeStatus(payload, true)
}

func readAttitude(ctx context.Context, client *connection.Client) (*Attitude, error) {
	frame, err := client.Request(ctx, msp.MSPAttitude, nil)
	if err != nil {
		return nil, fmt.Errorf("attitude unavailable: %w", err)
	}
	return decodeAttitude(frame.Payload)
}

func decodeAttitude(payload []byte) (*Attitude, error) {
	r := msp.NewPayloadReader(payload)
	roll, err := r.S16()
	if err != nil {
		return nil, err
	}
	pitch, err := r.S16()
	if err != nil {
		return nil, err
	}
	yaw, err := r.S16()
	if err != nil {
		return nil, err
	}
	return &Attitude{
		RollDegrees:  float64(roll) / 10,
		PitchDegrees: float64(pitch) / 10,
		YawDegrees:   yaw,
	}, nil
}

func DecodeAttitude(payload []byte) (*Attitude, error) {
	return decodeAttitude(payload)
}

func readAttitudeQuaternion(ctx context.Context, client *connection.Client) (*AttitudeQuaternion, error) {
	frame, err := client.Request(ctx, msp.MSPAttitudeQuaternion, nil)
	if err != nil {
		return nil, fmt.Errorf("attitude quaternion unavailable: %w", err)
	}
	return DecodeAttitudeQuaternion(frame.Payload)
}

func DecodeAttitudeQuaternion(payload []byte) (*AttitudeQuaternion, error) {
	r := msp.NewPayloadReader(payload)
	w, err := r.S16()
	if err != nil {
		return nil, err
	}
	x, err := r.S16()
	if err != nil {
		return nil, err
	}
	y, err := r.S16()
	if err != nil {
		return nil, err
	}
	z, err := r.S16()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_ATTITUDE_QUATERNION returned %d trailing byte(s)", r.Remaining())
	}
	const scale = 32767.0
	return &AttitudeQuaternion{
		W: float64(w) / scale,
		X: float64(x) / scale,
		Y: float64(y) / scale,
		Z: float64(z) / scale,
	}, nil
}

func readBattery(ctx context.Context, client *connection.Client) (*Battery, error) {
	frame, err := client.Request(ctx, msp.MSPBatteryState, nil)
	if err != nil {
		return nil, fmt.Errorf("battery unavailable: %w", err)
	}
	r := msp.NewPayloadReader(frame.Payload)
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
	return &Battery{
		CellCount:      cellCount,
		CapacityMAh:    capacity,
		VoltageLegacyV: float64(legacyVoltage) / 10,
		DrawnMAh:       drawn,
		AmperageA:      float64(amps) / 100,
		State:          state,
		VoltageV:       float64(voltage) / 100,
	}, nil
}

func readRC(ctx context.Context, client *connection.Client) ([]uint16, error) {
	frame, err := client.Request(ctx, msp.MSPRC, nil)
	if err != nil {
		return nil, fmt.Errorf("rc unavailable: %w", err)
	}
	r := msp.NewPayloadReader(frame.Payload)
	var values []uint16
	for r.Remaining() >= 2 {
		v, err := r.U16()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		return nil, errors.New("MSP_RC returned no channels")
	}
	return values, nil
}

func DecodeRC(payload []byte) ([]uint16, error) {
	r := msp.NewPayloadReader(payload)
	var values []uint16
	for r.Remaining() >= 2 {
		v, err := r.U16()
		if err != nil {
			return nil, err
		}
		values = append(values, v)
	}
	if len(values) == 0 {
		return nil, errors.New("MSP_RC returned no channels")
	}
	return values, nil
}
