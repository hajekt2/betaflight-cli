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

func TestClientRequestRespectsContextCancel(t *testing.T) {
	client, err := NewClient(&silentPort{}, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = client.Request(ctx, msp.MSPAPIVersion, nil)
	if err == nil {
		t.Fatal("Request() error = nil, want context cancellation")
	}
	coded, ok := err.(*CodedError)
	if !ok {
		t.Fatalf("Request() error = %T %v, want *CodedError", err, err)
	}
	if got, want := coded.Code, "context_canceled"; got != want {
		t.Fatalf("Request() code = %s, want %s", got, want)
	}
}

func TestCheckSupportedFirmwareVersion(t *testing.T) {
	t.Run("supportedVersions", func(t *testing.T) {
		for _, version := range []string{"2025.12.0", "2025.12.1", "2026.0.0", "2027.3.2"} {
			if !isSupportedFirmwareVersion(version) {
				t.Fatalf("isSupportedFirmwareVersion(%q) = false, want true", version)
			}
		}
	})

	t.Run("unsupportedVersions", func(t *testing.T) {
		for _, version := range []string{"2025.11.0", "2024.99.0", "foo", "", "abc.def"} {
			if isSupportedFirmwareVersion(version) {
				t.Fatalf("isSupportedFirmwareVersion(%q) = true, want false", version)
			}
		}
	})
}

func TestClientExecCLIRespectsContextDeadline(t *testing.T) {
	client, err := NewClient(&silentPort{}, time.Second)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	_, err = client.ExecCLI(ctx, "status")
	if err == nil {
		t.Fatal("ExecCLI() error = nil, want context timeout")
	}
	if coded, ok := err.(*CodedError); !ok {
		t.Fatalf("ExecCLI() error = %T %v, want *CodedError", err, err)
	} else if coded.Code != "context_deadline_exceeded" && coded.Code != "context_canceled" {
		t.Fatalf("ExecCLI() code = %s, want context deadline or cancel code", coded.Code)
	}
}

type silentPort struct {
	timeout time.Duration
}

func (s *silentPort) Read(p []byte) (int, error) {
	if s.timeout <= 0 {
		return 0, nil
	}
	time.Sleep(s.timeout)
	return 0, nil
}

func (s *silentPort) Write(p []byte) (int, error) {
	return len(p), nil
}

func (s *silentPort) ResetInputBuffer() error {
	return nil
}

func (s *silentPort) ResetOutputBuffer() error {
	return nil
}

func (s *silentPort) SetReadTimeout(timeout time.Duration) error {
	s.timeout = timeout
	return nil
}

func (s *silentPort) Close() error {
	return nil
}
