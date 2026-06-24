package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestMixerStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"mixer", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	mixer := data["mixer"].(map[string]any)
	mode := mixer["mixer"].(map[string]any)
	if mode["cli_name"] != "QUADX" || mode["display_name"] != "Quad X" || mode["motor_count"] != float64(4) {
		t.Fatalf("mixer mode = %+v", mode)
	}
	if mixer["yaw_motors_reversed"] != true {
		t.Fatalf("mixer = %+v", mixer)
	}
	commands := mixer["cli_commands"].([]any)
	if len(commands) != 2 || commands[0] != "mixer QUADX" || commands[1] != "set yaw_motors_reversed = ON" {
		t.Fatalf("commands = %+v", commands)
	}
}

func TestMixerSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"mixer_config":{"mode":3,"yaw_motors_reversed":true}}`
	env, err := runTestCommandWithInput(t, []string{"mixer", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["mixer_config"].(map[string]any)
	config := result["config"].(map[string]any)
	mode := result["mode"].(map[string]any)
	if config["mode"] != float64(3) || config["yaw_motors_reversed"] != true || mode["cli_name"] != "QUADX" || result["save_required"] != true {
		t.Fatalf("mixer_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "mixer_config" || env.SideEffects[0].Command != "MSP_SET_MIXER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMixerSetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"mixer_config":{"mode":3,"yaw_motors_reversed":true}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"mixer", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after mixer config confirmation failure")
	}
}

func TestMotorsStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	motors := data["motors"].(map[string]any)
	config := motors["config"].(map[string]any)
	if config["max_throttle"] != float64(2000) || config["motor_count"] != float64(4) {
		t.Fatalf("config = %+v", config)
	}
	outputs := motors["outputs"].([]any)
	if len(outputs) != 8 || outputs[3] != float64(1003) {
		t.Fatalf("outputs = %+v", outputs)
	}
	telemetry := motors["telemetry"].([]any)
	first := telemetry[0].(map[string]any)
	if first["rpm"] != float64(12500) || first["voltage_v"] != 16.8 {
		t.Fatalf("telemetry = %+v", telemetry)
	}
	escData := motors["esc_sensor_data"].([]any)
	firstESC := escData[0].(map[string]any)
	if firstESC["temperature_c"] != float64(40) || firstESC["rpm"] != float64(12000) {
		t.Fatalf("esc sensor data = %+v", escData)
	}
}

func TestMotorsSetConfigRejectsInvalidBoolBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-config", "2000", "1000", "14", "2", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestMotorsSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-config", "2000", "1000", "14", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestMotorsSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-config", "2000", "1000", "14", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["motor_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["max_throttle"] != float64(2000) || config["min_command"] != float64(1000) || config["use_dshot_telemetry"] != true {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_MOTOR_CONFIG" || result["save_required"] != true {
		t.Fatalf("motor config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_config" || env.SideEffects[0].Command != "MSP_SET_MOTOR_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMotorsSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"motor_config":{"max_throttle":2000,"min_command":1000,"motor_poles":14,"use_dshot_telemetry":true}}`
	env, err := runTestCommandWithInput(t, []string{"motors", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestMotorsSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"config":{"max_throttle":2000,"min_command":1000,"motor_poles":14,"use_dshot_telemetry":true}}`
	env, err := runTestCommandWithInput(t, []string{"motors", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["motor_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["max_throttle"] != float64(2000) || config["min_command"] != float64(1000) || config["use_dshot_telemetry"] != true {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_MOTOR_CONFIG" || result["save_required"] != true {
		t.Fatalf("motor config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_config" || env.SideEffects[0].Command != "MSP_SET_MOTOR_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMotorsSet3DConfigRejectsInvalidValueBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-3d-config", "1406", "70000", "1460", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestMotorsSet3DConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-3d-config", "1406", "1514", "1460"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestMotorsSet3DConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"motor_3d_config":{"deadband_low":1406,"deadband_high":1514,"neutral":1460}}`
	env, err := runTestCommandWithInput(t, []string{"motors", "set-3d-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestMotorsSet3DConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "set-3d-config", "1406", "1514", "1460", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["motor_3d_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["deadband_low"] != float64(1406) || config["deadband_high"] != float64(1514) || config["neutral"] != float64(1460) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_MOTOR_3D_CONFIG" || result["save_required"] != true {
		t.Fatalf("motor 3d config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_3d_config" || env.SideEffects[0].Command != "MSP_SET_MOTOR_3D_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMotorsSet3DConfigJSONWithFakeFC(t *testing.T) {
	input := `{"config":{"deadband_low":1406,"deadband_high":1514,"neutral":1460}}`
	env, err := runTestCommandWithInput(t, []string{"motors", "set-3d-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["motor_3d_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["deadband_low"] != float64(1406) || config["deadband_high"] != float64(1514) || config["neutral"] != float64(1460) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_MOTOR_3D_CONFIG" || result["save_required"] != true {
		t.Fatalf("motor 3d config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_3d_config" || env.SideEffects[0].Command != "MSP_SET_MOTOR_3D_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMotorsTestPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"motors", "test-plan", "--motor", "2", "--value", "1100", "--duration", "1500ms", "--props-off"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if called {
		t.Fatal("connector was called for motors test-plan")
	}
	plan := env.Data.(map[string]any)["motor_test_plan"].(map[string]any)
	if plan["applied"] != false || plan["dangerous"] != true || plan["command_preview"] != "motor 2 1100" || plan["duration_ms"] != float64(1500) {
		t.Fatalf("plan = %+v", plan)
	}
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 1 || commandPreviews[0] != "motor 2 1100" {
		t.Fatalf("command_previews = %+v", commandPreviews)
	}
	checks := plan["safety_checks"].([]any)
	if checks[0].(map[string]any)["passed"] != true || checks[1].(map[string]any)["passed"] != false {
		t.Fatalf("checks = %+v", checks)
	}
	if checks[2].(map[string]any)["name"] != "dry_run_only" || checks[2].(map[string]any)["passed"] != true {
		t.Fatalf("plan mode check = %+v", checks[2])
	}
}

func TestMotorsTestPlanAllMotors(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "test-plan", "--all", "--motor-count", "4", "--value", "1120", "--duration", "500ms", "--props-off"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)["motor_test_plan"].(map[string]any)
	if plan["all_motors"] != true || int(plan["motor_count"].(float64)) != 4 {
		t.Fatalf("plan = %+v", plan)
	}
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 4 || commandPreviews[0] != "motor 0 1120" || commandPreviews[3] != "motor 3 1120" {
		t.Fatalf("command_previews = %+v", commandPreviews)
	}
	stopPreviews := plan["stop_command_previews"].([]any)
	if len(stopPreviews) != 4 || stopPreviews[0] != "motor 0 1000" || stopPreviews[3] != "motor 3 1000" {
		t.Fatalf("stop_command_previews = %+v", stopPreviews)
	}
}

func TestMotorsTestPlanValidatesBounds(t *testing.T) {
	env, err := runTestCommand(t, []string{"motors", "test-plan", "--value", "2500"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorsTestApplyRequiresConfirmationBeforeConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"motors", "test-apply", "--motor", "1", "--value", "1050", "--duration", "1ms", "--props-off", "--battery-aware"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if called {
		t.Fatal("connector was called before --yes confirmation")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorsTestApplyUsesDangerousOperation(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommand(t, []string{"motors", "test-apply", "--motor", "1", "--value", "1050", "--duration", "1ms", "--props-off", "--battery-aware", "--yes"}, func(ctx context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(ctx)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", gotOp)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)["motor_test_plan"].(map[string]any)
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 1 || plan["command_preview"] != "motor 1 1050" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan["applied"] != true || plan["stopped"] != true || plan["command_preview"] != "motor 1 1050" || plan["stop_command_preview"] != "motor 1 1000" {
		t.Fatalf("plan = %+v", plan)
	}
	preflight := plan["preflight"].(map[string]any)
	if preflight["read_only"] != true || preflight["arming_blocked"] != true {
		t.Fatalf("preflight = %+v", preflight)
	}
	postStop := plan["post_stop"].(map[string]any)
	if postStop["read_only"] != true || len(postStop["outputs"].([]any)) == 0 || len(postStop["telemetry_rpm"].([]any)) == 0 {
		t.Fatalf("post stop = %+v", postStop)
	}
	comparison := plan["comparison"].(map[string]any)
	if comparison["read_only"] != true || comparison["preflight_captured"] != true || comparison["post_stop_captured"] != true || comparison["output_count"].(float64) == 0 {
		t.Fatalf("comparison = %+v", comparison)
	}
	audit := plan["audit"].(map[string]any)
	if audit["safety_passed"] != true || audit["preflight_captured"] != true || audit["post_stop_captured"] != true || audit["stop_succeeded"] != true {
		t.Fatalf("audit = %+v", audit)
	}
	checks := plan["safety_checks"].([]any)
	if checks[2].(map[string]any)["name"] != "bounded_apply" || checks[2].(map[string]any)["passed"] != true {
		t.Fatalf("apply mode check = %+v", checks[2])
	}
	if audit["requested_duration_ms"] != float64(1) || audit["elapsed_duration_ms"].(float64) < 0 {
		t.Fatalf("audit timing = %+v", audit)
	}
	if audit["start_command"] != "motor 1 1050" || audit["stop_command"] != "motor 1 1000" {
		t.Fatalf("audit = %+v", audit)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_output" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMotorsTestApplyAllMotors(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommand(t, []string{"motors", "test-apply", "--all", "--motor-count", "3", "--value", "1060", "--duration", "1ms", "--props-off", "--battery-aware", "--yes"}, func(ctx context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(ctx)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", gotOp)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)["motor_test_plan"].(map[string]any)
	if plan["all_motors"] != true || plan["applied"] != true || plan["stopped"] != true {
		t.Fatalf("plan = %+v", plan)
	}
	commandPreviews := plan["command_previews"].([]any)
	if len(commandPreviews) != 3 {
		t.Fatalf("command_previews = %+v", commandPreviews)
	}
	stopPreviews := plan["stop_command_previews"].([]any)
	if len(stopPreviews) != 3 {
		t.Fatalf("stop_command_previews = %+v", stopPreviews)
	}
}
