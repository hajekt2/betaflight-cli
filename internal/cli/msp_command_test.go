package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestMSPRequestWriteLikeRequiresYes(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "11"}, "", func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("msp request was sent without --yes")
	}

	_, err = runTestCommandWithInput(t, []string{"msp", "request", "11", "--yes", "--payload-hex", ""}, "", nil)
	if err != nil {
		// connection path is backed by fake serial transport in tests, so protocol-level failures are expected.
	}
}

func TestMSPRequestUnknownCodeRequiresYes(t *testing.T) {
	called := false
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "4095"}, "", func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if !strings.Contains(env.Errors[0].Message, "without compiled metadata") {
		t.Fatalf("message = %q, want unknown metadata warning", env.Errors[0].Message)
	}
	if called {
		t.Fatal("unknown msp request attempted transport without --yes")
	}
}

func TestMSPRequestUnknownCodeWithYesUsesDangerousOperation(t *testing.T) {
	var gotOp connection.OperationClass
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "4095", "--yes"}, "", func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		gotOp = op
		return nil, connection.TargetInfo{}, &connection.CodedError{Code: "test_stop", Message: "stop before transport"}
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if gotOp != connection.Dangerous {
		t.Fatalf("operation = %v, want %v", gotOp, connection.Dangerous)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "test_stop" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestMSPRequestBlocksRawActuationEvenWithYes(t *testing.T) {
	for _, code := range []string{"MSP_SET_RAW_RC", "MSP_SET_MOTOR", "MSP2_SEND_DSHOT_COMMAND"} {
		t.Run(code, func(t *testing.T) {
			called := false
			env, err := runTestCommand(t, []string{"msp", "request", code, "--yes"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				called = true
				return nil, connection.TargetInfo{}, nil
			})
			if err == nil || env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "dangerous_action_blocked" {
				t.Fatalf("unexpected envelope: %+v, err: %v", env, err)
			}
			if called {
				t.Fatal("blocked raw MSP actuation attempted transport")
			}
		})
	}
}

func TestMSP2SetterWithUnknownDirectionRequiresYes(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_SET_TEXT"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil || env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v, err: %v", env, err)
	}
	if called {
		t.Fatal("MSP2 setter attempted transport without --yes")
	}
}

func TestMSPRequestInvalidCodeReturnsValidationError(t *testing.T) {
	// invalid code that cannot be resolved must fail before attempting transport
	called := false
	env, err := runTestCommand(t, []string{"msp", "request", "not-a-code"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) == 0 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope for invalid code: %+v", env.Errors)
	}
	if called {
		t.Fatal("msp request attempted transport for invalid code")
	}
}

func TestMSPRequestInvalidPayloadHexFails(t *testing.T) {
	// malformed hex should fail with explicit payload error
	called := false
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "MSP_NAME", "--yes", "--payload-hex", "Z1"}, "", func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) == 0 || !strings.Contains(strings.ToLower(env.Errors[0].Message), "encoding/hex") {
		t.Fatalf("unexpected envelope for invalid payload hex: %+v", env.Errors)
	}
	if called {
		t.Fatal("msp request attempted transport for invalid payload")
	}
}

func TestMSPRequestReadLikeDoesNotRequireYes(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"msp", "request", "MSP_API_VERSION"}, "", nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	for _, e := range env.Errors {
		if e.Code == "confirmation_required" {
			t.Fatalf("unexpected confirmation_required for read-like MSP request: %+v", env.Errors)
		}
	}
}

func TestMSPListDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"msp", "list"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if called {
		t.Fatalf("msp list should not connect")
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := mspResponseData(t, env)
	if data["count"] == nil {
		t.Fatalf("msp list payload = %+v", data)
	}
}

func TestMSPMetadataDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"msp", "metadata"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if called {
		t.Fatalf("msp metadata should not connect")
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := mspResponseData(t, env)
	if data["count"] == nil || data["registry_version"] != msp.GeneratedMSPSourceVersion {
		t.Fatalf("msp metadata payload = %+v", data)
	}
}

