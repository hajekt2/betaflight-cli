package fakefc

import (
	"bytes"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestFCRespondsToMSPAPIVersion(t *testing.T) {
	fc := New()
	if _, err := fc.Write(msp.EncodeRequest(msp.MSPAPIVersion, nil)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	frame := readResponseFrame(t, fc)
	if frame.Version != 1 || frame.Direction != '>' || frame.Code != msp.MSPAPIVersion {
		t.Fatalf("frame = %+v", frame)
	}
	if !bytes.Equal(frame.Payload, []byte{0, 1, 48}) {
		t.Fatalf("payload = %v", frame.Payload)
	}
}

func TestFCMarksConfiguredUnsupportedMSP(t *testing.T) {
	fc := New()
	fc.Unsupported[msp.MSPFCVariant] = true
	if _, err := fc.Write(msp.EncodeRequest(msp.MSPFCVariant, nil)); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	frame := readResponseFrame(t, fc)
	if frame.Direction != '!' || !frame.Unsupported || frame.Code != msp.MSPFCVariant {
		t.Fatalf("frame = %+v", frame)
	}
}

func TestFCHandlesFramedCLICommands(t *testing.T) {
	fc := New()
	if _, err := fc.Write([]byte{0x02, 'v', 'e', 'r', 's', 'i', 'o', 'n', 0x03}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	got := readCLIResponse(t, fc)
	if !strings.HasPrefix(got, "\x02# Betaflight") || !strings.HasSuffix(got, "\x03") {
		t.Fatalf("CLI response = %q", got)
	}
	if !strings.Contains(got, "MSP API: 1.48") {
		t.Fatalf("CLI response = %q", got)
	}
}

func TestFCSaveCanCloseTransport(t *testing.T) {
	fc := New()
	fc.SaveCloses = true
	if _, err := fc.Write([]byte{0x02, 's', 'a', 'v', 'e', 0x03}); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := fc.Write(msp.EncodeRequest(msp.MSPAPIVersion, nil)); err != io.ErrClosedPipe {
		t.Fatalf("Write() error = %v, want %v", err, io.ErrClosedPipe)
	}
}

func TestFCReadTimeoutReturnsNoBytes(t *testing.T) {
	fc := New()
	if err := fc.SetReadTimeout(time.Millisecond); err != nil {
		t.Fatalf("SetReadTimeout() error = %v", err)
	}
	buf := make([]byte, 16)
	n, err := fc.Read(buf)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if n != 0 {
		t.Fatalf("Read() = %d bytes, want 0", n)
	}
}

func readResponseFrame(t *testing.T, fc *FC) msp.Frame {
	t.Helper()
	if err := fc.SetReadTimeout(time.Second); err != nil {
		t.Fatalf("SetReadTimeout() error = %v", err)
	}
	frame, err := msp.ReadFrame(fc)
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	return frame
}

func readCLIResponse(t *testing.T, fc *FC) string {
	t.Helper()
	if err := fc.SetReadTimeout(time.Second); err != nil {
		t.Fatalf("SetReadTimeout() error = %v", err)
	}
	var out strings.Builder
	buf := make([]byte, 32)
	for {
		n, err := fc.Read(buf)
		if err != nil {
			t.Fatalf("Read() error = %v", err)
		}
		if n == 0 {
			t.Fatal("Read() timed out before CLI response terminator")
		}
		out.Write(buf[:n])
		if strings.Contains(out.String(), "\x03") {
			return out.String()
		}
	}
}
