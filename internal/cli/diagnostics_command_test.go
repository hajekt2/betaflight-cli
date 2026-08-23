package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func TestVersionIncludesCompiledMetadata(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"version"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatal("connector was called for version")
	}
	data := env.Data.(map[string]any)
	if data["version"] != "test" || data["commit"] != "test" || data["date"] != "test" {
		t.Fatalf("version data = %+v", data)
	}
	if data["schema_version"] != output.SchemaVersion {
		t.Fatalf("schema_version = %v", data["schema_version"])
	}
	if data["msp_source_firmware"] != "2026.6.1" || data["settings_source_firmware"] != "2026.6.1" {
		t.Fatalf("metadata versions = %+v", data)
	}
	if data["settings_generated"] != true || data["settings_count"].(float64) < 100 {
		t.Fatalf("settings metadata = %+v", data)
	}
	files := data["settings_source_files"].([]any)
	if !containsAnyString(files, "src/main/cli/settings.c") {
		t.Fatalf("settings_source_files = %+v", files)
	}
}

func TestFirmwareFlashPlanModeWorksOffline(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0xaa, 0xbb}, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	env, err := runTestCommand(t, []string{
		"firmware", "flash", "--image", image, "--tool", "dfu-util", "--tool-arg", "-a", "--tool-arg=0", "--tool-arg", "-s", "--tool-arg=0x08000000:leave",
	}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	flash := data["firmware_flash"].(map[string]any)
	if flash["executed"].(bool) {
		t.Fatalf("plan mode must not execute: %+v", flash)
	}
	if flash["action"].(string) != "plan" {
		t.Fatalf("plan action = %v", flash["action"])
	}
}

