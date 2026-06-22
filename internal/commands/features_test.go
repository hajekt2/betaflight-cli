package commands

import "testing"

func TestDecodeFeatureStatus(t *testing.T) {
	payload := appendU32Test(nil, 0x00040488)
	status, err := DecodeFeatureStatus(payload)
	if err != nil {
		t.Fatalf("DecodeFeatureStatus() error = %v", err)
	}
	if status.Mask != 0x00040488 || status.UnknownMask != 0 {
		t.Fatalf("status = %+v", status)
	}
	want := []string{"RX_SERIAL", "GPS", "TELEMETRY", "OSD"}
	if len(status.EnabledNames) != len(want) {
		t.Fatalf("enabled names = %+v", status.EnabledNames)
	}
	for i, name := range want {
		if status.EnabledNames[i] != name {
			t.Fatalf("enabled names = %+v, want %v", status.EnabledNames, want)
		}
	}
	if len(status.Catalog) != len(featureDefinitions) {
		t.Fatalf("catalog length = %d", len(status.Catalog))
	}
}

func TestDecodeFeatureStatusUnknownMask(t *testing.T) {
	status, err := DecodeFeatureStatus(appendU32Test(nil, 1<<31))
	if err != nil {
		t.Fatalf("DecodeFeatureStatus() error = %v", err)
	}
	if status.UnknownMask != 1<<31 || len(status.EnabledNames) != 0 {
		t.Fatalf("status = %+v", status)
	}
}

func TestDecodeFeatureStatusRejectsShortPayload(t *testing.T) {
	if _, err := DecodeFeatureStatus([]byte{1, 2}); err == nil {
		t.Fatal("DecodeFeatureStatus() error = nil, want short payload error")
	}
}
