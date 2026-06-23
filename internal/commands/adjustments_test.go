package commands

import "testing"

func TestDecodeAdjustmentRangesModern(t *testing.T) {
	payload := []byte{0, 0, 0, 16, 12, 0}
	payload = appendU16Test(payload, 1500)
	payload = appendU16Test(payload, 100)
	payload = append(payload, 0, 1, 4, 8, 33, 2)
	payload = appendU16Test(payload, 1600)
	payload = appendU16Test(payload, 50)
	ranges, err := DecodeAdjustmentRanges(payload)
	if err != nil {
		t.Fatalf("DecodeAdjustmentRanges() error = %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("ranges = %+v", ranges)
	}
	if ranges[0].RangeStartUS != 900 || ranges[0].RangeEndUS != 1300 || ranges[0].AdjustmentFunctionName != "RATE_PROFILE" || !ranges[0].Active {
		t.Fatalf("range 0 = %+v", ranges[0])
	}
	if ranges[1].AuxChannelName != "AUX2" || ranges[1].AuxSwitchChannelName != "AUX3" || ranges[1].AdjustmentFunctionName != "BATTERY_PROFILE" {
		t.Fatalf("range 1 = %+v", ranges[1])
	}
	if ranges[1].CLICommand != "adjrange 1 0 1 1000 1100 33 2 1600 50" {
		t.Fatalf("cli command = %q", ranges[1].CLICommand)
	}
}

func TestDecodeAdjustmentRangesLegacy(t *testing.T) {
	ranges, err := DecodeAdjustmentRanges([]byte{0, 0, 0, 16, 12, 0})
	if err != nil {
		t.Fatalf("DecodeAdjustmentRanges() error = %v", err)
	}
	if len(ranges) != 1 {
		t.Fatalf("ranges = %+v", ranges)
	}
	if ranges[0].AdjustmentCenter != 0 || ranges[0].AdjustmentScale != 0 || ranges[0].CLICommand != "adjrange 0 0 0 900 1300 12 0 0 0" {
		t.Fatalf("range = %+v", ranges[0])
	}
}

func TestDecodeAdjustmentRangesRejectsPartialRow(t *testing.T) {
	if _, err := DecodeAdjustmentRanges([]byte{0, 1, 2, 3, 4}); err == nil {
		t.Fatal("DecodeAdjustmentRanges() error = nil, want partial row error")
	}
}

func TestEncodeAdjustmentRange(t *testing.T) {
	payload := EncodeAdjustmentRange(AdjustmentRange{
		Index:                 1,
		SlotIndex:             0,
		AuxChannelIndex:       2,
		RangeStartStep:        4,
		RangeEndStep:          8,
		AdjustmentFunction:    33,
		AuxSwitchChannelIndex: 3,
		AdjustmentCenter:      1600,
		AdjustmentScale:       50,
	})
	want := []byte{1, 0, 2, 4, 8, 33, 3, 0x40, 0x06, 0x32, 0}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}
