package commands

import "testing"

func TestEvaluateFirmwareSupport(t *testing.T) {
	tests := []struct {
		name      string
		variant   string
		version   string
		api       string
		supported bool
		reason    string
	}{
		{
			name:      "supported 2025.12",
			variant:   "BTFL",
			version:   "2025.12.1",
			api:       "1.48",
			supported: true,
			reason:    "firmware is inside the supported metadata range",
		},
		{
			name:      "supported later calver",
			variant:   "BTFL",
			version:   "2026.1.0",
			api:       "1.49",
			supported: true,
			reason:    "firmware is inside the supported metadata range",
		},
		{
			name:    "old firmware",
			variant: "BTFL",
			version: "4.5.0",
			api:     "1.46",
			reason:  "firmware is outside the supported metadata range",
		},
		{
			name:    "wrong variant",
			variant: "INAV",
			version: "2025.12.1",
			api:     "1.48",
			reason:  "non-Betaflight firmware variant",
		},
		{
			name:    "wrong api major",
			variant: "BTFL",
			version: "2025.12.1",
			api:     "2.0",
			reason:  "unsupported MSP API major version",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := EvaluateFirmwareSupport(tt.variant, tt.version, tt.api)
			if got.Supported != tt.supported || got.Reason != tt.reason {
				t.Fatalf("support = %+v", got)
			}
			if got.Policy == "" || got.AllowUnsupportedFlag != "--allow-unsupported" {
				t.Fatalf("support metadata = %+v", got)
			}
		})
	}
}
