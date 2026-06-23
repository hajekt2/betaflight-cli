package commands

import "testing"

func TestDecodeMixerStatus(t *testing.T) {
	status, err := DecodeMixerStatus([]byte{3, 1})
	if err != nil {
		t.Fatalf("DecodeMixerStatus() error = %v", err)
	}
	if status.Mixer.ID != 3 || status.Mixer.CLIName != "QUADX" || status.Mixer.DisplayName != "Quad X" || status.Mixer.MotorCount != 4 {
		t.Fatalf("mixer = %+v", status.Mixer)
	}
	if status.YawMotorsReversed == nil || !*status.YawMotorsReversed || status.ReverseMotorDirRaw == nil || *status.ReverseMotorDirRaw != 1 {
		t.Fatalf("reverse = %+v raw=%+v", status.YawMotorsReversed, status.ReverseMotorDirRaw)
	}
	if len(status.CLICommands) != 2 || status.CLICommands[0] != "mixer QUADX" || status.CLICommands[1] != "set yaw_motors_reversed = ON" {
		t.Fatalf("commands = %+v", status.CLICommands)
	}
	if len(status.Catalog) != 27 {
		t.Fatalf("catalog length = %d", len(status.Catalog))
	}
}

func TestDecodeMixerStatusAcceptsLegacySingleBytePayload(t *testing.T) {
	status, err := DecodeMixerStatus([]byte{1})
	if err != nil {
		t.Fatalf("DecodeMixerStatus() error = %v", err)
	}
	if status.Mixer.CLIName != "TRI" || status.YawMotorsReversed != nil {
		t.Fatalf("status = %+v", status)
	}
}

func TestDecodeMixerStatusReportsUnknownMode(t *testing.T) {
	status, err := DecodeMixerStatus([]byte{99, 0})
	if err != nil {
		t.Fatalf("DecodeMixerStatus() error = %v", err)
	}
	if status.Mixer.Known || len(status.Warnings) != 1 {
		t.Fatalf("status = %+v", status)
	}
}

func TestEncodeMixerConfig(t *testing.T) {
	payload := EncodeMixerConfig(MixerConfig{Mode: 3, YawMotorsReversed: true})
	want := []byte{3, 1}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestDecodeMixerStatusRejectsEmptyPayload(t *testing.T) {
	if _, err := DecodeMixerStatus(nil); err == nil {
		t.Fatal("DecodeMixerStatus() error = nil, want short payload error")
	}
}
