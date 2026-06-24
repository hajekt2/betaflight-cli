package cli

import (
	"context"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestSensorsStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	sensors := data["sensors"].(map[string]any)
	active := sensors["active"].([]any)
	if len(active) != 6 {
		t.Fatalf("active = %+v", active)
	}
	activeGyros := sensors["active_gyros"].(map[string]any)
	if activeGyros["source"] != "MSP2_GYRO_SENSOR_ACTIVE" || activeGyros["count"] != float64(2) {
		t.Fatalf("active gyros = %+v", activeGyros)
	}
	gyroHardware := activeGyros["hardware"].([]any)
	if len(gyroHardware) != 2 || gyroHardware[0].(map[string]any)["name"] != "ICM20689" || gyroHardware[1].(map[string]any)["hardware_id"] != float64(19) {
		t.Fatalf("gyro hardware = %+v", gyroHardware)
	}
	imu := sensors["imu"].(map[string]any)
	accRaw := imu["accelerometer_raw"].([]any)
	if accRaw[0] != float64(2048) || accRaw[1] != float64(-1024) {
		t.Fatalf("imu = %+v", imu)
	}
	alignment := sensors["alignment"].(map[string]any)
	if alignment["gyro_enabled_mask"] != float64(3) {
		t.Fatalf("alignment = %+v", alignment)
	}
	compass := sensors["compass"].(map[string]any)
	if compass["declination_degrees"] != 12.3 {
		t.Fatalf("compass = %+v", compass)
	}
}

func TestSensorsCalibrationRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"sensors", "calibrate-accelerometer"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after calibration confirmation failure")
	}
}

func TestSensorsCalibrationWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "calibrate-magnetometer", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	calibration := data["sensor_calibration"].(map[string]any)
	if calibration["kind"] != "magnetometer" || calibration["msp_name"] != "MSP_MAG_CALIBRATION" || calibration["acknowledged"] != true {
		t.Fatalf("calibration = %+v", calibration)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_calibration" || env.SideEffects[0].Command != "MSP_MAG_CALIBRATION" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetConfigRejectsInvalidValueBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-config", "1", "2", "3", "300", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-config", "1", "2", "3", "4"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"sensor_config":{"accelerometer":1,"barometer":2,"magnetometer":3,"rangefinder":4}}`
	env, err := runTestCommandWithInput(t, []string{"sensors", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-config", "1", "2", "3", "4", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["sensor_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["accelerometer"] != float64(1) || config["barometer"] != float64(2) || config["rangefinder"] != float64(4) {
		t.Fatalf("config = %+v", config)
	}
	hardware := result["hardware"].([]any)
	if len(hardware) != 4 || hardware[0].(map[string]any)["name"] != "accelerometer" || hardware[3].(map[string]any)["name"] != "rangefinder" {
		t.Fatalf("hardware = %+v", hardware)
	}
	if result["msp_name"] != "MSP_SET_SENSOR_CONFIG" || result["save_required"] != true {
		t.Fatalf("sensor config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_config" || env.SideEffects[0].Command != "MSP_SET_SENSOR_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"sensor_config":{"accelerometer":1,"barometer":2,"magnetometer":3,"rangefinder":4}}`
	env, err := runTestCommandWithInput(t, []string{"sensors", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["sensor_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["accelerometer"] != float64(1) || config["barometer"] != float64(2) || config["rangefinder"] != float64(4) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_SENSOR_CONFIG" || result["save_required"] != true {
		t.Fatalf("sensor config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_config" || env.SideEffects[0].Command != "MSP_SET_SENSOR_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetAlignmentRejectsPartialCustomAlignmentBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-alignment", "2", "3", "-10", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetAlignmentRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-alignment", "--", "2", "3", "-10", "20", "900"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetAlignmentJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"sensors":{"alignment":{"magnetometer_alignment":2,"gyro_enabled_mask":3,"mag_custom_alignment":{"roll":-10,"pitch":20,"yaw":900}}}}`
	env, err := runTestCommandWithInput(t, []string{"sensors", "set-alignment-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetAlignmentWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "sensors", "set-alignment", "--", "2", "3", "-10", "20", "900"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["sensor_alignment"].(map[string]any)
	config := result["config"].(map[string]any)
	custom := config["mag_custom_alignment"].(map[string]any)
	if config["magnetometer_alignment"] != float64(2) || config["gyro_enabled_mask"] != float64(3) || custom["yaw"] != float64(900) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_SENSOR_ALIGNMENT" || result["save_required"] != true {
		t.Fatalf("sensor alignment result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_alignment" || env.SideEffects[0].Command != "MSP_SET_SENSOR_ALIGNMENT" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetAlignmentJSONWithFakeFC(t *testing.T) {
	input := `{"sensors":{"alignment":{"magnetometer_alignment":2,"gyro_enabled_mask":3,"mag_custom_alignment":{"roll":-10,"pitch":20,"yaw":900}}}}`
	env, err := runTestCommandWithInput(t, []string{"sensors", "set-alignment-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["sensor_alignment"].(map[string]any)
	config := result["config"].(map[string]any)
	custom := config["mag_custom_alignment"].(map[string]any)
	if config["magnetometer_alignment"] != float64(2) || config["gyro_enabled_mask"] != float64(3) || custom["yaw"] != float64(900) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_SENSOR_ALIGNMENT" || result["save_required"] != true {
		t.Fatalf("sensor alignment result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "sensor_alignment" || env.SideEffects[0].Command != "MSP_SET_SENSOR_ALIGNMENT" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetCompassDeclinationRejectsInvalidValueBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-compass-declination", "40000", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetCompassDeclinationRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"sensors", "set-compass-declination", "123"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetCompassJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"sensors":{"compass":{"declination_deci_degrees":-123}}}`
	env, err := runTestCommandWithInput(t, []string{"sensors", "set-compass-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatalf("connector should not be called")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSensorsSetCompassDeclinationWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "sensors", "set-compass-declination", "--", "-123"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["compass_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["declination_deci_degrees"] != float64(-123) || config["declination_degrees"] != -12.3 {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_COMPASS_CONFIG" || result["save_required"] != true {
		t.Fatalf("compass config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "compass_config" || env.SideEffects[0].Command != "MSP_SET_COMPASS_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSensorsSetCompassJSONWithFakeFC(t *testing.T) {
	input := `{"sensors":{"compass":{"declination_deci_degrees":-123}}}`
	env, err := runTestCommandWithInput(t, []string{"sensors", "set-compass-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["compass_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["declination_deci_degrees"] != float64(-123) || config["declination_degrees"] != -12.3 {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_COMPASS_CONFIG" || result["save_required"] != true {
		t.Fatalf("compass config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "compass_config" || env.SideEffects[0].Command != "MSP_SET_COMPASS_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}
