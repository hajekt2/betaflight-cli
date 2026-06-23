package commands

import "testing"

func TestDecodeBlackboxConfig(t *testing.T) {
	payload := []byte{1, 2, 1, 4, 16, 0, 2, 0x01, 0x10, 0x00, 0x00}
	config, err := DecodeBlackboxConfig(payload)
	if err != nil {
		t.Fatalf("DecodeBlackboxConfig() error = %v", err)
	}
	if !config.Supported || config.Device != 2 || config.DeviceName != "SDCARD" {
		t.Fatalf("config = %+v", config)
	}
	if config.RateNumerator != 1 || config.RateDenominator != 4 || config.PRatio != 16 {
		t.Fatalf("config = %+v", config)
	}
	if config.SampleRate == nil || *config.SampleRate != 2 || config.SampleRateName != "1/4" {
		t.Fatalf("config = %+v", config)
	}
	if config.FieldsDisabledMask == nil || *config.FieldsDisabledMask != 0x1001 {
		t.Fatalf("config = %+v", config)
	}
	if len(config.DisabledFields) != 2 || config.DisabledFields[0] != "PID" || config.DisabledFields[1] != "GPS" {
		t.Fatalf("disabled = %+v", config.DisabledFields)
	}
	if len(config.EnabledFields) != len(blackboxSelectableFields)-2 {
		t.Fatalf("enabled = %+v", config.EnabledFields)
	}
}

func TestDecodeBlackboxConfigLegacyPayload(t *testing.T) {
	payload := []byte{0, 0, 0, 0, 0, 0}
	config, err := DecodeBlackboxConfig(payload)
	if err != nil {
		t.Fatalf("DecodeBlackboxConfig() error = %v", err)
	}
	if config.Supported || config.SampleRate != nil || config.FieldsDisabledMask != nil {
		t.Fatalf("config = %+v", config)
	}
}

func TestEncodeBlackboxConfig(t *testing.T) {
	sampleRate := uint8(2)
	mask := uint32(0x1001)
	payload := EncodeBlackboxConfig(BlackboxConfig{
		Device:             2,
		RateNumerator:      1,
		RateDenominator:    4,
		PRatio:             16,
		SampleRate:         &sampleRate,
		FieldsDisabledMask: &mask,
	})
	want := []byte{2, 1, 4, 16, 0, 2, 0x01, 0x10, 0x00, 0x00}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}
