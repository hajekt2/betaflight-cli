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

func TestClassifyPortFollowsPlatformConventions(t *testing.T) {
	tests := []struct {
		name          string
		goos          string
		port          string
		wantCandidate bool
	}{
		{"linux acm device", "linux", "/dev/ttyACM0", true},
		{"linux rejects mac path", "linux", "/dev/cu.usbmodem1234", false},
		{"generic usb fallback", "freebsd", "/dev/serial-usb", true},
		{"mac usb modem", "darwin", "/dev/cu.usbmodem1234", true},
		{"windows com", "windows", "COM3", true},
		{"generic serial", "linux", "/dev/ttyS0", false},
	}

	for _, tt := range tests {
		candidate, _ := classifyPortForOS(tt.port, tt.goos)
		if candidate != tt.wantCandidate {
			t.Fatalf("classifyPort(%s) = %v, want %v", tt.name, candidate, tt.wantCandidate)
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
		for _, version := range []string{"2025.12.0", "2025.12.1", "2026.0.0", "2027.3.2", "2025.12.0-rc.1"} {
			if !isSupportedFirmwareVersion(version) {
				t.Fatalf("isSupportedFirmwareVersion(%q) = false, want true", version)
			}
		}
	})

	t.Run("unsupportedVersions", func(t *testing.T) {
		for _, version := range []string{"2025.11.0", "2024.99.0", "foo", "", "abc.def", "2025.-1.0"} {
			if isSupportedFirmwareVersion(version) {
				t.Fatalf("isSupportedFirmwareVersion(%q) = true, want false", version)
			}
		}
	})
}

func TestCheckSupportedRequiresExplicitOverride(t *testing.T) {
	target := TargetInfo{
		Variant:         "BTFL",
		FirmwareVersion: "2025.11.0",
		MSPAPIVersion:   "1.48",
	}
	err := checkSupported(target, false)
	if err == nil {
		t.Fatal("checkSupported() error = nil, want unsupported firmware")
	}
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "unsupported_firmware" {
		t.Fatalf("checkSupported() error = %T %v, want unsupported_firmware", err, err)
	}
	if err := checkSupported(target, true); err != nil {
		t.Fatalf("checkSupported(..., allow=true) error = %v", err)
	}
}

func TestCheckSupportedRejectsNonBetaflightVariant(t *testing.T) {
	target := TargetInfo{Variant: "INAV", FirmwareVersion: "2025.12.1", MSPAPIVersion: "1.48"}
	if err := checkSupported(target, false); err == nil {
		t.Fatal("checkSupported() error = nil, want unsupported firmware")
	}
	if err := checkSupported(target, true); err != nil {
		t.Fatalf("checkSupported(..., allow=true) error = %v", err)
	}
}

func TestConnectWriteRequiresPortWhenAutoPortDisabled(t *testing.T) {
	_, _, err := Connect(context.Background(), Config{AutoPort: false}, Write)
	if err == nil {
		t.Fatal("Connect() error = nil, want auto_port_required")
	}
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "auto_port_required" {
		t.Fatalf("Connect() error = %T %v, want auto_port_required", err, err)
	}
}

func TestConnectReadRequiresPortWhenAutoPortDisabled(t *testing.T) {
	_, _, err := Connect(context.Background(), Config{AutoPort: false}, ReadOnly)
	if err == nil {
		t.Fatal("Connect() error = nil, want auto_port_required")
	}
	coded, ok := err.(*CodedError)
	if !ok || coded.Code != "auto_port_required" {
		t.Fatalf("Connect() error = %T %v, want auto_port_required", err, err)
	}
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
