package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestVTXConfigWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	config := data["vtx"].(map[string]any)
	if config["type_name"] != "SMARTAUDIO" || config["frequency_mhz"] != float64(5861) || config["pit_mode"] != true {
		t.Fatalf("config = %+v", config)
	}
	table := config["table"].(map[string]any)
	if table["available"] != true || table["bands"] != float64(5) || table["channels"] != float64(8) || table["power_levels"] != float64(3) {
		t.Fatalf("table = %+v", table)
	}
}

func TestVTXDeviceStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "device-status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	status := data["vtx_device"].(map[string]any)
	if status["supported"] != true || status["device_present"] != true || status["type_name"] != "SMARTAUDIO" || status["ready"] != true {
		t.Fatalf("status = %+v", status)
	}
	if status["band_channel_available"] != true || status["band"] != float64(5) || status["channel"] != float64(8) {
		t.Fatalf("status = %+v", status)
	}
	if status["frequency_available"] != true || status["frequency_mhz"] != float64(5861) || status["pit_mode"] != true || status["locked"] != true {
		t.Fatalf("status = %+v", status)
	}
	levels := status["power_levels"].([]any)
	if len(levels) != 2 || levels[1].(map[string]any)["power"] != float64(200) {
		t.Fatalf("power levels = %+v", levels)
	}
	custom := status["custom_status_bytes"].([]any)
	if len(custom) != 2 || custom[0] != float64(170) {
		t.Fatalf("custom status bytes = %+v", custom)
	}
}

func TestVTXDeviceStatusReportsUnsupportedTarget(t *testing.T) {
	env, err := runTestCommand(t, []string{"vtx", "device-status"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		fc := fakefc.New()
		fc.Unsupported[msp.MSP2GetVTXDeviceStatus] = true
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
	status := data["vtx_device"].(map[string]any)
	if status["supported"] != false || status["unsupported_reason"] == "" {
		t.Fatalf("status = %+v", status)
	}
	if len(env.Warnings) != 1 || env.Warnings[0].Code != "unsupported_msp" {
		t.Fatalf("warnings = %+v", env.Warnings)
	}
}
