package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

var sensorOrder = []string{"gyro", "accelerometer", "barometer", "magnetometer", "rangefinder", "opticalflow"}

type SensorStatus struct {
	Config        []SensorHardware `json:"config,omitempty"`
	Active        []SensorHardware `json:"active,omitempty"`
	ActiveGyros   *ActiveGyros     `json:"active_gyros,omitempty"`
	IMU           *RawIMU          `json:"imu,omitempty"`
	Alignment     *SensorAlignment `json:"alignment,omitempty"`
	Compass       *CompassConfig   `json:"compass,omitempty"`
	ActiveSensors *uint16          `json:"active_sensors,omitempty"`
	ActiveNames   []string         `json:"active_names,omitempty"`
}

type SensorHardware struct {
	Name       string `json:"name"`
	HardwareID uint8  `json:"hardware_id"`
	Available  bool   `json:"available"`
}

type SensorHardwareConfig struct {
	Accelerometer uint8 `json:"accelerometer"`
	Barometer     uint8 `json:"barometer"`
	Magnetometer  uint8 `json:"magnetometer"`
	Rangefinder   uint8 `json:"rangefinder"`
}

type SensorHardwareConfigSetResult struct {
	Config       SensorHardwareConfig `json:"config"`
	Hardware     []SensorHardware     `json:"hardware"`
	MSPCode      uint16               `json:"msp_code"`
	MSPName      string               `json:"msp_name"`
	Acknowledged bool                 `json:"acknowledged"`
	SaveRequired bool                 `json:"save_required"`
}

type ActiveGyros struct {
	Source   string           `json:"source"`
	Count    uint8            `json:"count"`
	Hardware []SensorHardware `json:"hardware"`
}

type RawIMU struct {
	AccelerometerRaw []int16   `json:"accelerometer_raw"`
	GyroscopeRaw     []int16   `json:"gyroscope_raw"`
	MagnetometerRaw  []int16   `json:"magnetometer_raw"`
	AccelerometerG   []float64 `json:"accelerometer_g"`
	GyroscopeDPS     []float64 `json:"gyroscope_dps"`
}

type SensorAlignment struct {
	GyroAlignment      uint8     `json:"gyro_alignment"`
	AccelerometerAlign uint8     `json:"accelerometer_alignment"`
	MagnetometerAlign  uint8     `json:"magnetometer_alignment"`
	GyroDetectionFlags uint8     `json:"gyro_detection_flags"`
	GyroEnabledMask    *uint8    `json:"gyro_enabled_mask,omitempty"`
	MagCustomAlignment *Axis3i16 `json:"mag_custom_alignment,omitempty"`
}

type Axis3i16 struct {
	Roll  int16 `json:"roll"`
	Pitch int16 `json:"pitch"`
	Yaw   int16 `json:"yaw"`
}

type CompassConfig struct {
	DeclinationDeciDegrees int16   `json:"declination_deci_degrees"`
	DeclinationDegrees     float64 `json:"declination_degrees"`
}

type CompassConfigSetResult struct {
	Config       CompassConfig `json:"config"`
	MSPCode      uint16        `json:"msp_code"`
	MSPName      string        `json:"msp_name"`
	Acknowledged bool          `json:"acknowledged"`
	SaveRequired bool          `json:"save_required"`
}

type SensorCalibrationKind string

const (
	SensorCalibrationAccelerometer SensorCalibrationKind = "accelerometer"
	SensorCalibrationMagnetometer  SensorCalibrationKind = "magnetometer"
)

type SensorCalibrationResult struct {
	Kind         SensorCalibrationKind `json:"kind"`
	MSPCode      uint16                `json:"msp_code"`
	MSPName      string                `json:"msp_name"`
	Acknowledged bool                  `json:"acknowledged"`
}

