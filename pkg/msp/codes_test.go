package msp

import (
	"bytes"
	"testing"
)

func TestIsLikelyWriteCodeFallsBackForUnknownMSP2Direction(t *testing.T) {
	for _, code := range []uint16{MSP2CommonSetSerialConfig, MSP2BetaflightBind, MSP2SetMotorOutputReordering, MSP2SendDshotCommand, MSP2SetText, MSP2SetLedStripConfigValues, MSP2SetBatteryProfile} {
		if !IsLikelyWriteCode(code) {
			t.Fatalf("IsLikelyWriteCode(%#x) = false, want true", code)
		}
	}
	if IsLikelyWriteCode(MSP2GetText) {
		t.Fatal("MSP2_GET_TEXT classified as write")
	}
}

func TestIsLikelyWriteCodeWithCapturedMSP2Frames(t *testing.T) {
	frames := []struct {
		name      string
		wire      []byte
		wantWrite bool
	}{
		{name: "MSP2_SET_TEXT", wire: []byte{'$', 'X', '<', 0, 0x07, 0x30, 0, 0, 0x25}, wantWrite: true},
		{name: "MSP2_GET_TEXT", wire: []byte{'$', 'X', '<', 0, 0x06, 0x30, 0, 0, 0x60}, wantWrite: false},
	}
	for _, tt := range frames {
		t.Run(tt.name, func(t *testing.T) {
			frame, err := ReadFrame(bytes.NewReader(tt.wire))
			if err != nil {
				t.Fatalf("ReadFrame() error = %v", err)
			}
			if got := IsLikelyWriteCode(frame.Code); got != tt.wantWrite {
				t.Fatalf("IsLikelyWriteCode(%#x) = %v, want %v", frame.Code, got, tt.wantWrite)
			}
		})
	}
}

func TestLookupCommandByName(t *testing.T) {
	meta, ok := LookupCommandByName("msp_name")
	if !ok {
		t.Fatalf("lookup failed")
	}
	if meta.Code != MSPName || meta.Name != "MSP_NAME" {
		t.Fatalf("meta = %+v", meta)
	}
	if _, ok := LookupCommandByName("does_not_exist"); ok {
		t.Fatal("unexpected lookup success")
	}
}

func TestForwardCommandsRecordExactUpstreamSource(t *testing.T) {
	for _, code := range []uint16{MSPAttitudeQuaternion, MSP2BatteryProfile, MSP2SetBatteryProfile, MSP2CLISetting, MSP2CLISettingInfo} {
		meta, ok := LookupCommand(code)
		if !ok {
			t.Fatalf("LookupCommand(%#x) missing", code)
		}
		if meta.SourceVersion != ForwardMSPSourceVersion {
			t.Fatalf("LookupCommand(%#x).SourceVersion = %q, want %q", code, meta.SourceVersion, ForwardMSPSourceVersion)
		}
	}
}

func TestListCommandsSortedByCode(t *testing.T) {
	commands := ListCommands()
	if len(commands) == 0 {
		t.Fatal("expected commands")
	}
	for i := 1; i < len(commands); i++ {
		if commands[i-1].Code > commands[i].Code {
			t.Fatalf("commands not sorted by code: index %d code %d > index %d code %d", i-1, commands[i-1].Code, i, commands[i].Code)
		}
	}
}