func TestMSPRequestByNameReturnsMetadata(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_NAME"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := mspResponseData(t, env)
	if data["code"].(float64) != float64(msp.MSPName) {
		t.Fatalf("code = %+v", data["code"])
	}
	if data["code_name"] != "MSP_NAME" {
		t.Fatalf("code_name = %+v", data["code_name"])
	}
	if data["payload_hex"].(string) != "42657461466c69676874" {
		t.Fatalf("payload_hex = %+v", data["payload_hex"])
	}
	if data["command_source"].(string) != "src/main/msp/msp_protocol.h" {
		t.Fatalf("command_source = %+v", data["command_source"])
	}
	if data["direction_hint"] != "read" {
		t.Fatalf("direction_hint = %+v", data["direction_hint"])
	}
}

func TestMSPBatchReturnsLengthPrefixedResponses(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "batch", "MSP_NAME", "MSP_ATTITUDE", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := mspResponseData(t, env)
	if data["code"].(float64) != float64(msp.MSPMultipleMsp) || data["request_count"].(float64) != 2 || data["response_count"].(float64) != 2 {
		t.Fatalf("msp batch payload = %+v", data)
	}
	if data["truncated"] != false {
		t.Fatalf("truncated = %+v", data["truncated"])
	}
	responses := data["responses"].([]any)
	name := responses[0].(map[string]any)
	if name["code_name"] != "MSP_NAME" || name["decoded"] != "Fake FC" || name["decode_supported"] != true {
		t.Fatalf("name response = %+v", name)
	}
	attitude := responses[1].(map[string]any)
	if attitude["code_name"] != "MSP_ATTITUDE" || attitude["decode_supported"] != true {
		t.Fatalf("attitude response = %+v", attitude)
	}
	decoded := attitude["decoded"].(map[string]any)
	if decoded["roll_degrees"] != 12.3 || decoded["pitch_degrees"] != -4.5 || decoded["yaw_degrees"] != float64(1800) {
		t.Fatalf("decoded attitude = %+v", decoded)
	}
}

func TestMSPBatchRejectsWriteAndV2Commands(t *testing.T) {
	cases := [][]string{
		{"msp", "batch", "MSP_SET_NAME"},
		{"msp", "batch", "MSP2_GET_TEXT"},
		{"msp", "batch", "0xFFFF"},
	}
	for _, args := range cases {
		t.Run(strings.Join(args, "_"), func(t *testing.T) {
			called := false
			env, err := runTestCommand(t, args, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				called = true
				return nil, connection.TargetInfo{}, nil
			})
			if err == nil {
				t.Fatal("command error = nil, want validation failure")
			}
			if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
				t.Fatalf("unexpected envelope = %+v", env)
			}
			if called {
				t.Fatalf("transport called for invalid batch args")
			}
		})
	}
}