func ReadSensorStatus(ctx context.Context, client *connection.Client) (*SensorStatus, []string, error) {
	status := &SensorStatus{}
	warnings := []string{}
	if activeSensors, activeNames, err := readStatusSensors(ctx, client); err == nil {
		status.ActiveSensors = &activeSensors
		status.ActiveNames = activeNames
	} else {
		warnings = append(warnings, err.Error())
	}
	frame, err := client.Request(ctx, msp.MSPSensorConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("sensor config unavailable: %w", err)
	}
	config, err := DecodeSensorHardware(frame.Payload, sensorOrder[1:])
	if err != nil {
		return nil, nil, fmt.Errorf("sensor config decode failed: %w", err)
	}
	status.Config = config
	if active, err := readActiveSensorHardware(ctx, client); err == nil {
		status.Active = active
	} else {
		warnings = append(warnings, err.Error())
	}
	if activeGyros, err := readActiveGyros(ctx, client); err == nil {
		status.ActiveGyros = activeGyros
	} else {
		warnings = append(warnings, err.Error())
	}
	if imu, err := readRawIMU(ctx, client); err == nil {
		status.IMU = imu
	} else {
		warnings = append(warnings, err.Error())
	}
	if alignment, err := readSensorAlignment(ctx, client); err == nil {
		status.Alignment = alignment
	} else {
		warnings = append(warnings, err.Error())
	}
	if compass, err := readCompassConfig(ctx, client); err == nil {
		status.Compass = compass
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func CalibrateSensor(ctx context.Context, client *connection.Client, kind SensorCalibrationKind) (*SensorCalibrationResult, error) {
	code, name, err := sensorCalibrationCommand(kind)
	if err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, code, nil); err != nil {
		return nil, fmt.Errorf("%s calibration request failed: %w", kind, err)
	}
	return &SensorCalibrationResult{
		Kind:         kind,
		MSPCode:      code,
		MSPName:      name,
		Acknowledged: true,
	}, nil
}

func SetCompassConfig(ctx context.Context, client *connection.Client, declinationDeciDegrees int16) (*CompassConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetCompassConfig, EncodeCompassConfig(declinationDeciDegrees)); err != nil {
		return nil, fmt.Errorf("compass config request failed: %w", err)
	}
	return &CompassConfigSetResult{
		Config:       compassConfigFromDeclination(declinationDeciDegrees),
		MSPCode:      msp.MSPSetCompassConfig,
		MSPName:      "MSP_SET_COMPASS_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetSensorHardwareConfig(ctx context.Context, client *connection.Client, config SensorHardwareConfig) (*SensorHardwareConfigSetResult, error) {
	payload := EncodeSensorHardwareConfig(config)
	if _, err := client.Request(ctx, msp.MSPSetSensorConfig, payload); err != nil {
		return nil, fmt.Errorf("sensor hardware config request failed: %w", err)
	}
	hardware, err := DecodeSensorHardware(payload, sensorOrder[1:])
	if err != nil {
		return nil, err
	}
	return &SensorHardwareConfigSetResult{
		Config:       config,
		Hardware:     hardware,
		MSPCode:      msp.MSPSetSensorConfig,
		MSPName:      "MSP_SET_SENSOR_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeSensorHardwareConfig(config SensorHardwareConfig) []byte {
	return []byte{config.Accelerometer, config.Barometer, config.Magnetometer, config.Rangefinder}
}

func EncodeCompassConfig(declinationDeciDegrees int16) []byte {
	v := uint16(declinationDeciDegrees)
	return []byte{byte(v), byte(v >> 8)}
}

func sensorCalibrationCommand(kind SensorCalibrationKind) (uint16, string, error) {
	switch kind {
	case SensorCalibrationAccelerometer:
		return msp.MSPAccCalibration, "MSP_ACC_CALIBRATION", nil
	case SensorCalibrationMagnetometer:
		return msp.MSPMagCalibration, "MSP_MAG_CALIBRATION", nil
	default:
		return 0, "", fmt.Errorf("unsupported sensor calibration kind %q", kind)
	}
}

func DecodeSensorHardware(payload []byte, names []string) ([]SensorHardware, error) {
	out := make([]SensorHardware, 0, len(payload))
	for i, hardwareID := range payload {
		name := fmt.Sprintf("sensor_%d", i)
		if i < len(names) {
			name = names[i]
		}
		out = append(out, SensorHardware{Name: name, HardwareID: hardwareID, Available: hardwareID != 0xff})
	}
	return out, nil
}

func DecodeActiveGyros(payload []byte) (*ActiveGyros, error) {
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "gyro count")
	}
	if r.Remaining() < int(count) {
		return nil, fmt.Errorf("payload length %d is shorter than gyro count %d", len(payload), count)
	}
	hardware := make([]SensorHardware, 0, count)
	for i := 0; i < int(count); i++ {
		id, err := r.U8()
		if err != nil {
			return nil, msp.RequireNoShort(err, fmt.Sprintf("gyro hardware %d", i))
		}
		hardware = append(hardware, SensorHardware{
			Name:       gyroHardwareName(id),
			HardwareID: id,
			Available:  id != 0xff && id != 0,
		})
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP2_GYRO_SENSOR_ACTIVE returned %d trailing byte(s)", r.Remaining())
	}
	return &ActiveGyros{
		Source:   "MSP2_GYRO_SENSOR_ACTIVE",
		Count:    count,
		Hardware: hardware,
	}, nil
}

func DecodeRawIMU(payload []byte) (*RawIMU, error) {
	r := msp.NewPayloadReader(payload)
	acc, err := readS16Triple(r)
	if err != nil {
		return nil, err
	}
	gyro, err := readS16Triple(r)
	if err != nil {
		return nil, err
	}
	mag, err := readS16Triple(r)
	if err != nil {
		return nil, err
	}
	return &RawIMU{
		AccelerometerRaw: acc,
		GyroscopeRaw:     gyro,
		MagnetometerRaw:  mag,
		AccelerometerG:   scaleInt16(acc, 1.0/2048.0),
		GyroscopeDPS:     scaleInt16(gyro, 4.0/16.4),
	}, nil
}

