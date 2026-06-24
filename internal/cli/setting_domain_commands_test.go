package cli

import (
	"context"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestPIDStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"pid", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	pid := data["pid"].(map[string]any)
	controller := pid["controller"].(map[string]any)
	if controller["name"] != "BETAFLIGHT" {
		t.Fatalf("controller = %+v", controller)
	}
	gains := pid["gains"].([]any)
	first := gains[0].(map[string]any)
	if first["name"] != "ROLL" || first["p"] != float64(45) || first["i"] != float64(80) || first["d"] != float64(30) {
		t.Fatalf("first gain = %+v", first)
	}
	advanced := pid["advanced"].(map[string]any)
	if advanced["anti_gravity_gain"] != float64(3500) || advanced["feedforward_pitch"] != float64(125) {
		t.Fatalf("advanced = %+v", advanced)
	}
	rateProfile := pid["rate_profile"].(map[string]any)
	if rateProfile["rates_type_name"] != "ACTUAL" {
		t.Fatalf("rate profile = %+v", rateProfile)
	}
	simplified := pid["simplified_tuning"].(map[string]any)
	pids := simplified["pids"].(map[string]any)
	if pids["mode"] != float64(2) || pids["master_multiplier"] != float64(100) {
		t.Fatalf("simplified pids = %+v", pids)
	}
	dterm := simplified["dterm"].(map[string]any)
	if dterm["enabled"] != true || dterm["lpf1_dynamic_max_hz"] != float64(170) {
		t.Fatalf("simplified dterm = %+v", dterm)
	}
}

