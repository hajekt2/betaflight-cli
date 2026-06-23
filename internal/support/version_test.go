package support

import "testing"

func TestIsSupportedFirmwareVersion(t *testing.T) {
	tests := []struct {
		name      string
		version   string
		supported bool
	}{
		{"2025.12.0", true},
		{"2025.12.0-rc.1", true},
		{"2025.12.5", true},
		{"2026.1.0", true},
		{"2024.12.0", false},
		{"2025.11.9", false},
		{"abc", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsSupportedFirmwareVersion(tt.version); got != tt.supported {
				t.Fatalf("IsSupportedFirmwareVersion(%q) = %v, want %v", tt.version, got, tt.supported)
			}
		})
	}
}

func TestSupportedFirmwarePolicy(t *testing.T) {
	if SupportedFirmwarePolicy == "" {
		t.Fatal("SupportedFirmwarePolicy must be set")
	}
}
