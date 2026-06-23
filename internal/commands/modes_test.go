package commands

import "testing"

func TestDecodeBoxNames(t *testing.T) {
	names := DecodeBoxNames([]byte("ARM;ANGLE;BEEPER MUTE;"))
	if len(names) != 3 || names[0] != "ARM" || names[2] != "BEEPER MUTE" {
		t.Fatalf("names = %+v", names)
	}
}

func TestDecodeModeRanges(t *testing.T) {
	payload := []byte{0, 0, 32, 48, 52, 2, 16, 32}
	ranges, err := DecodeModeRanges(payload)
	if err != nil {
		t.Fatalf("DecodeModeRanges() error = %v", err)
	}
	if len(ranges) != 2 {
		t.Fatalf("ranges = %+v", ranges)
	}
	if ranges[0].ID != 0 || ranges[0].AuxChannelName != "AUX1" || ranges[0].Range.StartUS != 1700 || ranges[0].Range.EndUS != 2100 {
		t.Fatalf("range 0 = %+v", ranges[0])
	}
	if ranges[1].ID != 52 || ranges[1].AuxChannelIndex != 2 || ranges[1].Range.StartUS != 1300 || ranges[1].Range.EndUS != 1700 {
		t.Fatalf("range 1 = %+v", ranges[1])
	}
}

func TestDecodeModeRangesRejectsPartialItem(t *testing.T) {
	_, err := DecodeModeRanges([]byte{0, 0, 32})
	if err == nil {
		t.Fatal("DecodeModeRanges() error = nil, want error")
	}
}

func TestDecodeModeRangeExtras(t *testing.T) {
	extras, err := DecodeModeRangeExtras([]byte{2, 0, 0, 0, 52, 1, 53})
	if err != nil {
		t.Fatalf("DecodeModeRangeExtras() error = %v", err)
	}
	if len(extras) != 2 || extras[1].ID != 52 || extras[1].Logic != 1 || extras[1].LinkedTo != 53 || !extras[1].HasLinked {
		t.Fatalf("extras = %+v", extras)
	}
}

func TestEncodeModeRange(t *testing.T) {
	payload := EncodeModeRange(ModeRange{
		Index:           1,
		ID:              52,
		AuxChannelIndex: 2,
		Range: StepRange{
			StartStep: 16,
			EndStep:   32,
		},
	})
	want := []byte{1, 52, 2, 16, 32}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestEncodeModeRangeWithExtras(t *testing.T) {
	logic := uint8(1)
	linkedTo := uint8(53)
	payload := EncodeModeRange(ModeRange{
		Index:           1,
		ID:              52,
		AuxChannelIndex: 2,
		Range: StepRange{
			StartStep: 16,
			EndStep:   32,
		},
		ModeLogic: &logic,
		LinkedTo:  &linkedTo,
	})
	want := []byte{1, 52, 2, 16, 32, 1, 53}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}
