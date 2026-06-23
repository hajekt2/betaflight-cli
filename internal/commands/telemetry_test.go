package commands

import "testing"

func TestDecodeStatusKeepsExtendedModeFlagBytes(t *testing.T) {
	payload := appendU16Test(nil, 250)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 33)
	payload = appendU32Test(payload, 2)
	payload = append(payload, 0)
	payload = appendU16Test(payload, 42)
	payload = append(payload, 4, 0, 4)
	payload = append(payload, 1, 2, 4, 8)
	payload = append(payload, 29)
	payload = appendU32Test(payload, 0x1234)
	status, err := decodeStatus(payload, true)
	if err != nil {
		t.Fatalf("decodeStatus() error = %v", err)
	}
	if status.ModeFlagByteCount == nil || *status.ModeFlagByteCount != 4 {
		t.Fatalf("mode flag byte count = %v", status.ModeFlagByteCount)
	}
	want := []int{2, 0, 0, 0, 1, 2, 4, 8}
	if len(status.ModeFlagsBytes) != len(want) {
		t.Fatalf("mode flag bytes = %+v", status.ModeFlagsBytes)
	}
	for i := range want {
		if status.ModeFlagsBytes[i] != want[i] {
			t.Fatalf("mode flag bytes = %+v, want %+v", status.ModeFlagsBytes, want)
		}
	}
}
