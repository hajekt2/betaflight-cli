package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestBeeperConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	beeper := data["beeper"].(map[string]any)
	if beeper["dshot_beacon_tone"] != float64(3) || beeper["disabled_mask"] != float64(0x12) {
		t.Fatalf("beeper = %+v", beeper)
	}
	disabled := beeper["disabled"].([]any)
	if len(disabled) != 2 || disabled[0] != "RX_LOST" || disabled[1] != "ARMING" {
		t.Fatalf("disabled = %+v", disabled)
	}
}

func TestBeeperSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "set-config", "0x12", "3", "0x202", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["beeper_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["disabled_mask"] != float64(0x12) || config["dshot_beacon_disabled_mask"] != float64(0x202) || result["save_required"] != true {
		t.Fatalf("beeper_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_BEEPER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBeeperSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"beeper":{"disabled_mask":18,"dshot_beacon_tone":3,"dshot_beacon_disabled_mask":514}}`
	env, err := runTestCommandWithInput(t, []string{"beeper", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["beeper_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["disabled_mask"] != float64(0x12) || config["dshot_beacon_disabled_mask"] != float64(0x202) || result["save_required"] != true {
		t.Fatalf("beeper_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "beeper_config" || env.SideEffects[0].Command != "MSP_SET_BEEPER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBeeperSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "set-config", "0x12", "3", "0x202"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestBeeperSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"beeper_config":{"disabled_mask":18,"dshot_beacon_tone":3,"dshot_beacon_disabled_mask":514}}`
	env, err := runTestCommandWithInput(t, []string{"beeper", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestBeeperSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "set-config", "0x12", "300", "0x202", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestBeeperEnablePlanOnly(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "enable", "ARMING"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	if plan["kind"] != "beeper" || plan["applied"] != false {
		t.Fatalf("plan = %+v", plan)
	}
	lines := plan["cli_lines"].([]any)
	if len(lines) != 1 || lines[0] != "beeper ARMING" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestBeeperEnableRejectsUnknownMode(t *testing.T) {
	env, err := runTestCommand(t, []string{"beeper", "enable", "NOT_A_MODE"}, nil)
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK {
		t.Fatalf("expected failure: %+v", env)
	}
}

func TestBeeperSetJSONPlanDoesNotConnect(t *testing.T) {
	input := `{"beeper":{"enable":["ARMING"],"disable":"BAT_LOW"}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"beeper", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	lines := plan["cli_lines"].([]any)
	if plan["kind"] != "beeper" || plan["applied"] != false || len(lines) != 2 || lines[0] != "beeper ARMING" || lines[1] != "beeper -BAT_LOW" {
		t.Fatalf("plan = %+v", plan)
	}
	if called {
		t.Fatal("connector was called for plan-only beeper set-json command")
	}
}

func TestBeeperSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"enabled":["NOT_A_MODE"]}`
	env, err := runTestCommandWithInput(t, []string{"beeper", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid beeper JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestBeeperSetJSONApply(t *testing.T) {
	input := `{"set":{"disable":["ARMING"]}}`
	var seenOp connection.OperationClass
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOp = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(context.Background())
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	}
	env, err := runTestCommandWithInput(t, []string{"beeper", "set-json", "-", "--apply", "--yes"}, input, connect)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if seenOp != connection.Write {
		t.Fatalf("operation = %v, want Write", seenOp)
	}
	plan := env.Data.(map[string]any)
	if plan["applied"] != true || plan["kind"] != "beeper" {
		t.Fatalf("plan = %+v", plan)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "beeper -ARMING" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestBeeperDisableApply(t *testing.T) {
	var seenOp connection.OperationClass
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOp = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(context.Background())
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	}
	env, err := runTestCommandWithInput(t, []string{"beeper", "disable", "ARMING", "--apply", "--yes"}, "", connect)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if seenOp != connection.Write {
		t.Fatalf("operation = %v, want Write", seenOp)
	}
	plan := env.Data.(map[string]any)
	if plan["applied"] != true || plan["kind"] != "beeper" {
		t.Fatalf("plan = %+v", plan)
	}
	if _, ok := plan["response_lines"]; !ok {
		t.Fatalf("missing response_lines: %+v", plan)
	}
}

func TestTransponderConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	transponder := data["transponder"].(map[string]any)
	if transponder["source"] != "MSP_TRANSPONDER_CONFIG" || transponder["provider_name"] != "ARCITIMER" || transponder["data_hex"] != "123456789ABCDEF042" {
		t.Fatalf("transponder = %+v", transponder)
	}
	providers := transponder["providers"].([]any)
	if len(providers) != 3 || providers[1].(map[string]any)["data_length"] != float64(9) {
		t.Fatalf("providers = %+v", providers)
	}
	commands := transponder["cli_commands"].([]any)
	if len(commands) != 2 || commands[0] != "set transponder_provider = ARCITIMER" {
		t.Fatalf("cli_commands = %+v", commands)
	}
}

func TestTransponderSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-config", "2", "18,52,86,120,154,188,222,240,66", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["transponder_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["provider"] != float64(2) || config["provider_name"] != "ARCITIMER" || config["data_hex"] != "123456789ABCDEF042" || result["save_required"] != true {
		t.Fatalf("transponder_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_TRANSPONDER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestTransponderSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"transponder":{"provider":2,"data_hex":"123456789ABCDEF042"}}`
	env, err := runTestCommandWithInput(t, []string{"transponder", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["transponder_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["provider"] != float64(2) || config["provider_name"] != "ARCITIMER" || config["data_hex"] != "123456789ABCDEF042" || result["save_required"] != true {
		t.Fatalf("transponder_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "transponder_config" || env.SideEffects[0].Command != "MSP_SET_TRANSPONDER_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestTransponderSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-config", "2", "18,52,86"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestTransponderSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"transponder_config":{"provider":2,"data":[18,52,86]}}`
	env, err := runTestCommandWithInput(t, []string{"transponder", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestTransponderSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-config", "0", "18", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestTransponderSetProviderPlan(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-provider", "ARCITIMER"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	if plan["kind"] != "transponder" || plan["applied"] != false {
		t.Fatalf("plan = %+v", plan)
	}
	lines := plan["cli_lines"].([]any)
	if len(lines) != 1 || lines[0] != "set transponder_provider = ARCITIMER" {
		t.Fatalf("lines = %+v", lines)
	}
}

func TestTransponderSetProviderRejectsUnknown(t *testing.T) {
	env, err := runTestCommand(t, []string{"transponder", "set-provider", "UNKNOWN"}, nil)
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK {
		t.Fatalf("expected failure: %+v", env)
	}
}

func TestTransponderSetJSONPlanDoesNotConnect(t *testing.T) {
	input := `{"transponder":{"provider":"ARCITIMER","data":[1,2,3]}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"transponder", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)
	lines := plan["cli_lines"].([]any)
	if plan["kind"] != "transponder" || plan["applied"] != false || len(lines) != 2 || lines[0] != "set transponder_provider = ARCITIMER" || lines[1] != "set transponder_data = 1,2,3" {
		t.Fatalf("plan = %+v", plan)
	}
	if called {
		t.Fatal("connector was called for plan-only transponder set-json command")
	}
}

func TestTransponderSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"provider":"UNKNOWN","data":[1,2,3]}`
	env, err := runTestCommandWithInput(t, []string{"transponder", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid transponder JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestTransponderSetJSONApply(t *testing.T) {
	input := `{"set":{"name":"ARCITIMER","data_hex":"010203"}}`
	var seenOp connection.OperationClass
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOp = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(context.Background())
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	}
	env, err := runTestCommandWithInput(t, []string{"transponder", "set-json", "-", "--apply", "--yes"}, input, connect)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if seenOp != connection.Write {
		t.Fatalf("operation = %v, want Write", seenOp)
	}
	plan := env.Data.(map[string]any)
	if plan["applied"] != true || plan["kind"] != "transponder" {
		t.Fatalf("plan = %+v", plan)
	}
	if len(env.SideEffects) != 2 || env.SideEffects[0].Command != "set transponder_provider = ARCITIMER" || env.SideEffects[1].Command != "set transponder_data = 1,2,3" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestTransponderSetDataApply(t *testing.T) {
	var seenOp connection.OperationClass
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOp = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(context.Background())
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	}
	env, err := runTestCommandWithInput(t, []string{"transponder", "set-data", "1,2,3", "--apply", "--yes"}, "", connect)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if seenOp != connection.Write {
		t.Fatalf("operation = %v, want Write", seenOp)
	}
	plan := env.Data.(map[string]any)
	if plan["applied"] != true || plan["kind"] != "transponder" {
		t.Fatalf("plan = %+v", plan)
	}
	if plan["command_preview"] != "set transponder_data = 1,2,3" {
		t.Fatalf("plan = %+v", plan)
	}
}
