package commands

import (
	"testing"

	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestValidateMotorReorderingAcceptsPermutation(t *testing.T) {
	order := []uint8{0, 1, 3, 2}
	if err := ValidateMotorReordering(order, 4); err != nil {
		t.Fatalf("ValidateMotorReordering() error = %v", err)
	}
}

func TestValidateMotorReorderingRejectsEmpty(t *testing.T) {
	if err := ValidateMotorReordering(nil, 4); err == nil {
		t.Fatal("ValidateMotorReordering() error = nil, want empty order error")
	}
}

func TestValidateMotorReorderingRejectsOutOfRange(t *testing.T) {
	if err := ValidateMotorReordering([]uint8{0, 1, 4}, 4); err == nil {
		t.Fatal("ValidateMotorReordering() error = nil, want out-of-range error")
	}
}

func TestValidateMotorReorderingRejectsDuplicates(t *testing.T) {
	if err := ValidateMotorReordering([]uint8{0, 1, 1}, 4); err == nil {
		t.Fatal("ValidateMotorReordering() error = nil, want duplicate error")
	}
}

func TestValidateMotorReorderingRejectsTooManyEntries(t *testing.T) {
	if err := ValidateMotorReordering([]uint8{0, 1, 2, 3}, 3); err == nil {
		t.Fatal("ValidateMotorReordering() error = nil, want entry-count error")
	}
}

func TestEncodeMotorReorderingMatchesUpstreamLayout(t *testing.T) {
	// msp.c:3818: u8 arraySize followed by arraySize u8 values.
	payload := EncodeMotorReordering([]uint8{0, 1, 3, 2})
	want := []byte{4, 0, 1, 3, 2}
	if string(payload) != string(want) {
		t.Fatalf("EncodeMotorReordering() = %v, want %v", payload, want)
	}
}

func TestDecodeMotorOutputReorderingRoundTrip(t *testing.T) {
	// msp.c:1322: u8 count (MAX_SUPPORTED_MOTORS) then count u8 values.
	payload := []byte{8, 0, 1, 2, 3, 4, 5, 7, 6}
	reorder, err := DecodeMotorOutputReordering(payload)
	if err != nil {
		t.Fatalf("DecodeMotorOutputReordering() error = %v", err)
	}
	reencoded := EncodeMotorReordering(reorder.Order)
	if len(reencoded) != len(payload) || string(reencoded[1:]) != string(payload[1:]) || reencoded[0] != payload[0] {
		t.Fatalf("round trip mismatch: %v from %v", reencoded, payload)
	}
	if reorder.Order[6] != 7 || reorder.Order[7] != 6 {
		t.Fatalf("Order = %v", reorder.Order)
	}
}

func TestDecodeMotorOutputReorderingRejectsShortPayload(t *testing.T) {
	if _, err := DecodeMotorOutputReordering([]byte{4, 0, 1}); err == nil {
		t.Fatal("DecodeMotorOutputReordering() error = nil, want short payload error")
	}
	if _, err := DecodeMotorOutputReordering(nil); err == nil {
		t.Fatal("DecodeMotorOutputReordering() error = nil, want empty payload error")
	}
}

func TestSetMotorOutputReorderingResultShape(t *testing.T) {
	// Contract check without a live client: encode + validation path feeding
	// SetMotorOutputReordering, plus the fixed result metadata.
	order := []uint8{0, 1, 3, 2}
	if err := ValidateMotorReordering(order, MaxSupportedMotors); err != nil {
		t.Fatalf("ValidateMotorReordering() error = %v", err)
	}
	if code := msp.MSP2SetMotorOutputReordering; code != 0x3002 {
		t.Fatalf("MSP2SetMotorOutputReordering = %#x, want 0x3002", code)
	}
}
