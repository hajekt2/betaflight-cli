package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestServosStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	servos := data["servos"].(map[string]any)
	outputs := servos["outputs"].([]any)
	if len(outputs) != 4 || outputs[1] != float64(1501) {
		t.Fatalf("outputs = %+v", outputs)
	}
	configs := servos["configurations"].([]any)
	first := configs[0].(map[string]any)
	if first["min"] != float64(1000) || first["reversed_sources_mask"] != float64(5) {
		t.Fatalf("configs = %+v", configs)
	}
	rules := servos["mix_rules"].([]any)
	rule := rules[0].(map[string]any)
	if rule["active"] != true || rule["rate"] != float64(100) {
		t.Fatalf("rules = %+v", rules)
	}
}

func TestServosSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "servos", "set-config", "--", "1", "1100", "1900", "1501", "-50", "2", "5"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["servo_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(1) || config["rate"] != float64(-50) || result["save_required"] != true {
		t.Fatalf("servo_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_SERVO_CONFIGURATION" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"servo_config":{"index":1,"min":1100,"max":1900,"middle":1501,"rate":-50,"forward_from_channel":2,"reversed_sources_mask":5}}`
	env, err := runTestCommandWithInput(t, []string{"servos", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["servo_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(1) || config["rate"] != float64(-50) || result["save_required"] != true {
		t.Fatalf("servo_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "servo_config" || env.SideEffects[0].Command != "MSP_SET_SERVO_CONFIGURATION" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "set-config", "--", "1", "1100", "1900", "1501", "-50", "2", "5"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestServosSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"config":{"index":1,"min":1100,"max":1900,"middle":1501,"rate":-50,"forward_from_channel":2,"reversed_sources_mask":5}}`
	env, err := runTestCommandWithInput(t, []string{"servos", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestServosSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "servos", "set-config", "--", "1", "1100", "1900", "1501", "-200", "2", "5"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestServosSetJSONWithFakeFC(t *testing.T) {
	input := `{"servo_table":{"configurations":[{"index":1,"min":1100,"max":1900,"middle":1501,"rate":-50,"forward_from_channel":2,"reversed_sources_mask":5}],"mix_rules":[{"index":2,"target_channel":1,"input_source":3,"rate":-25,"speed":10,"min":5,"max":95,"box":4}]}}`
	env, err := runTestCommandWithInput(t, []string{"servos", "set-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["servo_table"].(map[string]any)
	if result["configuration_count"] != float64(1) || result["mix_rule_count"] != float64(1) || result["save_required"] != true {
		t.Fatalf("servo_table = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_SERVO_CONFIGURATION,MSP_SET_SERVO_MIX_RULE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosSetJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"configurations":[{"index":1,"min":1100,"max":1900,"middle":1501,"rate":-50,"forward_from_channel":2,"reversed_sources_mask":5}]}`
	env, err := runTestCommandWithInput(t, []string{"servos", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestServosSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"configurations":[{"index":1,"min":1900,"max":1100,"middle":1501,"rate":-50,"forward_from_channel":2,"reversed_sources_mask":5}]}`
	env, err := runTestCommandWithInput(t, []string{"servos", "set-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestServosSetMixRuleWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"--yes", "servos", "set-mix-rule", "--", "2", "1", "3", "-25", "10", "5", "95", "4"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["servo_mix_rule"].(map[string]any)
	rule := result["rule"].(map[string]any)
	if rule["index"] != float64(2) || rule["rate"] != float64(-25) || rule["active"] != true || result["save_required"] != true {
		t.Fatalf("servo_mix_rule = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_SERVO_MIX_RULE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosSetMixRuleJSONWithFakeFC(t *testing.T) {
	input := `{"servo_mix_rule":{"index":2,"target_channel":1,"input_source":3,"rate":-25,"speed":10,"min":5,"max":95,"box":4}}`
	env, err := runTestCommandWithInput(t, []string{"servos", "set-mix-rule-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["servo_mix_rule"].(map[string]any)
	rule := result["rule"].(map[string]any)
	if rule["index"] != float64(2) || rule["rate"] != float64(-25) || rule["active"] != true || result["save_required"] != true {
		t.Fatalf("servo_mix_rule = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "servo_mix_rule" || env.SideEffects[0].Command != "MSP_SET_SERVO_MIX_RULE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestServosSetMixRuleRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"servos", "set-mix-rule", "--", "2", "1", "3", "-25", "10", "5", "95", "4"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestServosSetMixRuleJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"rule":{"index":2,"target_channel":1,"input_source":3,"rate":-25,"speed":10,"min":5,"max":95,"box":4}}`
	env, err := runTestCommandWithInput(t, []string{"servos", "set-mix-rule-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestTableDomainListsWithFakeFC(t *testing.T) {
	tests := []struct {
		args []string
		key  string
	}{
		{[]string{"vtxtable", "list"}, "vtx_table"},
		{[]string{"leds", "list"}, "leds"},
		{[]string{"servos", "list"}, "servos"},
		{[]string{"adjustments", "list"}, "adjranges"},
		{[]string{"rxrange", "list"}, "rxranges"},
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
	}
}

func TestSerialListIncludesDecodedFunctions(t *testing.T) {
	env, err := runTestCommand(t, []string{"serial", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	serial := view["serial"].([]any)
	first := serial[0].(map[string]any)
	functions := first["functions"].([]any)
	if first["function_mask_value"] != float64(64) || functions[0] != "RX_SERIAL" || first["msp_baudrate"] != "115200" {
		t.Fatalf("serial view = %+v", first)
	}
}

func TestAdjustmentsStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	adjustments := data["adjustments"].(map[string]any)
	if adjustments["source"] != "MSP_ADJUSTMENT_RANGES" {
		t.Fatalf("adjustments = %+v", adjustments)
	}
	ranges := adjustments["ranges"].([]any)
	if len(ranges) != 2 {
		t.Fatalf("ranges = %+v", ranges)
	}
	second := ranges[1].(map[string]any)
	if second["adjustment_function_name"] != "BATTERY_PROFILE" || second["range_start_us"] != float64(1000) || second["cli_command"] != "adjrange 1 0 1 1000 1100 33 2 1600 50" {
		t.Fatalf("second range = %+v", second)
	}
}

func TestAdjustmentsSetRangeWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "set-range", "1", "0", "2", "4", "8", "33", "3", "1600", "50", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["adjustment_range"].(map[string]any)
	row := result["range"].(map[string]any)
	if row["index"] != float64(1) || row["range_start_us"] != float64(1000) || row["adjustment_function_name"] != "BATTERY_PROFILE" || result["save_required"] != true {
		t.Fatalf("adjustment_range = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_ADJUSTMENT_RANGE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestAdjustmentsSetRangeJSONWithFakeFC(t *testing.T) {
	input := `{"adjustment_range":{"index":1,"slot_index":0,"aux_channel_index":2,"range_start_step":4,"range_end_step":8,"adjustment_function":33,"aux_switch_channel_index":3,"adjustment_center":1600,"adjustment_scale":50}}`
	env, err := runTestCommandWithInput(t, []string{"adjustments", "set-range-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["adjustment_range"].(map[string]any)
	row := result["range"].(map[string]any)
	if row["index"] != float64(1) || row["range_start_us"] != float64(1000) || row["adjustment_function_name"] != "BATTERY_PROFILE" || result["save_required"] != true {
		t.Fatalf("adjustment_range = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "adjustment_range" || env.SideEffects[0].Command != "MSP_SET_ADJUSTMENT_RANGE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestAdjustmentsSetJSONWithFakeFC(t *testing.T) {
	input := `{"adjustment_table":{"ranges":[{"index":1,"slot_index":0,"aux_channel_index":2,"range_start_step":4,"range_end_step":8,"adjustment_function":33,"aux_switch_channel_index":3,"adjustment_center":1600,"adjustment_scale":50}]}}`
	env, err := runTestCommandWithInput(t, []string{"adjustments", "set-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["adjustment_table"].(map[string]any)
	ranges := result["ranges"].([]any)
	row := ranges[0].(map[string]any)
	if result["range_count"] != float64(1) || row["adjustment_function_name"] != "BATTERY_PROFILE" || result["save_required"] != true {
		t.Fatalf("adjustment_table = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_ADJUSTMENT_RANGE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestAdjustmentsSetJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[{"index":1,"slot_index":0,"aux_channel_index":2,"range_start_step":4,"range_end_step":8,"adjustment_function":33,"aux_switch_channel_index":3,"adjustment_center":1600,"adjustment_scale":50}]`
	env, err := runTestCommandWithInput(t, []string{"adjustments", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestAdjustmentsSetJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"index":1,"slot_index":0,"aux_channel_index":2,"range_start_step":8,"range_end_step":4,"adjustment_function":33,"aux_switch_channel_index":3,"adjustment_center":1600,"adjustment_scale":50}]`
	env, err := runTestCommandWithInput(t, []string{"adjustments", "set-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestAdjustmentsSetRangeRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "set-range", "1", "0", "2", "4", "8", "33", "3", "1600", "50"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestAdjustmentsSetRangeJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"range":{"index":1,"slot_index":0,"aux_channel_index":2,"range_start_step":4,"range_end_step":8,"adjustment_function":33,"aux_switch_channel_index":3,"adjustment_center":1600,"adjustment_scale":50}}`
	env, err := runTestCommandWithInput(t, []string{"adjustments", "set-range-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestAdjustmentsSetRangeValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"adjustments", "set-range", "1", "0", "2", "4", "8", "33", "3", "99999", "50", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSerialStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"serial", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	status := data["serial"].(map[string]any)
	if status["source"] != "MSP2_COMMON_SERIAL_CONFIG" {
		t.Fatalf("serial status = %+v", status)
	}
	ports := status["ports"].([]any)
	if len(ports) != 2 {
		t.Fatalf("ports = %+v", ports)
	}
	first := ports[0].(map[string]any)
	if first["identifier_name"] != "USB_VCP" || first["msp_baudrate"] != "115200" {
		t.Fatalf("first = %+v", first)
	}
	second := ports[1].(map[string]any)
	functions := second["functions"].([]any)
	if second["identifier_name"] != "UART1" || second["function_mask"] != float64(66) || functions[0] != "GPS" || functions[1] != "RX_SERIAL" {
		t.Fatalf("second = %+v", second)
	}
}

func TestSerialApplyConfigJSONWithFakeFC(t *testing.T) {
	input := `{"ports":[{"identifier":51,"function_mask":66,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":0,"blackbox_baudrate_index":0}]}`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["serial_config"].(map[string]any)
	ports := result["ports"].([]any)
	first := ports[0].(map[string]any)
	if result["save_required"] != true || first["identifier_name"] != "UART1" || first["function_mask"] != float64(66) {
		t.Fatalf("serial_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_CF_SERIAL_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestSerialApplyConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[{"identifier":51,"function_mask":66,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":0,"blackbox_baudrate_index":0}]`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestSerialApplyConfigJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"identifier":51,"function_mask":70000,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":0,"blackbox_baudrate_index":0}]`
	env, err := runTestCommandWithInput(t, []string{"serial", "apply-config-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestLEDStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"leds", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	status := data["leds"].(map[string]any)
	strip := status["strip"].(map[string]any)
	if strip["advanced_supported"] != true || strip["profile"] != float64(0) {
		t.Fatalf("strip = %+v", strip)
	}
	leds := strip["leds"].([]any)
	if len(leds) != 2 {
		t.Fatalf("leds = %+v", leds)
	}
	first := leds[0].(map[string]any)
	directions := first["directions"].([]any)
	overlays := first["overlays"].([]any)
	if first["cli_syntax"] != "1,2:ne:Ct:3" || directions[0] != "n" || directions[1] != "e" || overlays[0] != "t" {
		t.Fatalf("first = %+v", first)
	}
	colors := status["colors"].([]any)
	secondColor := colors[1].(map[string]any)
	if secondColor["hue"] != float64(120) || secondColor["sat"] != float64(255) {
		t.Fatalf("colors = %+v", colors)
	}
	values := status["values"].(map[string]any)
	if values["brightness"] != float64(50) || values["rainbow_freq"] != float64(120) {
		t.Fatalf("values = %+v", values)
	}
}

func TestLEDSetValuesRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"leds", "set-values", "300", "20", "120", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after LED values validation failure")
	}
}

func TestLEDSetJSONPlansDoNotConnect(t *testing.T) {
	input := `{"leds":[{"index":0,"config":"0,0::C:0"},{"index":1,"config":"1,0::W:0"}]}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"leds", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if data["applied"] != false || len(lines) != 2 || lines[0] != "led 0 0,0::C:0" || lines[1] != "led 1 1,0::W:0" {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for LED JSON plan")
	}
}

func TestLEDSetJSONApplyWithFakeFC(t *testing.T) {
	input := `{"led":{"index":0,"config":"0,0::C:0"}}`
	env, err := runTestCommandWithInput(t, []string{"leds", "set-json", "-", "--apply", "--yes"}, input, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "led 0 0,0::C:0" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestLEDSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"led":{"index":0,"config":"bad\nline"}}`
	env, err := runTestCommandWithInput(t, []string{"leds", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid LED row JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestLEDSetValuesRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"leds", "set-values", "50", "20", "120"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after LED values confirmation failure")
	}
}

func TestLEDSetValuesWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"leds", "set-values", "50", "20", "120", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["led_values"].(map[string]any)
	values := result["values"].(map[string]any)
	if values["brightness"] != float64(50) || values["rainbow_delta"] != float64(20) || values["rainbow_freq"] != float64(120) {
		t.Fatalf("led values = %+v", result)
	}
	if result["msp_name"] != "MSP2_SET_LED_STRIP_CONFIG_VALUES" || result["save_required"] != true {
		t.Fatalf("led values = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "led_values" || env.SideEffects[0].Command != "MSP2_SET_LED_STRIP_CONFIG_VALUES" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestLEDSetValuesJSONWithFakeFC(t *testing.T) {
	input := `{"led_values":{"brightness":50,"rainbow_delta":20,"rainbow_freq":120}}`
	env, err := runTestCommandWithInput(t, []string{"leds", "set-values-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["led_values"].(map[string]any)
	values := result["values"].(map[string]any)
	if values["brightness"] != float64(50) || values["rainbow_delta"] != float64(20) || values["rainbow_freq"] != float64(120) {
		t.Fatalf("led values = %+v", result)
	}
	if result["msp_name"] != "MSP2_SET_LED_STRIP_CONFIG_VALUES" || result["save_required"] != true {
		t.Fatalf("led values = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "led_values" || env.SideEffects[0].Command != "MSP2_SET_LED_STRIP_CONFIG_VALUES" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestLEDSetValuesJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"values":{"brightness":50,"rainbow_delta":20,"rainbow_freq":120}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"leds", "set-values-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after LED values JSON confirmation failure")
	}
}

func TestLEDSetColorsJSONWithFakeFC(t *testing.T) {
	input := `{"colors":[{"index":0,"hue":0,"sat":0,"val":255},{"index":1,"hue":120,"sat":255,"val":255}]}`
	env, err := runTestCommandWithInput(t, []string{"leds", "set-colors-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["led_colors"].(map[string]any)
	colors := result["colors"].([]any)
	second := colors[1].(map[string]any)
	if second["hue"] != float64(120) || second["sat"] != float64(255) || result["save_required"] != true {
		t.Fatalf("led colors = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "led_colors" || env.SideEffects[0].Command != "MSP_SET_LED_COLORS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestLEDSetColorsJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `[{"hue":0,"sat":0,"val":255}]`
	called := false
	env, err := runTestCommandWithInput(t, []string{"leds", "set-colors-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after LED color confirmation failure")
	}
}

func TestLEDSetColorsJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"hue":360,"sat":0,"val":255}]`
	env, err := runTestCommandWithInput(t, []string{"leds", "set-colors-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid LED colors")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestLEDSetModeColorWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"leds", "set-mode-color", "1", "2", "5", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["led_mode_color"].(map[string]any)
	modeColor := result["mode_color"].(map[string]any)
	if modeColor["mode"] != float64(1) || modeColor["direction"] != float64(2) || modeColor["color"] != float64(5) || result["save_required"] != true {
		t.Fatalf("led mode color = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "led_mode_color" || env.SideEffects[0].Command != "MSP_SET_LED_STRIP_MODECOLOR" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestLEDSetModeColorJSONWithFakeFC(t *testing.T) {
	input := `{"mode_color":{"mode":1,"direction":2,"color":5}}`
	env, err := runTestCommandWithInput(t, []string{"leds", "set-mode-color-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["led_mode_color"].(map[string]any)
	modeColor := result["mode_color"].(map[string]any)
	if modeColor["mode"] != float64(1) || modeColor["direction"] != float64(2) || modeColor["color"] != float64(5) || result["save_required"] != true {
		t.Fatalf("led mode color = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "led_mode_color" || env.SideEffects[0].Command != "MSP_SET_LED_STRIP_MODECOLOR" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestLEDSetModeColorJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"led_mode_color":{"mode":1,"direction":2,"color":5}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"leds", "set-mode-color-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after LED mode color JSON confirmation failure")
	}
}

func TestLEDSetModeColorRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"leds", "set-mode-color", "1", "2", "5"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after LED mode color confirmation failure")
	}
}

func TestLEDSetModeColorValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"leds", "set-mode-color", "1", "300", "5", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid LED mode color")
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXTableListIncludesSummary(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	vtx := view["vtx"].(map[string]any)
	if vtx["bands"] != float64(1) || vtx["channels"] != float64(8) || vtx["power_levels"] != float64(2) {
		t.Fatalf("vtx = %+v", vtx)
	}
	bands := vtx["band_rows"].([]any)
	first := bands[0].(map[string]any)
	frequencies := first["frequencies_mhz"].([]any)
	if first["name"] != "RACEBAND" || first["factory"] != true || frequencies[0] != float64(5658) {
		t.Fatalf("band = %+v", first)
	}
	powerValues := vtx["power_values"].([]any)
	if len(powerValues) != 2 || powerValues[1] != float64(200) {
		t.Fatalf("power values = %+v", powerValues)
	}
}

func TestVTXTableStatusReadsTypedMSPRows(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	table := data["vtxtable"].(map[string]any)
	if table["supported"] != true {
		t.Fatalf("supported = %+v", table["supported"])
	}
	summary := table["summary"].(map[string]any)
	if summary["available"] != true || summary["bands"] != float64(5) || summary["channels"] != float64(8) || summary["power_levels"] != float64(3) {
		t.Fatalf("summary = %+v", summary)
	}
	bands := table["bands"].([]any)
	if len(bands) != 5 {
		t.Fatalf("bands = %+v", bands)
	}
	firstBand := bands[0].(map[string]any)
	frequencies := firstBand["frequencies_mhz"].([]any)
	if firstBand["band"] != float64(1) || firstBand["name"] != "RACEBAND" || firstBand["letter"] != "R" || firstBand["factory"] != true || frequencies[0] != float64(5658) {
		t.Fatalf("first band = %+v", firstBand)
	}
	powers := table["powers"].([]any)
	if len(powers) != 3 {
		t.Fatalf("powers = %+v", powers)
	}
	secondPower := powers[1].(map[string]any)
	if secondPower["level"] != float64(2) || secondPower["value"] != float64(200) || secondPower["label"] != "200" {
		t.Fatalf("second power = %+v", secondPower)
	}
}

func TestVTXTableStatusReportsUnsupportedTarget(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "status"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		fc := fakefc.New()
		fc.Unsupported[msp.MSPVTXConfig] = true
		client, clientErr := connection.NewClient(fc, time.Second)
		return client, connection.TargetInfo{}, clientErr
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	table := data["vtxtable"].(map[string]any)
	if table["supported"] != false || table["unsupported_reason"] == "" {
		t.Fatalf("table = %+v", table)
	}
	if len(env.Warnings) != 1 || env.Warnings[0].Code != "unsupported_msp" {
		t.Fatalf("warnings = %+v", env.Warnings)
	}
}

func TestVTXTableSetBandWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-band", "1", "raceband", "r", "true", "5658", "5695", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtxtable_band"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["name"] != "RACEBAND" || config["letter"] != "R" || config["factory"] != true || result["save_required"] != true {
		t.Fatalf("vtxtable_band = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTXTABLE_BAND" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXTableSetBandJSONWithFakeFC(t *testing.T) {
	input := `{"vtxtable":{"band":{"band":1,"name":"raceband","letter":"r","factory":true,"frequencies_mhz":[5658,5695]}}}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-band-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtxtable_band"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["name"] != "RACEBAND" || config["letter"] != "R" || config["factory"] != true || result["save_required"] != true {
		t.Fatalf("vtxtable_band = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTXTABLE_BAND" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXTableSetPowerWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-power", "2", "200", "200", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtxtable_power"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["level"] != float64(2) || config["value"] != float64(200) || config["label"] != "200" || result["save_required"] != true {
		t.Fatalf("vtxtable_power = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTXTABLE_POWERLEVEL" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXTableSetPowerJSONWithFakeFC(t *testing.T) {
	input := `{"vtxtable":{"power":{"level":2,"value":200,"label":"200"}}}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-power-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtxtable_power"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["level"] != float64(2) || config["value"] != float64(200) || config["label"] != "200" || result["save_required"] != true {
		t.Fatalf("vtxtable_power = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTXTABLE_POWERLEVEL" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXTableSetJSONWithFakeFC(t *testing.T) {
	input := `{"vtxtable":{"bands":[{"band":1,"name":"raceband","letter":"r","factory":true,"frequencies_mhz":[5658,5695]}],"powers":[{"level":2,"value":200,"label":"200"}]}}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtxtable"].(map[string]any)
	if result["band_count"] != float64(1) || result["power_count"] != float64(1) || result["save_required"] != true {
		t.Fatalf("vtxtable = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTXTABLE_BAND,MSP_SET_VTXTABLE_POWERLEVEL" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXTableSetJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"bands":[{"band":1,"name":"raceband","letter":"r","factory":true,"frequencies_mhz":[5658]}]}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestVTXTableSetJSONValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-json", "-", "--yes"}, `{"powers":[{"level":1,"value":25,"label":"TOOLONG"}]}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXTableSetBandRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-band", "1", "raceband", "r", "true", "5658"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestVTXTableSetBandJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"band":1,"name":"raceband","letter":"r","factory":true,"frequencies_mhz":[5658]}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-band-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXTableSetBandJSONValidationBeforeConnect(t *testing.T) {
	input := `{"band":1,"name":"raceband","letter":"too-long","factory":true,"frequencies_mhz":[5658]}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-band-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXTableSetPowerValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtxtable", "set-power", "2", "200", "TOOLONG", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXTableSetPowerJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"level":2,"value":200,"label":"200"}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-power-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestVTXTableSetPowerJSONValidationBeforeConnect(t *testing.T) {
	input := `{"level":2,"value":200,"label":"TOOLONG"}`
	env, err := runTestCommandWithInput(t, []string{"vtxtable", "set-power-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestTableDomainSetPlansDoNotConnect(t *testing.T) {
	tests := []struct {
		args []string
		line string
	}{
		{[]string{"vtxtable", "set", "bands", "1"}, "vtxtable bands 1"},
		{[]string{"leds", "set", "0", "0,0::C:0"}, "led 0 0,0::C:0"},
		{[]string{"servos", "set", "0", "1000", "2000", "1500", "100", "none"}, "servo 0 1000 2000 1500 100 -1"},
		{[]string{"servos", "reverse", "0", "2", "r"}, "smix reverse 0 2 r"},
		{[]string{"adjustments", "set", "0", "0", "0", "900", "1300", "12", "0", "0", "0"}, "adjrange 0 0 0 900 1300 12 0 0 0"},
		{[]string{"rxrange", "set", "0", "1000", "2000"}, "rxrange 0 1000 2000"},
	}
	for _, tt := range tests {
		called := false
		env, err := runTestCommand(t, tt.args, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
			called = true
			return nil, connection.TargetInfo{}, nil
		})
		if err != nil {
			t.Fatalf("%v command error = %v", tt.args, err)
		}
		if !env.OK {
			t.Fatalf("%v env.OK = false: %+v", tt.args, env.Errors)
		}
		data := env.Data.(map[string]any)
		lines := data["cli_lines"].([]any)
		if lines[0] != tt.line || data["applied"] != false {
			t.Fatalf("%v plan = %+v", tt.args, data)
		}
		if called {
			t.Fatalf("%v connector was called for plan-only table command", tt.args)
		}
	}
}

func TestTableDomainSetValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"servos", "set", "0", "1000", "2000", "middle", "100", "none"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after table validation failure")
	}
}

func TestServoReverseJSONPlanDoesNotConnect(t *testing.T) {
	input := `{"servo_reverse":{"servo":0,"input_source":2,"reversed":true}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"servos", "reverse-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if len(lines) != 1 || lines[0] != "smix reverse 0 2 r" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only servo reverse-json command")
	}
}

func TestServoReverseJSONValidationBeforeConnect(t *testing.T) {
	input := `{"servo":0,"source":2,"mode":"sideways"}`
	env, err := runTestCommandWithInput(t, []string{"servos", "reverse-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid servo reverse JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestServoReverseJSONApplyWithFakeFC(t *testing.T) {
	input := `{"reverse_row":{"index":0,"source":2,"reverse":"n"}}`
	env, err := runTestCommandWithInput(t, []string{"servos", "reverse-json", "-", "--apply", "--yes"}, input, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "smix reverse 0 2 n" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestTableDomainSetApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rxrange", "set", "0", "1000", "2000", "--apply", "--yes"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "rxrange 0 1000 2000" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRXRangeSetJSONPlansDoNotConnect(t *testing.T) {
	input := `{"rxranges":[{"channel":0,"min":1000,"max":2000},{"channel":1,"min":1010,"max":1990}]}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"rxrange", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if data["applied"] != false || len(lines) != 2 || lines[0] != "rxrange 0 1000 2000" || lines[1] != "rxrange 1 1010 1990" {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for rxrange JSON plan")
	}
}

func TestRXRangeSetJSONApplyWithFakeFC(t *testing.T) {
	input := `{"range":{"channel":0,"min":1000,"max":2000}}`
	env, err := runTestCommandWithInput(t, []string{"rxrange", "set-json", "-", "--apply", "--yes"}, input, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "rxrange 0 1000 2000" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRXRangeSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"rxrange":{"channel":0,"min":2000,"max":1000}}`
	env, err := runTestCommandWithInput(t, []string{"rxrange", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid rxrange JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestResourcesSetJSONPlansDoNotConnect(t *testing.T) {
	input := `{"resources":[{"kind":"motor","index":1,"target":"a00"},{"kind":"serial_tx","index":2,"target":"b10"}]}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"resources", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if data["applied"] != false || len(lines) != 2 || lines[0] != "resource MOTOR 1 A00" || lines[1] != "resource SERIAL_TX 2 B10" {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for resources JSON plan")
	}
}

func TestResourcesSetJSONApplyWithFakeFC(t *testing.T) {
	input := `{"resource":{"kind":"motor","index":1,"target":"a00"}}`
	env, err := runTestCommandWithInput(t, []string{"resources", "set-json", "-", "--apply", "--yes"}, input, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "resource MOTOR 1 A00" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestResourcesSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"resource":{"kind":"motor output","index":1,"target":"a00"}}`
	env, err := runTestCommandWithInput(t, []string{"resources", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid resources JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestResourcesSetJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"resource":{"kind":"motor","index":1,"target":"a00"}}`
	env, err := runTestCommandWithInput(t, []string{"resources", "set-json", "-", "--apply"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestBatchPlanAllowsTableRows(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"batch", "plan"}, "led 0 0,0::C:0\nservo 0 1000 2000 1500 100 -1\nadjrange 0 0 0 900 1300 12 0 0 0\nrxrange 0 1000 2000\nrxfail 2 s 1100\nbattery_profile 1\n", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if len(lines) != 6 {
		t.Fatalf("plan = %+v", data)
	}
}
