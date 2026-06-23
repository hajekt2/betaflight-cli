package commands

import "testing"

func TestDecodeDebugValues(t *testing.T) {
	payload := appendS16Test(nil, -1)
	payload = appendS16Test(payload, 2)
	payload = appendS16Test(payload, -3)
	values, err := DecodeDebugValues(payload)
	if err != nil {
		t.Fatalf("DecodeDebugValues() error = %v", err)
	}
	if len(values) != 3 || values[0] != -1 || values[2] != -3 {
		t.Fatalf("values = %+v", values)
	}
}

func TestDecodeDebugValuesRejectsOddPayload(t *testing.T) {
	if _, err := DecodeDebugValues([]byte{1}); err == nil {
		t.Fatal("DecodeDebugValues() error = nil, want odd payload error")
	}
}

func TestDecodeAccelerometerTrim(t *testing.T) {
	payload := appendS16Test(nil, -12)
	payload = appendS16Test(payload, 34)
	trim, err := DecodeAccelerometerTrim(payload)
	if err != nil {
		t.Fatalf("DecodeAccelerometerTrim() error = %v", err)
	}
	if trim.Pitch != -12 || trim.Roll != 34 {
		t.Fatalf("trim = %+v", trim)
	}
}

func TestDecodeAccelerometerTrimRejectsShortPayload(t *testing.T) {
	if _, err := DecodeAccelerometerTrim([]byte{1, 2}); err == nil {
		t.Fatal("DecodeAccelerometerTrim() error = nil, want short payload error")
	}
}
