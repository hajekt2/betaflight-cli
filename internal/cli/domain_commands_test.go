package cli

import (
	"context"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestResourcesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"resources", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	resources := data["resources"].(map[string]any)
	assignments := resources["resources"].([]any)
	if len(assignments) != 2 {
		t.Fatalf("resources = %+v", assignments)
	}
	first := assignments[0].(map[string]any)
	if first["kind"] != "MOTOR" || first["index"] != "1" || first["target"] != "A00" {
		t.Fatalf("first resource = %+v", first)
	}
	timers := resources["timers"].([]any)
	if len(timers) != 2 {
		t.Fatalf("timers = %+v", timers)
	}
	timer := timers[1].(map[string]any)
	if timer["pin"] != "A09" || timer["none"] != true {
		t.Fatalf("none timer = %+v", timer)
	}
	dma := resources["dma"].([]any)
	if len(dma) != 2 {
		t.Fatalf("dma = %+v", dma)
	}
	deviceDMA := dma[1].(map[string]any)
	if deviceDMA["scope"] != "SPI_TX" || deviceDMA["device"] != "1" || deviceDMA["option"] != "0" {
		t.Fatalf("device dma = %+v", deviceDMA)
	}
}

func TestVTXSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "set-config", "5", "8", "2", "true", "5861", "2", "5662", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtx_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["band"] != float64(5) || config["pit_mode"] != true || config["frequency_mhz"] != float64(5861) || result["save_required"] != true {
		t.Fatalf("vtx_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_VTX_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"vtx_config":{"band":5,"channel":8,"power":2,"pit_mode":true,"frequency_mhz":5861,"low_power_disarm":2,"pit_mode_frequency_mhz":5662}}`
	env, err := runTestCommandWithInput(t, []string{"vtx", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["vtx_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["band"] != float64(5) || config["pit_mode"] != true || config["frequency_mhz"] != float64(5861) || result["save_required"] != true {
		t.Fatalf("vtx_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "vtx_config" || env.SideEffects[0].Command != "MSP_SET_VTX_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestVTXSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "set-config", "5", "8", "2", "true", "5861", "2", "5662"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestVTXSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"config":{"band":5,"channel":8,"power":2,"pit_mode":true,"frequency_mhz":5861,"low_power_disarm":2,"pit_mode_frequency_mhz":5662}}`
	env, err := runTestCommandWithInput(t, []string{"vtx", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestVTXSetConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "set-config", "5", "8", "2", "maybe", "5861", "2", "5662", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestModesActiveWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "active"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	modes := data["modes"].(map[string]any)
	definitions := modes["definitions"].([]any)
	if len(definitions) != 34 {
		t.Fatalf("definitions = %+v", definitions)
	}
	last := definitions[33].(map[string]any)
	if last["id"] != float64(53) || last["name"] != "READY" {
		t.Fatalf("last definition = %+v", last)
	}
	ranges := modes["ranges"].([]any)
	if len(ranges) != 3 {
		t.Fatalf("ranges = %+v", ranges)
	}
	arm := ranges[0].(map[string]any)
	if arm["name"] != "ARM" || arm["aux_channel_name"] != "AUX1" || arm["active"] != true {
		t.Fatalf("arm range = %+v", arm)
	}
	beeperMute := ranges[1].(map[string]any)
	if beeperMute["name"] != "BEEPER MUTE" || beeperMute["mode_logic_name"] != "AND" || beeperMute["linked_to_name"] != "READY" {
		t.Fatalf("beeper mute range = %+v", beeperMute)
	}
}

func TestModesSetRangeWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "set-range", "1", "52", "2", "16", "32", "1", "53", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["mode_range"].(map[string]any)
	row := result["range"].(map[string]any)
	rangeData := row["range"].(map[string]any)
	if row["index"] != float64(1) || row["mode_logic_name"] != "AND" || row["linked_to"] != float64(53) || rangeData["start_us"] != float64(1300) || result["save_required"] != true {
		t.Fatalf("mode_range = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_MODE_RANGE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestModesSetJSONWithFakeFC(t *testing.T) {
	input := `{"mode_ranges":{"ranges":[{"index":1,"id":52,"aux_channel_index":2,"range":{"start_step":16,"end_step":32},"mode_logic":1,"linked_to":53}]}}`
	env, err := runTestCommandWithInput(t, []string{"modes", "set-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["mode_ranges"].(map[string]any)
	ranges := result["ranges"].([]any)
	row := ranges[0].(map[string]any)
	rangeData := row["range"].(map[string]any)
	if result["range_count"] != float64(1) || row["mode_logic_name"] != "AND" || rangeData["start_us"] != float64(1300) || result["save_required"] != true {
		t.Fatalf("mode_ranges = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_MODE_RANGE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestModesSetJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[{"index":1,"id":52,"aux_channel_index":2,"range":{"start_step":16,"end_step":32}}]`
	env, err := runTestCommandWithInput(t, []string{"modes", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestModesSetJSONValidationBeforeConnect(t *testing.T) {
	input := `[{"index":1,"id":52,"aux_channel_index":2,"range":{"start_step":32,"end_step":16}}]`
	env, err := runTestCommandWithInput(t, []string{"modes", "set-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestModesSetRangeRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "set-range", "1", "52", "2", "16", "32"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestModesSetRangeValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"modes", "set-range", "1", "52", "2", "16", "300", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	receiver := data["receiver"].(map[string]any)
	config := receiver["config"].(map[string]any)
	if config["serial_provider"] != float64(2) || config["stick_center"] != float64(1500) || config["rx_min_usec"] != float64(885) {
		t.Fatalf("config = %+v", config)
	}
	if receiver["rssi_channel"] != float64(8) {
		t.Fatalf("receiver = %+v", receiver)
	}
	txInfo := receiver["tx_info"].(map[string]any)
	if txInfo["rssi_source_name"] != "RX_PROTOCOL_CRSF" || txInfo["rtc_status_name"] != "SET" || txInfo["rtc_is_set"] != true {
		t.Fatalf("tx info = %+v", txInfo)
	}
	deadband := receiver["deadband"].(map[string]any)
	if deadband["deadband"] != float64(5) || deadband["yaw_deadband"] != float64(7) || deadband["deadband_3d_throttle"] != float64(50) {
		t.Fatalf("deadband = %+v", deadband)
	}
	channels := receiver["channels"].([]any)
	if len(channels) != 6 || channels[3] != float64(1000) {
		t.Fatalf("channels = %+v", channels)
	}
	rcMapNames := receiver["rc_map_names"].([]any)
	if len(rcMapNames) != 4 || rcMapNames[2] != "THROTTLE" || rcMapNames[3] != "YAW" {
		t.Fatalf("rc map names = %+v", rcMapNames)
	}
	failsafe := receiver["failsafe"].([]any)
	if len(failsafe) != 4 {
		t.Fatalf("failsafe = %+v", failsafe)
	}
	row := failsafe[2].(map[string]any)
	if row["mode_name"] != "SET" || row["value"] != float64(1100) || row["cli_command"] != "rxfail 2 s 1100" {
		t.Fatalf("failsafe row = %+v", row)
	}
}

func TestReceiverSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"receiver_config":{"serial_provider":2,"stick_max":2000,"stick_center":1500,"stick_min":1000,"spektrum_satellite_bind":0,"rx_min_usec":885,"rx_max_usec":2115,"airmode_activate_threshold":1350,"rx_spi_protocol":0,"rx_spi_id":0,"rx_spi_rf_channel_count":0,"fpv_cam_angle_degrees":10,"rc_smoothing_setpoint_cutoff":50,"rc_smoothing_throttle_cutoff":60,"rc_smoothing_auto_factor_throttle":70,"usb_cdc_hid_type":0,"rc_smoothing_auto_factor":80,"rc_smoothing":1,"elrs_uid":[1,2,3,4,5,6],"elrs_model_id":7}}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["receiver_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["serial_provider"] != float64(2) || config["stick_center"] != float64(1500) || result["save_required"] != true {
		t.Fatalf("receiver_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "receiver_config" || env.SideEffects[0].Command != "MSP_SET_RX_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetConfigJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"receiver_config":{"serial_provider":2,"stick_max":2000,"stick_center":1500,"stick_min":1000}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after receiver config confirmation failure")
	}
}

func TestReceiverSetRSSIChannelRejectsOutOfRangeDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-rssi-channel", "19", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after RSSI channel validation failure")
	}
}

func TestReceiverSetRSSIChannelRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-rssi-channel", "8"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after RSSI channel confirmation failure")
	}
}

func TestReceiverSetRSSIChannelJSONValidationBeforeConnect(t *testing.T) {
	input := `{"rssi_channel":19}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-rssi-channel-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetRSSIChannelJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"receiver":{"rssi_channel":8}}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-rssi-channel-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetMapWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "3", "2", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rc_map"].(map[string]any)
	names := result["names"].([]any)
	if result["save_required"] != true || len(names) != 4 || names[2] != "THROTTLE" || names[3] != "YAW" {
		t.Fatalf("rc_map = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RX_MAP" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetMapJSONWithFakeFC(t *testing.T) {
	input := `{"receiver":{"rc_map":[0,1,3,2]}}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-map-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rc_map"].(map[string]any)
	names := result["names"].([]any)
	if result["save_required"] != true || len(names) != 4 || names[2] != "THROTTLE" || names[3] != "YAW" {
		t.Fatalf("rc_map = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RX_MAP" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetMapRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "3", "2"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetMapJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[0,1,3,2]`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-map-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetMapValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "3", "300", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetMapJSONValidationBeforeConnect(t *testing.T) {
	input := `{"rc_map":[0,1,1,2]}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-map-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestReceiverSetMapRejectsDuplicateValuesBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-map", "0", "1", "1", "2", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetRSSIChannelWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rssi-channel", "8", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rssi_channel"].(map[string]any)
	if result["channel"] != float64(8) || result["msp_name"] != "MSP_SET_RSSI_CONFIG" || result["save_required"] != true {
		t.Fatalf("rssi channel = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "rssi_channel" || env.SideEffects[0].Command != "MSP_SET_RSSI_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetRSSIChannelJSONWithFakeFC(t *testing.T) {
	input := `{"receiver":{"rssi_channel":8}}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-rssi-channel-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rssi_channel"].(map[string]any)
	if result["channel"] != float64(8) || result["msp_name"] != "MSP_SET_RSSI_CONFIG" || result["save_required"] != true {
		t.Fatalf("rssi channel = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "rssi_channel" || env.SideEffects[0].Command != "MSP_SET_RSSI_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetDeadbandRejectsInvalidValueDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-deadband", "5", "300", "3", "50", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after RC deadband validation failure")
	}
}

func TestReceiverSetDeadbandRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "set-deadband", "5", "7", "3", "50"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after RC deadband confirmation failure")
	}
}

func TestReceiverSetDeadbandJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"deadband":5,"yaw_deadband":7,"pos_hold_deadband":3,"deadband_3d_throttle":50}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-deadband-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after RC deadband confirmation failure")
	}
}

func TestReceiverSetDeadbandWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-deadband", "5", "7", "3", "50", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rc_deadband"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["deadband"] != float64(5) || config["yaw_deadband"] != float64(7) || config["deadband_3d_throttle"] != float64(50) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_RC_DEADBAND" || result["save_required"] != true {
		t.Fatalf("rc deadband result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "rc_deadband" || env.SideEffects[0].Command != "MSP_SET_RC_DEADBAND" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetDeadbandJSONWithFakeFC(t *testing.T) {
	input := `{"receiver":{"deadband":{"deadband":5,"yaw_deadband":7,"pos_hold_deadband":3,"deadband_3d_throttle":50}}}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-deadband-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rc_deadband"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["deadband"] != float64(5) || config["yaw_deadband"] != float64(7) || config["deadband_3d_throttle"] != float64(50) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_RC_DEADBAND" || result["save_required"] != true {
		t.Fatalf("rc deadband result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "rc_deadband" || env.SideEffects[0].Command != "MSP_SET_RC_DEADBAND" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	gps := data["gps"].(map[string]any)
	config := gps["config"].(map[string]any)
	if config["provider"] != float64(1) || config["auto_config"] != true || config["ublox_use_galileo"] != true {
		t.Fatalf("config = %+v", config)
	}
	position := gps["position"].(map[string]any)
	if position["fix"] != true || position["satellites"] != float64(12) || position["latitude_degrees"] != 59.9123456 {
		t.Fatalf("position = %+v", position)
	}
	rescue := gps["rescue"].(map[string]any)
	if rescue["min_sats"] != float64(8) || rescue["initial_climb_m"] != float64(20) {
		t.Fatalf("rescue = %+v", rescue)
	}
	satellites := gps["satellites"].([]any)
	if len(satellites) != 2 {
		t.Fatalf("satellites = %+v", satellites)
	}
}

func TestGPSSetConfigRejectsInvalidBoolBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-config", "1", "0", "2", "1", "1", "1", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestGPSSetConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-config", "1", "0", "1", "1", "1", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestGPSSetConfigJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"gps":{"config":{"provider":1,"sbas_mode":0,"auto_config":true,"auto_baud":true,"home_point_once":true,"ublox_use_galileo":true}}}`
	env, err := runTestCommandWithInput(t, []string{"gps", "set-config-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestGPSSetConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-config", "1", "0", "1", "1", "1", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["provider"] != float64(1) || config["auto_config"] != true || config["ublox_use_galileo"] != true {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_CONFIG" || result["save_required"] != true {
		t.Fatalf("gps config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_config" || env.SideEffects[0].Command != "MSP_SET_GPS_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSSetConfigJSONWithFakeFC(t *testing.T) {
	input := `{"gps":{"config":{"provider":1,"sbas_mode":0,"auto_config":true,"auto_baud":true,"home_point_once":true,"ublox_use_galileo":true}}}`
	env, err := runTestCommandWithInput(t, []string{"gps", "set-config-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_config"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["provider"] != float64(1) || config["auto_config"] != true || config["ublox_use_galileo"] != true {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_CONFIG" || result["save_required"] != true {
		t.Fatalf("gps config result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_config" || env.SideEffects[0].Command != "MSP_SET_GPS_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSSetRescueRejectsInvalidValueBeforeConnect(t *testing.T) {
	args := []string{"gps", "set-rescue", "3200", "100", "50", "1500", "1200", "1800", "1450", "1", "8", "500", "150", "2", "2", "30", "20", "--yes"}
	env, err := runTestCommand(t, args, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestGPSSetRescueRequiresConfirmationBeforeConnect(t *testing.T) {
	args := []string{"gps", "set-rescue", "3200", "100", "50", "1500", "1200", "1800", "1450", "1", "8", "500", "150", "1", "2", "30", "20"}
	env, err := runTestCommand(t, args, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestGPSSetRescueJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"gps":{"rescue":{"max_rescue_angle":3200,"return_altitude_m":100,"descent_distance_m":50,"ground_speed_cm_s":1500,"throttle_min":1200,"throttle_max":1800,"throttle_hover":1450,"sanity_checks":1,"min_sats":8,"ascend_rate":500,"descend_rate":150,"allow_arming_without_fix":true,"altitude_mode":2,"min_start_distance_m":30,"initial_climb_m":20}}}`
	env, err := runTestCommandWithInput(t, []string{"gps", "set-rescue-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestGPSSetRescueWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-rescue", "3200", "100", "50", "1500", "1200", "1800", "1450", "1", "8", "500", "150", "1", "2", "30", "20", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_rescue"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["max_rescue_angle"] != float64(3200) || config["min_sats"] != float64(8) || config["initial_climb_m"] != float64(20) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_RESCUE" || result["save_required"] != true {
		t.Fatalf("gps rescue result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_rescue" || env.SideEffects[0].Command != "MSP_SET_GPS_RESCUE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSSetRescueJSONWithFakeFC(t *testing.T) {
	input := `{"gps":{"rescue":{"max_rescue_angle":3200,"return_altitude_m":100,"descent_distance_m":50,"ground_speed_cm_s":1500,"throttle_min":1200,"throttle_max":1800,"throttle_hover":1450,"sanity_checks":1,"min_sats":8,"ascend_rate":500,"descend_rate":150,"allow_arming_without_fix":true,"altitude_mode":2,"min_start_distance_m":30,"initial_climb_m":20}}}`
	env, err := runTestCommandWithInput(t, []string{"gps", "set-rescue-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_rescue"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["max_rescue_angle"] != float64(3200) || config["min_sats"] != float64(8) || config["initial_climb_m"] != float64(20) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_RESCUE" || result["save_required"] != true {
		t.Fatalf("gps rescue result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_rescue" || env.SideEffects[0].Command != "MSP_SET_GPS_RESCUE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSSetRescuePIDWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"gps", "set-rescue-pids", "80", "10", "5", "120", "20", "10", "45", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_rescue_pids"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["altitude_p"] != float64(80) || config["velocity_p"] != float64(120) || config["yaw_p"] != float64(45) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_RESCUE_PIDS" || result["save_required"] != true {
		t.Fatalf("gps rescue pid result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_rescue_pids" || env.SideEffects[0].Command != "MSP_SET_GPS_RESCUE_PIDS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestGPSSetRescuePIDJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"gps":{"rescue_pid":{"altitude_p":80,"altitude_i":10,"altitude_d":5,"velocity_p":120,"velocity_i":20,"velocity_d":10,"yaw_p":45}}}`
	env, err := runTestCommandWithInput(t, []string{"gps", "set-rescue-pids-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestGPSSetRescuePIDJSONWithFakeFC(t *testing.T) {
	input := `{"gps":{"rescue_pid":{"altitude_p":80,"altitude_i":10,"altitude_d":5,"velocity_p":120,"velocity_i":20,"velocity_d":10,"yaw_p":45}}}`
	env, err := runTestCommandWithInput(t, []string{"gps", "set-rescue-pids-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["gps_rescue_pids"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["altitude_p"] != float64(80) || config["velocity_p"] != float64(120) || config["yaw_p"] != float64(45) {
		t.Fatalf("config = %+v", config)
	}
	if result["msp_name"] != "MSP_SET_GPS_RESCUE_PIDS" || result["save_required"] != true {
		t.Fatalf("gps rescue pid result = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "gps_rescue_pids" || env.SideEffects[0].Command != "MSP_SET_GPS_RESCUE_PIDS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	osd := data["osd"].(map[string]any)
	config := osd["config"].(map[string]any)
	if config["video_system_name"] != "HD" || config["units_name"] != "METRIC" {
		t.Fatalf("config = %+v", config)
	}
	flags := config["flags"].(map[string]any)
	if flags["feature_enabled"] != true || flags["hardware_max7456"] != true || flags["device_detected"] != true {
		t.Fatalf("flags = %+v", flags)
	}
	canvas := osd["canvas"].(map[string]any)
	if canvas["columns"] != float64(53) || canvas["rows"] != float64(20) {
		t.Fatalf("canvas = %+v", canvas)
	}
	warnings := osd["warnings"].(map[string]any)
	if warnings["text"] != "LOW BATTERY" {
		t.Fatalf("warnings = %+v", warnings)
	}
}

func TestOSDSetCanvasWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-canvas", "53", "20", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_canvas"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["columns"] != float64(53) || config["rows"] != float64(20) || result["save_required"] != true || result["reboot_possible"] != true {
		t.Fatalf("osd_canvas = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_OSD_CANVAS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetCanvasRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-canvas", "53", "20"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetCanvasValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-canvas", "300", "20", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetGeneralJSONWithFakeFC(t *testing.T) {
	input := `{"osd_general_config":{"video_system":3,"units":1,"alarms":{"rssi":25,"capacity_mah":1600,"altitude_m":130,"link_quality":80,"rssi_dbm":-90},"enabled_warnings":7,"selected_profile":2,"stick_overlay_mode":1,"camera_frame_width":24,"camera_frame_height":18}}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-general-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_general_config"].(map[string]any)
	config := result["config"].(map[string]any)
	alarms := config["alarms"].(map[string]any)
	if config["video_system"] != float64(3) || config["units"] != float64(1) || alarms["capacity_mah"] != float64(1600) || result["save_required"] != true {
		t.Fatalf("osd_general_config = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetGeneralJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"osd", "set-general-json", "-"}, `{"units":1}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetGeneralJSONValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"osd", "set-general-json", "-", "--yes"}, `{"video_system":4}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetVideoSystemWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-video-system", "3", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_video_system"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["video_system"] != float64(3) || config["video_system_name"] != "HD" || result["save_required"] != true || result["reboot_possible"] != true {
		t.Fatalf("osd_video_system = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetVideoSystemJSONWithFakeFC(t *testing.T) {
	input := `{"osd":{"video_system":3}}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-video-system-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_video_system"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["video_system"] != float64(3) || config["video_system_name"] != "HD" || result["save_required"] != true || result["reboot_possible"] != true {
		t.Fatalf("osd_video_system = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetVideoSystemRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-video-system", "3"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetVideoSystemJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"video_system":3}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-video-system-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetVideoSystemValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-video-system", "4", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetVideoSystemJSONValidationBeforeConnect(t *testing.T) {
	input := `{"video_system":4}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-video-system-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetPositionWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-position", "7", "2122", "1", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_position"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(7) || config["raw"] != float64(2122) || config["screen"] != float64(1) || result["save_required"] != true {
		t.Fatalf("osd_position = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetPositionJSONWithFakeFC(t *testing.T) {
	input := `{"osd_position":{"index":7,"raw":2122,"screen":1}}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-position-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_position"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(7) || config["raw"] != float64(2122) || config["screen"] != float64(1) || result["save_required"] != true {
		t.Fatalf("osd_position = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "osd_position" || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetStatWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-stat", "3", "true", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_stat"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(3) || config["enabled"] != true || result["save_required"] != true {
		t.Fatalf("osd_stat = %+v", result)
	}
}

func TestOSDSetStatJSONWithFakeFC(t *testing.T) {
	input := `{"stat":{"index":3,"enabled":true}}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-stat-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_stat"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(3) || config["enabled"] != true || result["save_required"] != true {
		t.Fatalf("osd_stat = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "osd_stat" || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetTimerWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-timer", "1", "1110", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_timer"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(1) || config["value"] != float64(1110) || result["save_required"] != true {
		t.Fatalf("osd_timer = %+v", result)
	}
}

func TestOSDSetTimerJSONWithFakeFC(t *testing.T) {
	input := `{"config":{"index":1,"value":1110}}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-timer-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["osd_timer"].(map[string]any)
	config := result["config"].(map[string]any)
	if config["index"] != float64(1) || config["value"] != float64(1110) || result["save_required"] != true {
		t.Fatalf("osd_timer = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "osd_timer" || env.SideEffects[0].Command != "MSP_SET_OSD_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestOSDSetPositionRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-position", "7", "2122"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetPositionJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `{"position":{"index":7,"raw":2122,"screen":1}}`
	env, err := runTestCommandWithInput(t, []string{"osd", "set-position-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestOSDSetStatValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "set-stat", "3", "maybe", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestFeaturesListWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	features := view["features"].([]any)
	if len(features) != 1 {
		t.Fatalf("features = %+v", features)
	}
}

func TestFeaturesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	features := data["features"].(map[string]any)
	if features["source"] != "MSP_FEATURE_CONFIG" || features["mask"] != float64(0x00040488) {
		t.Fatalf("features = %+v", features)
	}
	enabled := features["enabled_names"].([]any)
	want := []string{"RX_SERIAL", "GPS", "TELEMETRY", "OSD"}
	if len(enabled) != len(want) {
		t.Fatalf("enabled = %+v", enabled)
	}
	for i, name := range want {
		if enabled[i] != name {
			t.Fatalf("enabled = %+v, want %v", enabled, want)
		}
	}
	catalog := features["catalog"].([]any)
	if len(catalog) != 24 {
		t.Fatalf("catalog length = %d", len(catalog))
	}
}

func TestFeaturesSetMaskWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "set-mask", "0x00040488", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["feature_mask"].(map[string]any)
	features := result["features"].(map[string]any)
	if features["mask"] != float64(0x00040488) || result["save_required"] != true {
		t.Fatalf("feature_mask = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_FEATURE_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFeaturesSetMaskRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "set-mask", "0x00040488"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestFeaturesSetMaskValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "set-mask", "not-a-mask", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestFeatureEnablePlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"features", "enable", "gps"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if lines[0] != "feature GPS" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only feature command")
	}
}

func TestFeaturesSetJSONPlanDoesNotConnect(t *testing.T) {
	input := `{"features":{"enable":["gps","osd"],"disable":["airmode"]}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"features", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if len(lines) != 3 || lines[0] != "feature GPS" || lines[1] != "feature OSD" || lines[2] != "feature -AIRMODE" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only feature JSON command")
	}
}

func TestFeaturesSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"enable":["gps"],"disable":["GPS"]}`
	env, err := runTestCommandWithInput(t, []string{"features", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for conflicting feature JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFeaturesSetJSONApplyWithFakeFC(t *testing.T) {
	input := `{"enable":["gps"],"disable":["gps"]}`
	env, err := runTestCommandWithInput(t, []string{"features", "set-json", "-", "--apply", "--yes"}, input, nil)
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}

	input = `{"enable":["gps"],"disable":["airmode"]}`
	env, err = runTestCommandWithInput(t, []string{"features", "set-json", "-", "--apply", "--yes"}, input, nil)
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
	lines := data["cli_lines"].([]any)
	if len(lines) != 2 || lines[0] != "feature GPS" || lines[1] != "feature -AIRMODE" {
		t.Fatalf("lines = %+v", lines)
	}
	if len(env.SideEffects) != 2 || env.SideEffects[0].Command != "feature GPS" || env.SideEffects[1].Command != "feature -AIRMODE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFeaturesSetJSONReportsPartialSideEffects(t *testing.T) {
	input := `{"enable":["gps"],"disable":["airmode"]}`
	env, err := runTestCommandWithInput(t, []string{"features", "set-json", "-", "--apply", "--yes"}, input, failingCLIConnector(2))
	if err == nil || env.OK {
		t.Fatalf("expected failed partial apply: env=%+v err=%v", env, err)
	}
	data := env.Data.(map[string]any)
	if data["partially_applied"] != true || data["failed_cli_line"] != "feature -AIRMODE" {
		t.Fatalf("partial apply data = %+v", data)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "feature GPS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverRXFailPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "rxfail", "2", "s", "1100"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if lines[0] != "rxfail 2 s 1100" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only rxfail command")
	}
}

func TestReceiverRXFailValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"receiver", "rxfail", "4", "a"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after rxfail validation failure")
	}
}

func TestReceiverRXFailApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "rxfail", "2", "s", "1100", "--apply", "--yes"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "rxfail 2 s 1100" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetRXFailWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rxfail", "2", "2", "1100", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rx_fail"].(map[string]any)
	channel := result["channel"].(map[string]any)
	if channel["index"] != float64(2) || channel["mode_name"] != "SET" || channel["value"] != float64(1100) || result["save_required"] != true {
		t.Fatalf("rx_fail = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RXFAIL_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetRXFailJSONWithFakeFC(t *testing.T) {
	input := `{"rx_fail_table":{"channels":[{"index":2,"mode":2,"value":1100},{"index":4,"mode":1,"value":1500}]}}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-rxfail-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["rx_fail_table"].(map[string]any)
	channels := result["channels"].([]any)
	second := channels[1].(map[string]any)
	if result["channel_count"] != float64(2) || second["index"] != float64(4) || second["mode_name"] != "HOLD" || result["save_required"] != true {
		t.Fatalf("rx_fail_table = %+v", result)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_SET_RXFAIL_CONFIG" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestReceiverSetRXFailRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rxfail", "2", "2", "1100"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetRXFailJSONRequiresConfirmationBeforeConnect(t *testing.T) {
	input := `[{"index":2,"mode":2,"value":1100}]`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-rxfail-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetRXFailValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommand(t, []string{"receiver", "set-rxfail", "4", "0", "1000", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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

func TestReceiverSetRXFailJSONValidationBeforeConnect(t *testing.T) {
	input := `{"channels":[{"index":4,"mode":0,"value":1000}]}`
	env, err := runTestCommandWithInput(t, []string{"receiver", "set-rxfail-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestFeatureEnableApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"features", "enable", "gps", "--apply", "--yes"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "feature GPS" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestModeSetValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"modes", "set", "x", "0", "0", "1700", "2100"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after validation failure")
	}
}

func TestSerialSetPlan(t *testing.T) {
	env, err := runTestCommand(t, []string{"serial", "set", "UART1", "64", "115200", "57600", "0", "115200"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	lines := data["cli_lines"].([]any)
	if lines[0] != "serial UART1 64 115200 57600 0 115200" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
}

func TestSerialSetJSONPlanDoesNotConnect(t *testing.T) {
	input := `{"serial":{"port":"UART1","function_mask":64,"msp_baud":115200,"gps_baud":57600,"telemetry_baud":0,"blackbox_baud":115200}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"serial", "set-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if lines[0] != "serial UART1 64 115200 57600 0 115200" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only serial set-json command")
	}
}

func TestSerialSetJSONValidationBeforeConnect(t *testing.T) {
	input := `{"identifier":51,"function_mask":64,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":-1,"blackbox_baudrate_index":0}`
	env, err := runTestCommandWithInput(t, []string{"serial", "set-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid serial JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestSerialSetJSONApplyWithFakeFC(t *testing.T) {
	input := `{"row":{"identifier":51,"function_mask":64,"msp_baudrate_index":5,"gps_baudrate_index":4,"telemetry_baudrate_index":0,"blackbox_baudrate_index":5}}`
	env, err := runTestCommandWithInput(t, []string{"serial", "set-json", "-", "--apply", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	if data["kind"] != "serial" || data["applied"] != true {
		t.Fatalf("data = %+v", data)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "serial 51 64 5 4 0 5" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRateprofilesListWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"rateprofiles", "list"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	view := data["view"].(map[string]any)
	rateprofiles := view["rateprofiles"].([]any)
	if len(rateprofiles) != 1 {
		t.Fatalf("rateprofiles = %+v", rateprofiles)
	}
}

func TestProfilesStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"profiles", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	profiles := data["profiles"].(map[string]any)
	if profiles["source"] != "MSP_STATUS_EX" {
		t.Fatalf("profiles = %+v", profiles)
	}
	pid := profiles["pid_profile"].(map[string]any)
	if pid["index"] != float64(0) || pid["count"] != float64(4) || pid["cli_command"] != "profile 0" {
		t.Fatalf("pid profile = %+v", pid)
	}
	rate := profiles["rate_profile"].(map[string]any)
	if rate["index"] != float64(0) || rate["count"] != float64(6) || rate["cli_command"] != "rateprofile 0" {
		t.Fatalf("rate profile = %+v", rate)
	}
	battery := profiles["battery_profile"].(map[string]any)
	if battery["index"] != float64(1) || battery["count"] != float64(3) || battery["cli_command"] != "battery_profile 1" {
		t.Fatalf("battery profile = %+v", battery)
	}
	if profiles["reboot_required"] != false {
		t.Fatalf("profiles = %+v", profiles)
	}
}

func TestProfilesBatterySelectPlanDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "battery-select", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if lines[0] != "battery_profile 1" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only battery profile command")
	}
}

func TestProfilesBatterySelectValidationFailureDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "battery-select", "one"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after battery profile validation failure")
	}
}

func TestProfilesBatterySelectApplyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"profiles", "battery-select", "1", "--apply", "--yes"}, nil)
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
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "battery_profile 1" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestProfilesSelectJSONPlanDoesNotConnect(t *testing.T) {
	input := `{"profiles":{"pid_profile":1,"rate_profile":2,"battery_profile":1}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"profiles", "select-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
	if len(lines) != 3 || lines[0] != "profile 1" || lines[1] != "rateprofile 2" || lines[2] != "battery_profile 1" || data["applied"] != false {
		t.Fatalf("plan = %+v", data)
	}
	if called {
		t.Fatal("connector was called for plan-only profile select-json command")
	}
}

func TestProfilesSelectJSONValidationBeforeConnect(t *testing.T) {
	input := `{"profile":1,"pid_profile":2}`
	env, err := runTestCommandWithInput(t, []string{"profiles", "select-json", "-", "--apply", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for conflicting profile JSON")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestProfilesSelectJSONApplyWithFakeFC(t *testing.T) {
	input := `{"pid_profile":1,"rateprofile":2,"battery_profile":1}`
	env, err := runTestCommandWithInput(t, []string{"profiles", "select-json", "-", "--apply", "--yes"}, input, nil)
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
	if len(env.SideEffects) != 3 || env.SideEffects[0].Command != "profile 1" || env.SideEffects[1].Command != "rateprofile 2" || env.SideEffects[2].Command != "battery_profile 1" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestProfilesCopyJSONRequiresYesDoesNotConnect(t *testing.T) {
	input := `{"profile_copy":{"kind":"pid","source":0,"destination":1}}`
	called := false
	env, err := runTestCommandWithInput(t, []string{"profiles", "copy-json", "-"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called before --yes confirmation")
	}
}

func TestProfilesCopyJSONValidationBeforeConnect(t *testing.T) {
	input := `{"copy":{"kind":"battery","source":0,"dest":1}}`
	env, err := runTestCommandWithInput(t, []string{"profiles", "copy-json", "-", "--yes"}, input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for unsupported copy kind")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
}

func TestProfilesCopyJSONWithFakeFC(t *testing.T) {
	input := `{"kind":"rate","source_index":0,"destination_index":1}`
	env, err := runTestCommandWithInput(t, []string{"profiles", "copy-json", "-", "--yes"}, input, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	copyResult := data["profile_copy"].(map[string]any)
	if copyResult["acknowledged"] != true || copyResult["save_required"] != true {
		t.Fatalf("profile_copy = %+v", copyResult)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Command != "MSP_COPY_PROFILE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestProfilesCopyRejectsUnsupportedKindDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "copy", "battery", "0", "1", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after profile copy validation failure")
	}
}

func TestProfilesCopyRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"profiles", "copy", "pid", "0", "1"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after profile copy confirmation failure")
	}
}

func TestProfilesCopyWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"profiles", "copy", "rate", "1", "2", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	copyResult := data["profile_copy"].(map[string]any)
	request := copyResult["request"].(map[string]any)
	if request["kind"] != "rate" || request["source"] != float64(1) || request["destination"] != float64(2) {
		t.Fatalf("profile copy = %+v", copyResult)
	}
	if copyResult["msp_name"] != "MSP_COPY_PROFILE" || copyResult["save_required"] != true {
		t.Fatalf("profile copy = %+v", copyResult)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "profile_copy" || env.SideEffects[0].Command != "MSP_COPY_PROFILE" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestDomainSaveRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"features", "enable", "gps", "--apply", "--save"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called after confirmation failure")
	}
}