func TestPIDSetGainsJSONWithFakeFC(t *testing.T) {
	input := `{"gains":[{"p":45,"i":80,"d":30},{"p":47,"i":84,"d":34},{"p":45,"i":80,"d":0},{"p":50,"i":50,"d":75},{"p":40,"i":0,"d":0}]}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-gains-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["pid_gains"].(map[string]any)
	gains := result["gains"].([]any)
	first := gains[0].(map[string]any)
	if first["name"] != "ROLL" || first["p"] != float64(45) || first["i"] != float64(80) || result["save_required"] != true {
		t.Fatalf("pid_gains = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_PID" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPIDSetGainsJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[{"p":45,"i":80,"d":30},{"p":47,"i":84,"d":34},{"p":45,"i":80,"d":0},{"p":50,"i":50,"d":75},{"p":40,"i":0,"d":0}]`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-gains-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetGainsJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"p":45,"i":80,"d":30}]`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-gains-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON shape")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetAdvancedJSONWithFakeFC(t *testing.T) {
	input := `{"pid_advanced":{"feedforward_transition":20,"rate_accel_limit":100,"yaw_rate_accel_limit":120,"level_angle_limit":55,"anti_gravity_gain":3500,"iterm_rotation":1,"iterm_relax":2,"iterm_relax_type":1,"throttle_boost":5,"acro_trainer_angle_limit":20,"feedforward_roll":120,"feedforward_pitch":125,"feedforward_yaw":120,"d_max_roll":40,"d_max_pitch":42,"d_max_gain":35,"d_max_advance":50,"integrated_yaw_relax":20,"iterm_relax_cutoff":15,"motor_output_limit":95,"idle_min_rpm":30,"feedforward_averaging":2,"feedforward_smooth_factor":10,"feedforward_boost":20,"feedforward_max_rate_limit":30,"feedforward_jitter_factor":90,"vbat_sag_compensation":5,"thrust_linearization":10,"tpa":{"mode":2,"rate":15,"breakpoint":1350}}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-advanced-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["pid_advanced"].(map[string]any)
	advanced := result["advanced"].(map[string]any)
	if advanced["anti_gravity_gain"] != float64(3500) || advanced["feedforward_pitch"] != float64(125) || result["save_required"] != true {
		t.Fatalf("pid_advanced = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_PID_ADVANCED" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPIDSetAdvancedJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"pid_advanced":{"feedforward_averaging":2}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-advanced-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetAdvancedJSONValidationBeforeConnect(t *testing.T) {
	input := `{"pid_advanced":{"feedforward_averaging":4}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-advanced-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid PID advanced config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetSimplifiedJSONWithFakeFC(t *testing.T) {
	input := `{"simplified_tuning":{"pids":{"mode":2,"master_multiplier":100,"roll_pitch_ratio":100,"i_gain":100,"d_gain":100,"pi_gain":100,"d_max_gain":100,"feedforward_gain":100,"pitch_pi_gain":100},"dterm":{"enabled":true,"multiplier":100,"lpf1_static_hz":100,"lpf2_static_hz":150,"lpf1_dynamic_min_hz":70,"lpf1_dynamic_max_hz":170},"gyro":{"enabled":true,"multiplier":100,"lpf1_static_hz":150,"lpf2_static_hz":250,"lpf1_dynamic_min_hz":75,"lpf1_dynamic_max_hz":300}}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-simplified-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["simplified_tuning"].(map[string]any)
	tuning := result["tuning"].(map[string]any)
	pids := tuning["pids"].(map[string]any)
	if pids["mode"] != float64(2) || pids["master_multiplier"] != float64(100) || result["save_required"] != true {
		t.Fatalf("simplified_tuning = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "simplified_tuning" || env.SideEffects[0].Command != "MSP_SET_SIMPLIFIED_TUNING" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPIDSetSimplifiedJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"simplified_tuning":{"pids":{"mode":2,"master_multiplier":100,"roll_pitch_ratio":100,"i_gain":100,"d_gain":100,"pi_gain":100,"d_max_gain":100,"feedforward_gain":100,"pitch_pi_gain":100},"dterm":{"enabled":true,"multiplier":100},"gyro":{"enabled":true,"multiplier":100}}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-simplified-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDSetSimplifiedJSONValidationBeforeConnect(t *testing.T) {
	input := `{"simplified_tuning":{"pids":{"mode":3},"dterm":{"multiplier":100},"gyro":{"multiplier":100}}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "set-simplified-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid simplified tuning")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestPIDPreviewSimplifiedJSONWithFakeFC(t *testing.T) {
	input := `{"simplified_tuning":{"pids":{"mode":2,"master_multiplier":100,"roll_pitch_ratio":100,"i_gain":100,"d_gain":100,"pi_gain":100,"d_max_gain":100,"feedforward_gain":100,"pitch_pi_gain":100},"dterm":{"enabled":true,"multiplier":100,"lpf1_static_hz":100,"lpf2_static_hz":150,"lpf1_dynamic_min_hz":70,"lpf1_dynamic_max_hz":170},"gyro":{"enabled":true,"multiplier":100,"lpf1_static_hz":150,"lpf2_static_hz":250,"lpf1_dynamic_min_hz":75,"lpf1_dynamic_max_hz":300}}}`
	env, err := runTestCommandWithInput(t, []string{"pid", "preview-simplified-json", "-"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	preview := data["simplified_tuning_preview"].(map[string]any)
	gains := preview["pid_gains"].([]any)
	roll := gains[0].(map[string]any)
	if preview["read_only"] != true || roll["axis"] != "roll" || roll["d_max"] != float64(45) {
		t.Fatalf("preview = %+v", preview)
	}
	dterm := preview["dterm"].(map[string]any)
	if dterm["lpf1_dynamic_max_hz"] != float64(175) {
		t.Fatalf("dterm = %+v", dterm)
	}
	if len(env.SideEffects) != 0 {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestPIDValidateSimplifiedWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"pid", "validate-simplified"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	validation := data["simplified_tuning_validation"].(map[string]any)
	if validation["pids_match"] != true || validation["gyro_match"] != true || validation["dterm_match"] != false || validation["read_only"] != true {
		t.Fatalf("validation = %+v", validation)
	}
	if len(env.SideEffects) != 0 {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRatesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rates", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	rates := data["rates"].(map[string]any)
	rateProfile := rates["rate_profile"].(map[string]any)
	axes := rateProfile["axes"].([]any)
	roll := axes[0].(map[string]any)
	if roll["axis"] != "roll" || roll["rc_rate"] != float64(7) || roll["rate_limit_dps"] != float64(900) {
		t.Fatalf("roll = %+v", roll)
	}
	throttle := rateProfile["throttle"].(map[string]any)
	if throttle["limit_type_name"] != "SCALE" || throttle["hover_value"] != float64(0.45) {
		t.Fatalf("throttle = %+v", throttle)
	}
	tpa := rates["tpa"].(map[string]any)
	if tpa["mode"] != float64(2) || tpa["rate"] != float64(15) || tpa["breakpoint"] != float64(1350) {
		t.Fatalf("tpa = %+v", tpa)
	}
}

func TestRatesSetProfileJSONWithFakeFC(t *testing.T) {
	input := `{"rate_profile":{"axes":[{"axis":"roll","rc_rate":7,"expo":10,"rate":70,"rate_limit_dps":900},{"axis":"pitch","rc_rate":7,"expo":9,"rate":72,"rate_limit_dps":850},{"axis":"yaw","rc_rate":8,"expo":5,"rate":65,"rate_limit_dps":800}],"throttle":{"mid_percent":50,"expo_percent":20,"hover_percent":45,"limit_type":1,"limit_percent":80},"rates_type":3}}`
	env, err := runTestCommandWithInput(t, []string{"rates", "set-profile-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rate_profile"].(map[string]any)
	profile := result["rate_profile"].(map[string]any)
	axes := profile["axes"].([]any)
	roll := axes[0].(map[string]any)
	if roll["axis"] != "roll" || roll["rate_limit_dps"] != float64(900) || result["save_required"] != true {
		t.Fatalf("rate_profile = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RC_TUNING" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRatesSetProfileJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"axes":[{"axis":"roll"},{"axis":"pitch"},{"axis":"yaw"}]}`
	env, err := runTestCommandWithInput(t, []string{"rates", "set-profile-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestRatesSetProfileJSONValidationBeforeConnect(t *testing.T) {
	input := `{"axes":[{"axis":"roll"},{"axis":"roll"},{"axis":"yaw"}]}`
	env, err := runTestCommandWithInput(t, []string{"rates", "set-profile-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON shape")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFiltersStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"filters", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	filters := data["filters"].(map[string]any)
	advanced := filters["advanced_config"].(map[string]any)
	if advanced["pid_process_denom"] != float64(4) || advanced["motor_protocol_name"] != "DSHOT600" || advanced["gyro_check_overflow_name"] != "ALL" {
		t.Fatalf("advanced = %+v", advanced)
	}
	config := filters["filter_config"].(map[string]any)
	if config["gyro_lpf1_static_hz"] != float64(150) || config["gyro_lpf2_type_name"] != "PT2" {
		t.Fatalf("filter config = %+v", config)
	}
	dynamicNotch := config["dynamic_notch"].(map[string]any)
	if dynamicNotch["q"] != float64(300) || dynamicNotch["max_hz"] != float64(600) {
		t.Fatalf("dynamic notch = %+v", dynamicNotch)
	}
	rpmFilter := config["rpm_filter"].(map[string]any)
	weights := rpmFilter["weights"].([]any)
	if rpmFilter["harmonics"] != float64(3) || weights[2] != float64(60) {
		t.Fatalf("rpm filter = %+v", rpmFilter)
	}
}

func TestFiltersSetAdvancedJSONWithFakeFC(t *testing.T) {
	input := `{"advanced_config":{"gyro_sync_denom":1,"pid_process_denom":4,"use_unsynced_pwm":0,"motor_protocol":7,"motor_pwm_rate":480,"motor_idle":550,"gyro_use_32khz_deprecated":0,"motor_pwm_inversion":1,"gyro_to_use_deprecated":0,"gyro_high_fsr":1,"gyro_movement_calib_threshold":32,"gyro_calib_duration":125,"gyro_offset_yaw":10,"gyro_check_overflow":2,"debug_mode":5,"debug_mode_count":80}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-advanced-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["advanced_config"].(map[string]any)
	config := result["advanced_config"].(map[string]any)
	if config["pid_process_denom"] != float64(4) || config["motor_protocol"] != float64(7) || result["save_required"] != true {
		t.Fatalf("advanced_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_ADVANCED_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFiltersSetAdvancedJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"advanced_config":{"debug_mode":5,"debug_mode_count":80}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-advanced-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFiltersSetAdvancedJSONValidationBeforeConnect(t *testing.T) {
	input := `{"advanced_config":{"debug_mode":80,"debug_mode_count":80}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-advanced-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid advanced config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFiltersSetFilterJSONWithFakeFC(t *testing.T) {
	input := `{"filter_config":{"legacy_gyro_lowpass_hz":90,"gyro_lpf1_static_hz":150,"gyro_lpf2_static_hz":500,"gyro_lpf1_type":0,"gyro_lpf2_type":2,"gyro_hardware_lpf":1,"gyro_32khz_hardware_lpf_deprecated":0,"gyro_soft_notch_hz_1":0,"gyro_soft_notch_cutoff_1":0,"gyro_soft_notch_hz_2":0,"gyro_soft_notch_cutoff_2":0,"dterm_lpf1_static_hz":100,"dterm_lpf2_static_hz":150,"dterm_lpf1_type":0,"dterm_lpf2_type":2,"dterm_notch_hz":0,"dterm_notch_cutoff":0,"yaw_lowpass_hz":0,"dynamic_lowpass":{"gyro_min_hz":100,"gyro_max_hz":400,"dterm_min_hz":80,"dterm_max_hz":200,"dterm_expo":5},"dynamic_notch":{"range_deprecated":0,"width_percent_deprecated":0,"q":300,"min_hz":100,"max_hz":600,"count":3},"rpm_filter":{"harmonics":3,"min_hz":100,"fade_range_hz":50,"q":500,"weights":[100,80,60]}}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-filter-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["filter_config"].(map[string]any)
	config := result["filter_config"].(map[string]any)
	if config["gyro_lpf1_static_hz"] != float64(150) || config["dterm_lpf2_type"] != float64(2) || result["save_required"] != true {
		t.Fatalf("filter_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_FILTER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFiltersSetFilterJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"filter_config":{"rpm_filter":{"weights":[100,80,60]}}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-filter-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFiltersSetFilterJSONValidationBeforeConnect(t *testing.T) {
	input := `{"filter_config":{"rpm_filter":{"fade_range_hz":1001,"weights":[100,80,60]}}}`
	env, err := runTestCommandWithInput(t, []string{"filters", "set-filter-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid filter config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBatteryStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"battery", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	battery := data["battery"].(map[string]any)
	config := battery["config"].(map[string]any)
	if config["capacity_mah"] != float64(1300) || config["voltage_meter_source_name"] != "ADC" || config["warning_cell_voltage_v"] != float64(3.5) {
		t.Fatalf("config = %+v", config)
	}
	profile := battery["profile"].(map[string]any)
	if profile["force_cell_count"] != float64(4) || profile["full_cell_voltage_v"] != float64(4.2) {
		t.Fatalf("profile = %+v", profile)
	}
	state := battery["state"].(map[string]any)
	if state["state_name"] != "OK" || state["voltage_v"] != float64(15.99) {
		t.Fatalf("state = %+v", state)
	}
	voltageMeters := battery["voltage_meters"].([]any)
	firstVoltageMeter := voltageMeters[0].(map[string]any)
	if firstVoltageMeter["id_name"] != "BATTERY_1" || firstVoltageMeter["voltage_v"] != float64(16) {
		t.Fatalf("voltage meters = %+v", voltageMeters)
	}
	currentConfigs := battery["current_meter_configs"].([]any)
	firstCurrentConfig := currentConfigs[0].(map[string]any)
	if firstCurrentConfig["sensor_type_name"] != "ADC" || firstCurrentConfig["offset"] != float64(-10) {
		t.Fatalf("current configs = %+v", currentConfigs)
	}
}

func TestBatterySetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"battery_config":{"legacy_min_cell_voltage_v":3.3,"legacy_max_cell_voltage_v":4.3,"legacy_warning_cell_voltage_v":3.5,"capacity_mah":1300,"voltage_meter_source":1,"current_meter_source":1,"min_cell_voltage_v":3.3,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["battery_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["capacity_mah"] != float64(1300) || config["voltage_meter_source_name"] != "ADC" || result["save_required"] != true {
		t.Fatalf("battery_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "battery_config" || env.SideEffects[0].Command != "MSP_SET_BATTERY_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatterySetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"battery_config":{"capacity_mah":1300,"voltage_meter_source":1,"current_meter_source":1,"min_cell_voltage_v":3.3,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"battery", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after battery config confirmation failure")
	}
}

func TestBatterySetConfigJSONValidationBeforeConnect(t *testing.T) {
	input := `{"battery_config":{"capacity_mah":1300,"min_cell_voltage_v":3.6,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-config-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid battery config")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBatterySetProfileJSONWithFakeFC(t *testing.T) {
	input := `{"battery_profile":{"index":1,"min_cell_voltage_v":3.3,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5,"full_cell_voltage_v":4.2,"capacity_mah":1300,"force_cell_count":4,"consumption_warning_percent":20}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-profile-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["battery_profile"].(map[string]any)
	profile := result["profile"].(map[string]any)
	if profile["index"] != float64(1) || profile["full_cell_voltage_v"] != float64(4.2) || result["save_required"] != true {
		t.Fatalf("battery_profile = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "battery_profile" || env.SideEffects[0].Command != "MSP2_SET_BATTERY_PROFILE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatterySetProfileJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"battery_profile":{"index":1,"min_cell_voltage_v":3.3,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5,"full_cell_voltage_v":4.2,"capacity_mah":1300,"force_cell_count":4,"consumption_warning_percent":20}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"battery", "set-profile-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after battery profile confirmation failure")
	}
}

func TestBatterySetProfileJSONValidationBeforeConnect(t *testing.T) {
	input := `{"battery_profile":{"index":1,"min_cell_voltage_v":3.6,"max_cell_voltage_v":4.35,"warning_cell_voltage_v":3.5,"full_cell_voltage_v":4.2,"capacity_mah":1300,"force_cell_count":4,"consumption_warning_percent":20}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-profile-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid battery profile")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBatterySetVoltageMeterRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"battery", "set-voltage-meter", "10", "300", "10", "1", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after voltage meter validation failure")
	}
}

func TestBatterySetVoltageMeterRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"battery", "set-voltage-meter", "10", "110", "10", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after voltage meter confirmation failure")
	}
}

func TestBatterySetVoltageMeterJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"battery":{"voltage_meter_configs":[{"id":10,"sensor_type":0,"vbat_scale":110,"vbat_res_div_val":10,"vbat_res_div_multiplier":1}]}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"battery", "set-voltage-meter-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after voltage meter confirmation failure")
	}
}

func TestBatterySetVoltageMeterWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"battery", "set-voltage-meter", "10", "110", "10", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["voltage_meter_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["id"] != float64(10) || config["vbat_scale"] != float64(110) || config["vbat_res_div_val"] != float64(10) || config["vbat_res_div_multiplier"] != float64(1) {
		t.Fatalf("voltage meter config = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_VOLTAGE_METER_CONFIG" || result["save_required"] != true {
		t.Fatalf("voltage meter config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "voltage_meter_config" || env.SideEffects[0].Command != "MSP_SET_VOLTAGE_METER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatterySetVoltageMeterJSONWithFakeFC(t *testing.T) {
	input := `{"battery":{"voltage_meter_configs":[{"id":10,"sensor_type":0,"vbat_scale":110,"vbat_res_div_val":10,"vbat_res_div_multiplier":1}]}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-voltage-meter-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["voltage_meter_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["id"] != float64(10) || config["vbat_scale"] != float64(110) || config["vbat_res_div_val"] != float64(10) || config["vbat_res_div_multiplier"] != float64(1) {
		t.Fatalf("voltage meter config = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_VOLTAGE_METER_CONFIG" || result["save_required"] != true {
		t.Fatalf("voltage meter config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "voltage_meter_config" || env.SideEffects[0].Command != "MSP_SET_VOLTAGE_METER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatterySetCurrentMeterRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"--yes", "battery", "set-current-meter", "--", "10", "40000", "-10"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after current meter validation failure")
	}
}

func TestBatterySetCurrentMeterRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"battery", "set-current-meter", "--", "10", "400", "-10"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after current meter confirmation failure")
	}
}

func TestBatterySetCurrentMeterJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"battery":{"current_meter_configs":[{"id":10,"sensor_type":1,"scale":400,"offset":-10}]}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"battery", "set-current-meter-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after current meter confirmation failure")
	}
}

func TestBatterySetCurrentMeterWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "battery", "set-current-meter", "--", "10", "400", "-10"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["current_meter_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["id"] != float64(10) || config["scale"] != float64(400) || config["offset"] != float64(-10) {
		t.Fatalf("current meter config = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_CURRENT_METER_CONFIG" || result["save_required"] != true {
		t.Fatalf("current meter config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "current_meter_config" || env.SideEffects[0].Command != "MSP_SET_CURRENT_METER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBatterySetCurrentMeterJSONWithFakeFC(t *testing.T) {
	input := `{"battery":{"current_meter_configs":[{"id":10,"sensor_type":1,"scale":400,"offset":-10}]}}`
	env, err := runTestCommandWithInput(t, []string{"battery", "set-current-meter-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["current_meter_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["id"] != float64(10) || config["scale"] != float64(400) || config["offset"] != float64(-10) {
		t.Fatalf("current meter config = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_CURRENT_METER_CONFIG" || result["save_required"] != true {
		t.Fatalf("current meter config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "current_meter_config" || env.SideEffects[0].Command != "MSP_SET_CURRENT_METER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFailsafeStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"failsafe", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	failsafeStatus := data["failsafe"].(map[string]any)
	armingConfig := failsafeStatus["arming_config"].(map[string]any)
	if armingConfig["auto_disarm_delay_s"] != float64(5) || armingConfig["small_angle_degrees"] != float64(25) || armingConfig["gyro_cal_on_first_arm"] != true {
		t.Fatalf("arming config = %+v", armingConfig)
	}
	failsafe := failsafeStatus["failsafe_config"].(map[string]any)
	if failsafe["switch_mode_name"] != "STAGE2" || failsafe["procedure_name"] != "DROP" {
		t.Fatalf("failsafe = %+v", failsafe)
	}
	board := failsafeStatus["board_alignment"].(map[string]any)
	if board["roll_degrees"] != float64(-2) || board["yaw_degrees"] != float64(90) {
		t.Fatalf("board = %+v", board)
	}
	arming := failsafeStatus["arming"].(map[string]any)
	if arming["flag_count"] != float64(29) || arming["disabled"] != true {
		t.Fatalf("arming = %+v", arming)
	}
	activeNames := arming["active_names"].([]any)
	if activeNames[0] != "RXLOSS" || activeNames[len(activeNames)-1] != "CALIB" {
		t.Fatalf("active names = %+v", activeNames)
	}
}

func TestFailsafeSetBoardAlignmentRequiresValidInt16(t *testing.T) {
	env, err := runTestCommand(t, []string{"failsafe", "set-board-alignment", "0", "40000", "0", "--yes"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestFailsafeSetArmingJSONWithFakeFC(t *testing.T) {
	input := `{"arming_config":{"auto_disarm_delay_s":5,"small_angle_degrees":25,"gyro_cal_on_first_arm":true}}`
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-arming-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["arming_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["auto_disarm_delay_s"] != float64(5) || config["small_angle_degrees"] != float64(25) || config["gyro_cal_on_first_arm"] != true || result["save_required"] != true {
		t.Fatalf("arming_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "arming_config" || env.SideEffects[0].Command != "MSP_SET_ARMING_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFailsafeSetArmingJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"arming_config":{"auto_disarm_delay_s":5,"small_angle_degrees":25,"gyro_cal_on_first_arm":true}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-arming-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after arming config confirmation failure")
	}
}

func TestFailsafeSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"failsafe_config":{"delay_tenths_s":15,"landing_time_s":60,"throttle":1000,"switch_mode":2,"throttle_low_delay_tenths_s":100,"procedure":1}}`
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["failsafe_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["throttle"] != float64(1000) || config["switch_mode"] != float64(2) || result["save_required"] != true {
		t.Fatalf("failsafe_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "failsafe_config" || env.SideEffects[0].Command != "MSP_SET_FAILSAFE_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFailsafeSetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"failsafe_config":{"delay_tenths_s":15,"landing_time_s":60,"throttle":1000,"switch_mode":2,"throttle_low_delay_tenths_s":100,"procedure":1}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after failsafe config confirmation failure")
	}
}

func TestFailsafeSetBoardAlignmentRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"failsafe", "set-board-alignment", "--", "-2", "3", "90"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after board alignment confirmation failure")
	}
}

func TestFailsafeSetBoardAlignmentJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"board_alignment":{"roll_degrees":-2,"pitch_degrees":3,"yaw_degrees":90}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-board-alignment-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after board alignment confirmation failure")
	}
}

func TestFailsafeSetBoardAlignmentWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "failsafe", "set-board-alignment", "--", "-2", "3", "90"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["board_alignment"].(map[string]any)
	alignment := result["board_alignment"].(map[string]any)
	if alignment["roll_degrees"] != float64(-2) || alignment["pitch_degrees"] != float64(3) || alignment["yaw_degrees"] != float64(90) {
		t.Fatalf("board alignment = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_BOARD_ALIGNMENT_CONFIG" || result["save_required"] != true {
		t.Fatalf("board alignment = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "board_alignment" || env.SideEffects[0].Command != "MSP_SET_BOARD_ALIGNMENT_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFailsafeSetBoardAlignmentJSONWithFakeFC(t *testing.T) {
	input := `{"failsafe":{"board_alignment":{"roll_degrees":-2,"pitch_degrees":3,"yaw_degrees":90}}}`
	env, err := runTestCommandWithInput(t, []string{"failsafe", "set-board-alignment-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["board_alignment"].(map[string]any)
	alignment := result["board_alignment"].(map[string]any)
	if alignment["roll_degrees"] != float64(-2) || alignment["pitch_degrees"] != float64(3) || alignment["yaw_degrees"] != float64(90) {
		t.Fatalf("board alignment = %+v", result)
	}
	if result["msp_name"] != "MSP_SET_BOARD_ALIGNMENT_CONFIG" || result["save_required"] != true {
		t.Fatalf("board alignment = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "board_alignment" || env.SideEffects[0].Command != "MSP_SET_BOARD_ALIGNMENT_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSettingDomainListsWithFakeFC(t *testing.T) {
	tests := []struct {
		args []string
		key  string
	}{
		{[]string{"pid", "list"}, "settings"},
		{[]string{"rates", "list"}, "settings"},
		{[]string{"filters", "list"}, "settings"},
		{[]string{"receiver", "list"}, "settings"},
		{[]string{"vtx", "list"}, "settings"},
		{[]string{"osd", "list"}, "settings"},
		{[]string{"gps", "list"}, "settings"},
		{[]string{"battery", "list"}, "settings"},
		{[]string{"failsafe", "list"}, "settings"},
	}
	for _, tt := range tests {
		env, err := runTestCommand(t, tt.args, nil)
		if err != nil {
			t.Fatalf("%v command error = %v", tt.args, err)
		}
		if !env.OK {
			t.Fatalf("%v env.OK = false: %+v", tt.args, env.Errors)
		}
		data := env.Data.(map[string]any)
		view := data["view"].(map[string]any)
		items := view[tt.key].([]any)
		if len(items) == 0 {
			t.Fatalf("%v view = %+v", tt.args, view)
		}
		if tt.args[0] == "vtx" {
			if _, ok := view["vtx"]; !ok {
				t.Fatalf("%v view = %+v", tt.args, view)
			}
			table, ok := view["vtx_table"].([]any)
			if !ok || len(table) == 0 {
				t.Fatalf("%v vtx_table = %+v", tt.args, view["vtx_table"])
			}
		}
		if tt.args[0] == "osd" {
			if _, ok := view["osd"]; !ok {
				t.Fatalf("%v view = %+v", tt.args, view)
			}
			osdLines, ok := view["lines"].([]any)
			if !ok || len(osdLines) == 0 {
				t.Fatalf("%v osd lines = %+v", tt.args, view["lines"])
			}
		}
	}
}

func TestSettingDomainSetPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"pid", "set", "p_roll", "46"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if lines[0] != "set p_roll = 46" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only domain set")
	}
}

func TestSettingDomainSetRejectsWrongDomain(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"pid", "set", "roll_rc_rate", "8"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "unknown_setting" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after domain validation failure")
	}
}

func TestSettingDomainSetApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"pid", "set", "p_roll", "46", "--apply", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["applied"] != true {
		t.Fatalf("data = %+v", data)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "set p_roll = 46" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}