func DecodeSensorAlignment(payload []byte) (*SensorAlignment, error) {
	r := msp.NewPayloadReader(payload)
	gyro, err := r.U8()
	if err != nil {
		return nil, err
	}
	acc, err := r.U8()
	if err != nil {
		return nil, err
	}
	mag, err := r.U8()
	if err != nil {
		return nil, err
	}
	flags, err := r.U8()
	if err != nil {
		return nil, err
	}
	alignment := &SensorAlignment{
		GyroAlignment:      gyro,
		AccelerometerAlign: acc,
		MagnetometerAlign:  mag,
		GyroDetectionFlags: flags,
	}
	if r.Remaining() >= 1 {
		mask, err := r.U8()
		if err != nil {
			return nil, err
		}
		alignment.GyroEnabledMask = &mask
	}
	if r.Remaining() >= 6 {
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
		alignment.MagCustomAlignment = &Axis3i16{Roll: roll, Pitch: pitch, Yaw: yaw}
	}
	return alignment, nil
}

func DecodeCompassConfig(payload []byte) (*CompassConfig, error) {
	r := msp.NewPayloadReader(payload)
	declination, err := r.S16()
	if err != nil {
		return nil, err
	}
	config := compassConfigFromDeclination(declination)
	return &config, nil
}

func compassConfigFromDeclination(declination int16) CompassConfig {
	return CompassConfig{
		DeclinationDeciDegrees: declination,
		DeclinationDegrees:     float64(declination) / 10,
	}
}

func readActiveSensorHardware(ctx context.Context, client *connection.Client) ([]SensorHardware, error) {
	frame, err := client.Request(ctx, msp.MSP2SensorConfigActive, nil)
	if err != nil {
		return nil, fmt.Errorf("active sensor config unavailable: %w", err)
	}
	return DecodeSensorHardware(frame.Payload, sensorOrder)
}

func readActiveGyros(ctx context.Context, client *connection.Client) (*ActiveGyros, error) {
	frame, err := client.Request(ctx, msp.MSP2GyroSensorActive, nil)
	if err != nil {
		return nil, fmt.Errorf("active gyro sensors unavailable: %w", err)
	}
	return DecodeActiveGyros(frame.Payload)
}

func readRawIMU(ctx context.Context, client *connection.Client) (*RawIMU, error) {
	frame, err := client.Request(ctx, msp.MSPRawImu, nil)
	if err != nil {
		return nil, fmt.Errorf("raw imu unavailable: %w", err)
	}
	return DecodeRawIMU(frame.Payload)
}

func readSensorAlignment(ctx context.Context, client *connection.Client) (*SensorAlignment, error) {
	frame, err := client.Request(ctx, msp.MSPSensorAlignment, nil)
	if err != nil {
		return nil, fmt.Errorf("sensor alignment unavailable: %w", err)
	}
	return DecodeSensorAlignment(frame.Payload)
}

func readCompassConfig(ctx context.Context, client *connection.Client) (*CompassConfig, error) {
	frame, err := client.Request(ctx, msp.MSPCompassConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("compass config unavailable: %w", err)
	}
	return DecodeCompassConfig(frame.Payload)
}

func readStatusSensors(ctx context.Context, client *connection.Client) (uint16, []string, error) {
	status, err := readStatus(ctx, client)
	if err != nil {
		return 0, nil, err
	}
	return status.ActiveSensors, activeSensorNames(status.ActiveSensors), nil
}

func activeSensorNames(mask uint16) []string {
	names := []string{"accelerometer", "barometer", "magnetometer", "gps", "rangefinder", "gyro", "opticalflow"}
	out := []string{}
	for i, name := range names {
		if mask&(1<<i) != 0 {
			out = append(out, name)
		}
	}
	return out
}

func gyroHardwareName(id uint8) string {
	if int(id) < len(gyroHardwareNames) {
		return gyroHardwareNames[id]
	}
	return ""
}

var gyroHardwareNames = []string{
	"NONE",
	"AUTO",
	"MPU6050",
	"L3GD20",
	"MPU6000",
	"MPU6500",
	"MPU9250",
	"ICM20601",
	"ICM20602",
	"ICM20608G",
	"ICM20649",
	"ICM20689",
	"ICM42605",
	"ICM42688P",
	"BMI160",
	"BMI270",
	"LSM6DSO",
	"LSM6DSV16X",
	"IIM42653",
	"ICM45605",
	"ICM45686",
	"ICM40609D",
	"IIM42652",
	"LSM6DSK320X",
	"ICM42622P",
	"ICM42686P",
	"VIRTUAL",
}

func readS16Triple(r *msp.PayloadReader) ([]int16, error) {
	out := make([]int16, 3)
	for i := range out {
		v, err := r.S16()
		if err != nil {
			return nil, err
		}
		out[i] = v
	}
	return out, nil
}

func scaleInt16(values []int16, factor float64) []float64 {
	out := make([]float64, len(values))
	for i, value := range values {
		out[i] = float64(value) * factor
	}
	return out
}