func TestFirmwareFlashExecuteRequiresYes(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0x11, 0x22}, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	env, err := runTestCommand(t, []string{
		"firmware", "flash", "--image", image, "--tool", "dfu-util", "--execute",
	}, nil)
	if err == nil {
		t.Fatalf("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestFirmwareFlashExecuteWithoutRebootDoesNotConnect(t *testing.T) {
	tmp := t.TempDir()
	image := filepath.Join(tmp, "firmware.bin")
	if err := os.WriteFile(image, []byte{0x11, 0x22}, 0o600); err != nil {
		t.Fatalf("write image: %v", err)
	}
	called := false
	env, err := runTestCommand(t, []string{
		"firmware", "flash", "--image", image, "--tool", "/bin/echo", "--execute", "--yes",
	}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if called {
		t.Fatal("connector was called for firmware flash execute without --reboot-first")
	}
	if env.Target != nil {
		t.Fatalf("target = %+v, want nil for external-only flash", env.Target)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	flash := env.Data.(map[string]any)["firmware_flash"].(map[string]any)
	if flash["executed"] != true || flash["successful"] != true {
		t.Fatalf("firmware flash = %+v", flash)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "firmware_flash" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestFirmwareFlashPlanModeRequiresImage(t *testing.T) {
	env, err := runTestCommand(t, []string{"firmware", "flash", "--tool", "dfu-util"}, nil)
	if err == nil {
		t.Fatalf("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

func TestWriteOperationsDefaultAutoPortWhenNotProvided(t *testing.T) {
	var observed connection.Config
	env, err := runTestCommand(t, []string{"settings", "set", "small_angle", "5", "--apply", "--yes"}, func(_ context.Context, cfg connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		observed = cfg
		client, clientErr := connection.NewClient(fakefc.New(), time.Second)
		if clientErr != nil {
			return nil, connection.TargetInfo{}, clientErr
		}
		target, targetErr := client.Handshake(context.Background())
		if targetErr != nil {
			client.Close()
			return nil, connection.TargetInfo{}, targetErr
		}
		target.Port = "fake"
		return client, target, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	if !observed.AutoPort {
		t.Fatalf("expected AutoPort to default true for write commands: %#v", observed)
	}
}

func TestAllowUnsupportedFlagIsForwarded(t *testing.T) {
	var observed connection.Config
	env, err := runTestCommand(t, []string{"--allow-unsupported", "features", "status"}, func(_ context.Context, cfg connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		observed = cfg
		client, clientErr := connection.NewClient(fakefc.New(), time.Second)
		if clientErr != nil {
			return nil, connection.TargetInfo{}, clientErr
		}
		target, targetErr := client.Handshake(context.Background())
		if targetErr != nil {
			client.Close()
			return nil, connection.TargetInfo{}, targetErr
		}
		target.Port = "fake"
		return client, target, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = %v: %+v", env.OK, env.Errors)
	}
	if !observed.AllowUnsupported {
		t.Fatalf("expected AllowUnsupported to be forwarded: %#v", observed)
	}
}

func TestAllowUnsupportedAddsTopLevelWarning(t *testing.T) {
	env, err := runTestCommand(t, []string{"--allow-unsupported", "features", "status"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		client, clientErr := connection.NewClient(fakefc.New(), time.Second)
		if clientErr != nil {
			return nil, connection.TargetInfo{}, clientErr
		}
		return client, connection.TargetInfo{Port: "fake", Variant: "INAV", FirmwareVersion: "2025.12.1", MSPAPIVersion: "1.48"}, nil
	})
	if err != nil || !env.OK {
		t.Fatalf("unexpected result: env=%+v err=%v", env, err)
	}
	if len(env.Warnings) == 0 || env.Warnings[len(env.Warnings)-1].Code != "unsupported_firmware" {
		t.Fatalf("warnings = %+v, want unsupported_firmware", env.Warnings)
	}
}

func TestUnsupportedFirmwareFailureIncludesTarget(t *testing.T) {
	unsupportedTarget := connection.TargetInfo{
		Port:            "fake",
		Variant:         "BTFL",
		FirmwareVersion: "2025.11.0",
		MSPAPIVersion:   "1.48",
		MSPProtocol:     0,
	}
	env, err := runTestCommand(t, []string{"features", "status"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		return nil, unsupportedTarget, &connection.CodedError{
			Code:    "unsupported_firmware",
			Message: "firmware \"2025.11.0\" is outside the supported metadata set; pass --allow-unsupported to continue",
		}
	})
	if !isExitError(err) {
		t.Fatalf("command error = %T %v, want exitError", err, err)
	}
	if env.OK {
		t.Fatalf("env.OK = true, want false")
	}
	if len(env.Errors) != 1 || env.Errors[0].Code != "unsupported_firmware" {
		t.Fatalf("env.Errors = %+v, want unsupported_firmware", env.Errors)
	}
	if env.Target == nil || env.Target.FirmwareVersion != "2025.11.0" || env.Target.MSPAPIVersion != "1.48" {
		t.Fatalf("env.Target = %+v, want unsupported target metadata", env.Target)
	}
}

func TestInfoWithFakeFCIncludesBuildMetadata(t *testing.T) {
	env, err := runTestCommand(t, []string{"info"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)["info"].(map[string]any)
	if data["legacy_name"] != "BetaFlight" {
		t.Fatalf("legacy name = %+v", data["legacy_name"])
	}
	support := data["support"].(map[string]any)
	if support["supported"] != true {
		t.Fatalf("support = %+v", support)
	}
	if support["policy"] != "official Betaflight 2025.12.x and newer" {
		t.Fatalf("support policy = %+v", support)
	}
	board := data["board"].(map[string]any)
	if board["configuration_state_name"] != "CONFIGURED" || board["sample_rate_hz"] != float64(8000) {
		t.Fatalf("board = %+v", board)
	}
	problems := board["configuration_problems"].(map[string]any)
	if problems["mask"] != float64(3) {
		t.Fatalf("configuration problems = %+v", problems)
	}
	mcu := data["mcu"].(map[string]any)
	if mcu["source"] != "MSP2_MCU_INFO" || mcu["id"] != float64(254) || mcu["name"] != "STM32F405" {
		t.Fatalf("mcu = %+v", mcu)
	}
	uid := data["uid"].(map[string]any)
	if uid["source"] != "MSP_UID" || uid["hex"] != "0123456789abcdef00000042" {
		t.Fatalf("uid = %+v", uid)
	}
	build := data["build"].(map[string]any)
	if build["date_time"] != "Jan 01 2026 00:00:00" || build["git_revision"] != "abc1234" {
		t.Fatalf("build = %+v", build)
	}
	names := build["build_option_names"].([]any)
	if len(names) != 2 || names[0] != "USE_GPS" || names[1] != "USE_DSHOT" {
		t.Fatalf("build option names = %+v", names)
	}
	unknown := build["unknown_options"].([]any)
	if len(unknown) != 1 || unknown[0].(map[string]any)["code"] != float64(65000) {
		t.Fatalf("unknown options = %+v", unknown)
	}
}

func TestFirmwareStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"firmware", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	firmware := data["firmware"].(map[string]any)
	if firmware["variant"] != "BTFL" || firmware["version"] != "2025.12.1" || firmware["msp_api"] != "1.48" {
		t.Fatalf("firmware identity = %+v", firmware)
	}
	support := firmware["support"].(map[string]any)
	if support["supported"] != true || support["policy"] != "official Betaflight 2025.12.x and newer" {
		t.Fatalf("support = %+v", support)
	}
	target := firmware["target"].(map[string]any)
	if target["target_name"] != "STM32F405" || target["board_name"] != "FAKEF405" {
		t.Fatalf("target = %+v", target)
	}
	metadata := firmware["metadata"].(map[string]any)
	if metadata["settings_source_firmware"] != "2026.6.1" || metadata["settings_count"].(float64) < 100 {
		t.Fatalf("metadata = %+v", metadata)
	}
	capabilities := firmware["capabilities"].(map[string]any)
	if capabilities["sample_rate_hz"] != float64(8000) {
		t.Fatalf("capabilities = %+v", capabilities)
	}
	identity := firmware["identity"].(map[string]any)
	if identity["configurator_uid"] != "123456789abcdef42" || identity["configuration_state_name"] != "CONFIGURED" {
		t.Fatalf("identity = %+v", identity)
	}
}

func TestTargetStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"target", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	target := data["target"].(map[string]any)
	summary := target["summary"].(map[string]any)
	if summary["supported"] != true || summary["firmware_version"] != "2025.12.1" || summary["target_name"] != "STM32F405" {
		t.Fatalf("summary = %+v", summary)
	}
	if summary["board_name"] != "FAKEF405" || summary["configuration_state"] != "CONFIGURED" || summary["build_key"] != "fake-build-key" {
		t.Fatalf("summary identity = %+v", summary)
	}
	if summary["resource_count"] != float64(2) || summary["timer_count"] != float64(2) || summary["dma_count"] != float64(2) {
		t.Fatalf("summary counts = %+v", summary)
	}
	if target["firmware"] == nil || target["system"] == nil || target["resources"] == nil {
		t.Fatalf("target sections = %+v", target)
	}
}

func TestProbeSupportAndMetadata(t *testing.T) {
	support := probeSupport(connection.TargetInfo{
		Variant:         "BTFL",
		FirmwareVersion: "2025.12.1",
		MSPAPIVersion:   "1.48",
	})
	if !support.Supported || support.Reason != "firmware is inside the supported metadata range" {
		t.Fatalf("support = %+v", support)
	}
	metadata := probeMetadata()
	if metadata["settings_source_firmware"] != "2026.6.1" || metadata["settings_count"].(int) < 100 {
		t.Fatalf("metadata = %+v", metadata)
	}
	files := metadata["settings_source_files"].([]string)
	if len(files) == 0 || files[0] != "src/main/cli/settings.c" {
		t.Fatalf("settings_source_files = %+v", files)
	}
}

func TestDiagnosePortsRecommendations(t *testing.T) {
	tests := []struct {
		name              string
		ports             []connection.PortInfo
		candidateCount    int
		singleCandidate   bool
		recommendedPort   string
		recommendedAction string
		warningCount      int
	}{
		{
			name: "none",
			ports: []connection.PortInfo{
				{Name: "/dev/cu.Bluetooth-Incoming-Port", Candidate: false, Reason: "ignored non-USB or debug port"},
			},
			recommendedAction: "connect a Betaflight flight controller over USB, then run doctor --probe",
			warningCount:      1,
		},
		{
			name: "single",
			ports: []connection.PortInfo{
				{Name: "/dev/cu.usbmodem01", Candidate: true, Reason: "macOS USB serial candidate"},
			},
			candidateCount:    1,
			singleCandidate:   true,
			recommendedPort:   "/dev/cu.usbmodem01",
			recommendedAction: "run doctor --probe or use this port for read-only commands",
		},
		{
			name: "multiple",
			ports: []connection.PortInfo{
				{Name: "/dev/cu.usbmodem01", Candidate: true, Reason: "macOS USB serial candidate"},
				{Name: "/dev/cu.usbmodem02", Candidate: true, Reason: "macOS USB serial candidate"},
			},
			candidateCount:    2,
			recommendedAction: "run doctor --probe or pass --port explicitly after selecting the intended flight controller",
			warningCount:      1,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := diagnosePorts(tt.ports)
			if got.CandidateCount != tt.candidateCount || got.SingleCandidate != tt.singleCandidate || got.RecommendedPort != tt.recommendedPort {
				t.Fatalf("diagnostics = %+v", got)
			}
			if got.RecommendedAction != tt.recommendedAction {
				t.Fatalf("recommended action = %q", got.RecommendedAction)
			}
			if len(got.Warnings) != tt.warningCount {
				t.Fatalf("warnings = %+v", got.Warnings)
			}
		})
	}
}