func TestMSPRequestNumericFallbackUsesCodeName(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "0xFFFF", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := mspResponseData(t, env)
	if data["code"].(float64) != 65535 {
		t.Fatalf("code = %+v", data["code"])
	}
	if data["code_name"] != "MSP_65535" {
		t.Fatalf("code_name = %+v", data["code_name"])
	}
	if _, ok := data["command_source"].(string); !ok || data["command_source"] != "" {
		t.Fatalf("command_source = %+v", data["command_source"])
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "raw_msp" || !strings.Contains(env.SideEffects[0].Detail, "without compiled metadata") {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestMSPRequestWithDecodeReturnsStructuredPayload(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_NAME", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	if data["decode_requested"] != true {
		t.Fatalf("decode_requested = %v", data["decode_requested"])
	}
	if data["decoded"] != "BetaFlight" {
		t.Fatalf("decoded = %v", data["decoded"])
	}
}

func TestMSPRequestWithDecodeUnsupportedCodeFallsBackToRaw(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "0xFFFF", "--decode", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != false {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	if data["decode_requested"] != true {
		t.Fatalf("decode_requested = %v", data["decode_requested"])
	}
	if _, ok := data["decoded"]; ok {
		t.Fatalf("decoded should be absent for unsupported code: %+v", data["decoded"])
	}
}

func TestMSPRequestWithDecodeForBatteryProfile(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_BATTERY_PROFILE", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["index"] != float64(0) {
		t.Fatalf("decoded index = %v", decoded["index"])
	}
	if decoded["capacity_mah"] != float64(1300) {
		t.Fatalf("decoded capacity_mah = %v", decoded["capacity_mah"])
	}
	if decoded["force_cell_count"] != float64(4) {
		t.Fatalf("decoded force_cell_count = %v", decoded["force_cell_count"])
	}
}

func TestMSPRequestWithDecodeForLEDStripConfig(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_LED_STRIP_CONFIG", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["profile"] != float64(0) {
		t.Fatalf("decoded profile = %v", decoded["profile"])
	}
	if decoded["advanced_supported"] != true {
		t.Fatalf("decoded advanced_supported = %v", decoded["advanced_supported"])
	}
	leds := decoded["leds"].([]any)
	if len(leds) < 2 {
		t.Fatalf("decoded leds = %+v", leds)
	}
}

func TestMSPRequestWithDecodeForGPSRescue(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_GPS_RESCUE", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["max_rescue_angle"] != float64(3200) {
		t.Fatalf("decoded max_rescue_angle = %v", decoded["max_rescue_angle"])
	}
	if decoded["return_altitude_m"] != float64(100) || decoded["descent_distance_m"] != float64(50) {
		t.Fatalf("decoded altitude fields = %+v", decoded)
	}
	if decoded["sanity_checks"] != float64(1) || decoded["min_sats"] != float64(8) {
		t.Fatalf("decoded rescue options = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForGPSHomeFromCompGPS(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_COMP_GPS", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["distance_m"] != float64(342) || decoded["direction_degrees"] != float64(184) || decoded["update"] != true {
		t.Fatalf("decoded home = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForGPSRescuePIDs(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_GPS_RESCUE_PIDS", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["altitude_p"] != float64(80) || decoded["velocity_p"] != float64(120) || decoded["yaw_p"] != float64(45) {
		t.Fatalf("decoded rescue pid = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForDebugValues(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_DEBUG", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].([]any)
	if len(decoded) != 8 || decoded[0] != float64(-1) || decoded[7] != float64(8) {
		t.Fatalf("decoded debug values = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForAttitudeQuaternion(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_ATTITUDE_QUATERNION", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["w"] != float64(1) || decoded["x"] != float64(0) || decoded["y"] != float64(0) || decoded["z"] != float64(0) {
		t.Fatalf("decoded quaternion = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForTXInfo(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_TX_INFO", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["rssi_source_name"] != "RX_PROTOCOL_CRSF" || decoded["rtc_status_name"] != "SET" || decoded["rtc_is_set"] != true {
		t.Fatalf("decoded tx info = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForText(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_GET_TEXT", "--decode", "--payload-hex", "01"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["type"] != float64(1) {
		t.Fatalf("decoded type = %v", decoded["type"])
	}
	if decoded["value"] != "BF pilot" {
		t.Fatalf("decoded value = %v", decoded["value"])
	}
	if decoded["text_length"] != float64(8) {
		t.Fatalf("decoded text_length = %v", decoded["text_length"])
	}
}

func TestMSPRequestWithDecodeForFirmwareSetting(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_CLI_SETTING", "--payload-hex", "6779726f5f6c7066315f7374617469635f687a", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["name"] != "gyro_lpf1_static_hz" || decoded["value"] != "42" || decoded["source"] != "MSP2_CLI_SETTING" {
		t.Fatalf("decoded firmware setting = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForFirmwareSettingInfo(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_CLI_SETTING_INFO", "--payload-hex", "6779726f5f6c7066315f7374617469635f687a000000", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["complete"] != true || decoded["total_bytes"].(float64) <= 0 || !strings.Contains(decoded["text"].(string), "type: uint16") {
		t.Fatalf("decoded firmware setting info = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForGyroSensorsActive(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_GYRO_SENSOR_ACTIVE", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["source"] != "MSP2_GYRO_SENSOR_ACTIVE" {
		t.Fatalf("decoded source = %v", decoded["source"])
	}
	if decoded["count"] != float64(2) {
		t.Fatalf("decoded count = %v", decoded["count"])
	}
	hardware := decoded["hardware"].([]any)
	if len(hardware) != 2 || hardware[0].(map[string]any)["hardware_id"] != float64(11) || hardware[1].(map[string]any)["name"] != "ICM45605" {
		t.Fatalf("decoded hardware = %+v", hardware)
	}
}

func TestMSPRequestWithDecodeForSensorConfigActive(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_SENSOR_CONFIG_ACTIVE", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].([]any)
	if len(decoded) != 6 || decoded[0] != float64(10) || decoded[5] != float64(15) {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForOSDWarnings(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_GET_OSD_WARNINGS", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["display_attributes"] != float64(2) {
		t.Fatalf("decoded display_attributes = %v", decoded["display_attributes"])
	}
	if decoded["text"] != "LOW BATTERY" {
		t.Fatalf("decoded text = %v", decoded["text"])
	}
}

func TestMSPRequestWithDecodeForServoAndMotorOutputOrder(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_SERVO", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	servo := data["decoded"].([]any)
	if len(servo) != 4 || servo[0] != float64(1500) || servo[3] != float64(0) {
		t.Fatalf("decoded servo = %+v", servo)
	}

	env, err = runTestCommand(t, []string{"msp", "request", "MSP2_MOTOR_OUTPUT_REORDERING", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data = mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	order := data["decoded"].([]any)
	if len(order) != 4 || order[0] != float64(0) || order[3] != float64(3) {
		t.Fatalf("order channels = %+v", order)
	}

	env, err = runTestCommand(t, []string{"msp", "request", "MSP_ESC_SENSOR_DATA", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data = mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	escData := data["decoded"].([]any)
	firstESC := escData[0].(map[string]any)
	if firstESC["temperature_c"] != float64(40) || firstESC["rpm"] != float64(12000) {
		t.Fatalf("decoded esc sensor data = %+v", escData)
	}
}

func TestMSPRequestWithDecodeForRawIMU(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_RAW_IMU", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	imuRaw := decoded["accelerometer_raw"].([]any)
	if len(imuRaw) != 3 || imuRaw[0] != float64(2048) || imuRaw[1] != float64(-1024) {
		t.Fatalf("accelerometer_raw = %+v", imuRaw)
	}
	g := decoded["accelerometer_g"].([]any)
	if g[0] != float64(1) || g[1] != float64(-0.5) {
		t.Fatalf("accelerometer_g = %+v", g)
	}
}

func TestMSPRequestWithDecodeForDataflashErase(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_DATAFLASH_ERASE", "--decode", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	if _, ok := data["decoded"].(map[string]any); !ok {
		t.Fatalf("decoded = %+v", data["decoded"])
	}
}

func TestMSPRequestWithDecodeForReboot(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_REBOOT", "--decode", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	payload, ok := decoded["payload_bytes"].([]any)
	if !ok || len(payload) == 0 {
		t.Fatalf("decoded payload_bytes = %+v", decoded["payload_bytes"])
	}
	if payload[0] != float64(0) {
		t.Fatalf("decoded payload_bytes = %+v", payload)
	}
}

func TestMSPRequestWithDecodeForSensorConfig(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_SENSOR_CONFIG", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	sensorConfig := data["decoded"].([]any)
	if len(sensorConfig) != 5 {
		t.Fatalf("decoded sensor config = %+v", sensorConfig)
	}
	if sensorConfig[0].(map[string]any)["hardware_id"] != float64(1) || sensorConfig[0].(map[string]any)["available"] != true {
		t.Fatalf("sensor config[0] = %+v", sensorConfig[0])
	}
}

func TestMSPRequestWithDecodeForRXMapUsesNumberArray(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_RX_MAP", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	if decoded, ok := data["decoded"].([]any); ok {
		if len(decoded) != 4 || decoded[0] != float64(0) || decoded[1] != float64(1) {
			t.Fatalf("decoded = %+v", decoded)
		}
	} else {
		t.Fatalf("decoded type = %T, value = %+v", data["decoded"], data["decoded"])
	}
}

func TestMSPRequestWithDecodeForRSSIConfig(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_RSSI_CONFIG", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["channel"] != float64(8) || decoded["source"] != "MSP_RSSI_CONFIG" {
		t.Fatalf("decoded rssi config = %+v", decoded)
	}
}

func TestMSPRequestWithDecodeForVTXTableRows(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_VTXTABLE_BAND", "--payload-hex", "01", "--decode"}, nil)
	if err != nil {
		t.Fatalf("band command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("band env.OK = %v: %+v", env.OK, env.Errors)
	}
	bandData := mspResponseData(t, env)
	if bandData["decode_supported"] != true {
		t.Fatalf("band decode_supported = %v", bandData["decode_supported"])
	}
	band := bandData["decoded"].(map[string]any)
	frequencies := band["frequencies_mhz"].([]any)
	if band["band"] != float64(1) || band["name"] != "RACEBAND" || frequencies[0] != float64(5658) {
		t.Fatalf("decoded band = %+v", band)
	}

	env, err = runTestCommand(t, []string{"msp", "request", "MSP_VTXTABLE_POWERLEVEL", "--payload-hex", "02", "--decode"}, nil)
	if err != nil {
		t.Fatalf("power command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("power env.OK = %v: %+v", env.OK, env.Errors)
	}
	powerData := mspResponseData(t, env)
	if powerData["decode_supported"] != true {
		t.Fatalf("power decode_supported = %v", powerData["decode_supported"])
	}
	power := powerData["decoded"].(map[string]any)
	if power["level"] != float64(2) || power["value"] != float64(200) || power["label"] != "200" {
		t.Fatalf("decoded power = %+v", power)
	}
}

func TestMSPRequestWithDecodeForVTXDeviceStatus(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP2_GET_VTX_DEVICE_STATUS", "--decode"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["decode_supported"] != true {
		t.Fatalf("decode_supported = %v", data["decode_supported"])
	}
	decoded := data["decoded"].(map[string]any)
	if decoded["type_name"] != "SMARTAUDIO" || decoded["frequency_mhz"] != float64(5861) || decoded["pit_mode"] != true {
		t.Fatalf("decoded = %+v", decoded)
	}
}

func TestMSPMetadataByName(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "metadata", "MSP_NAME"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["code"].(float64) != float64(msp.MSPName) {
		t.Fatalf("code = %+v", data["code"])
	}
	if data["code_name"] != "MSP_NAME" {
		t.Fatalf("code_name = %+v", data["code_name"])
	}
	if data["direction"] != string(msp.DirectionRead) {
		t.Fatalf("direction = %+v", data["direction"])
	}
}

func TestMSPMetadataSummary(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "metadata"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["registry_version"] != msp.GeneratedMSPSourceVersion || data["count"].(float64) < 100 {
		t.Fatalf("summary = %+v", data)
	}
	directions := data["directions"].(map[string]any)
	if directions[string(msp.DirectionRead)].(float64) == 0 || directions[string(msp.DirectionWrite)].(float64) == 0 {
		t.Fatalf("directions = %+v", directions)
	}
	protocols := data["protocols"].(map[string]any)
	if protocols["1"].(float64) == 0 || protocols["2"].(float64) == 0 {
		t.Fatalf("protocols = %+v", protocols)
	}
	sources := data["sources"].(map[string]any)
	if sources["src/main/msp/msp_protocol.h"].(float64) == 0 {
		t.Fatalf("sources = %+v", sources)
	}
}

func TestMSPListReturnsCommands(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "list", "--name", "MSP_NAME", "--direction", "read"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	if data["count"].(float64) == 0 {
		t.Fatalf("count = %+v", data["count"])
	}
	commands := data["commands"].([]any)
	if len(commands) == 0 {
		t.Fatalf("commands = %+v", data["commands"])
	}
	first := commands[0].(map[string]any)
	if first["name"].(string) != "MSP_NAME" {
		t.Fatalf("first name = %+v", first["name"])
	}
}

func TestMSPListCanFilterByProtocol(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "list", "--protocol", "1", "--name", "MSP_NAME"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	data := mspResponseData(t, env)
	commands := data["commands"].([]any)
	if len(commands) != 1 {
		t.Fatalf("commands = %+v", data["commands"])
	}
	command := commands[0].(map[string]any)
	if command["protocol"].(float64) != 1 {
		t.Fatalf("protocol = %+v", command["protocol"])
	}
}

func TestMSPRequestRequiresYesForWriteLikeCommands(t *testing.T) {
	env, err := runTestCommand(t, []string{"msp", "request", "MSP_SET_NAME", "--payload-hex", "466f6f"}, nil)
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func mspResponseData(t *testing.T, env output.Envelope) map[string]any {
	t.Helper()
	data := env.Data.(map[string]any)
	mspData, ok := data["msp"].(map[string]any)
	if !ok {
		t.Fatalf("data.msp = %T %+v", data["msp"], data["msp"])
	}
	return mspData
}
