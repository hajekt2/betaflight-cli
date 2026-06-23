//go:build hardware

package hardwaretest

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestHardwareReadOnlySmoke(t *testing.T) {
	port := os.Getenv("BETAFLIGHT_CLI_HARDWARE_PORT")
	if port == "" {
		t.Skip("set BETAFLIGHT_CLI_HARDWARE_PORT and run with -tags hardware to enable read-only hardware integration tests")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	client, target, err := connection.Connect(ctx, connection.Config{
		Port:     port,
		Baud:     115200,
		Timeout:  3 * time.Second,
		AutoPort: false,
	}, connection.ReadOnly)
	if err != nil {
		t.Fatalf("read-only hardware connect failed: %v", err)
	}
	defer client.Close()

	if target.Port != port || target.Variant == "" || target.FirmwareVersion == "" || target.MSPAPIVersion == "" {
		t.Fatalf("target identity is incomplete: %+v", target)
	}

	info, warnings := commands.ReadInfo(ctx, client)
	if len(warnings) > 0 {
		t.Logf("read-only info warnings: %v", warnings)
	}
	if info.Variant == "" || info.FirmwareVersion == "" || info.MSPAPIVersion == "" {
		t.Fatalf("info identity is incomplete: %+v", info)
	}

	telemetry, telemetryWarnings := commands.ReadTelemetry(ctx, client)
	if len(telemetryWarnings) > 0 {
		t.Logf("read-only telemetry warnings: %v", telemetryWarnings)
	}
	if len(telemetry.Sources) == 0 {
		t.Fatalf("telemetry sources are empty: %+v", telemetry)
	}
}
