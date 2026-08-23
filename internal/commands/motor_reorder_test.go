package commands

import (
	"strings"
	"testing"

	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestValidateMotorReordering(t *testing.T) {
	tests := []struct {
		name       string
		order      []uint8
		motorCount int
		wantErr    bool
	}{
		{
			name:       "full permutation accepted",
			order:      []uint8{0, 1, 3, 2},
			motorCount: 4,
		},
		{
			name:       "full permutation of maximum motors accepted",
			order:      []uint8{7, 6, 5, 4, 3, 2, 1, 0},
			motorCount: int(MaxSupportedMotors),
		},
		{
			name:       "partial prefix padding to identity tail accepted",
			order:      []uint8{1, 0},
			motorCount: int(MaxSupportedMotors),
		},
		{
			name:       "single entry colliding with identity padding rejected",
			order:      []uint8{1},
			motorCount: int(MaxSupportedMotors),
			wantErr:    true,
		},
		{
			name:       "partial order whose identity tail duplicates supplied value rejected",
			order:      []uint8{0, 1, 3},
			motorCount: 4,
			wantErr:    true,
		},
		{
			name:       "duplicate in supplied entries rejected",
			order:      []uint8{0, 1, 1},
			motorCount: 4,
			wantErr:    true,
		},
		{
			name:       "duplicate at start and end rejected",
			order:      []uint8{2, 0, 1, 2},
			motorCount: 4,
			wantErr:    true,
		},
		{
			name:       "empty order rejected",
			order:      nil,
			motorCount: 4,
			wantErr:    true,
		},
		{
			name:       "out-of-range target rejected",
			order:      []uint8{0, 1, 4},
			motorCount: 4,
			wantErr:    true,
		},
		{
			name:       "too many entries rejected",
			order:      []uint8{0, 1, 2, 3},
			motorCount: 3,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateMotorReordering(tt.order, tt.motorCount)
			if (err != nil) != tt.wantErr {
				t.Fatalf("ValidateMotorReordering(%v, %d) error = %v, wantErr = %v", tt.order, tt.motorCount, err, tt.wantErr)
			}
			if tt.wantErr && len(tt.order) > 0 && len(tt.order) < tt.motorCount && !strings.Contains(err.Error(), "supply the full") && isPaddingCollision(tt.order, tt.motorCount) {
				t.Fatalf("ValidateMotorReordering(%v, %d) error = %q, want supply-the-full-mapping hint", tt.order, tt.motorCount, err)
			}
		})
	}
}

// isPaddingCollision reports whether the identity-padded map for a partial
// order collides because of an identity-tail entry (duplicate first seen at a
// padded position), i.e. the rejection must carry the full-mapping hint.
// Duplicates wholly inside the supplied prefix are reported as plain
// duplicates instead.
func isPaddingCollision(order []uint8, motorCount int) bool {
	padded := make([]uint8, motorCount)
	copy(padded, order)
	for i := len(order); i < motorCount; i++ {
		padded[i] = uint8(i)
	}
	seen := make(map[uint8]bool, motorCount)
	for i, value := range padded {
		if seen[value] {
			return i >= len(order)
		}
		seen[value] = true
	}
	return false
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
