package connection

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestClassifyPortIgnoresDebugAndBluetooth(t *testing.T) {
	tests := []string{"/dev/cu.Bluetooth-Incoming-Port", "/dev/cu.debug-console"}
	for _, name := range tests {
		candidate, _ := classifyPort(name)
		if candidate {
			t.Fatalf("classifyPort(%q) candidate = true, want false", name)
		}
	}
}

func TestClientHandshakeWithFakeFC(t *testing.T) {
	fc := fakefc.New()
	client, err := NewClient(fc, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	target, err := client.Handshake(context.Background())
	if err != nil {
		t.Fatalf("Handshake() error = %v", err)
	}
	if target.Variant != "BTFL" || target.FirmwareVersion != "2025.12.1" || target.MSPAPIVersion != "1.48" {
		t.Fatalf("unexpected target: %+v", target)
	}
}

func TestClientExecCLIWithFakeFC(t *testing.T) {
	fc := fakefc.New()
	client, err := NewClient(fc, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	lines, err := client.ExecCLI(context.Background(), "version")
	if err != nil {
		t.Fatalf("ExecCLI() error = %v", err)
	}
	if len(lines) == 0 || lines[0] == "" {
		t.Fatalf("ExecCLI() lines = %+v", lines)
	}
}

func TestClientRequestUnsupportedWithFakeFC(t *testing.T) {
	fc := fakefc.New()
	fc.Unsupported[msp.MSPStatusEx] = true
	client, err := NewClient(fc, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	_, err = client.Request(context.Background(), msp.MSPStatusEx, nil)
	if err == nil {
		t.Fatal("Request() error = nil, want unsupported error")
	}
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "unsupported_msp" {
		t.Fatalf("Request() error = %T %v, want unsupported_msp", err, err)
	}
}
