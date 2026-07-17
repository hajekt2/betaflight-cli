package support

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const SupportedFirmwarePolicy = "official Betaflight 2025.12.x and newer"

func EvaluateFirmwareCompatibility(variant, firmwareVersion, apiVersion string) (bool, string) {
	switch {
	case variant == "":
		return false, "missing Betaflight firmware variant"
	case variant != "BTFL":
		return false, "non-Betaflight firmware variant"
	case apiVersion == "":
		return false, "missing MSP API version"
	case !strings.HasPrefix(apiVersion, "1."):
		return false, "unsupported MSP API major version"
	case firmwareVersion == "":
		return false, "missing firmware version"
	case !IsSupportedFirmwareVersion(firmwareVersion):
		return false, "firmware is outside the supported metadata range"
	default:
		return true, "firmware is inside the supported metadata range"
	}
}

func IsSupportedFirmwareVersion(version string) bool {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return false
	}
	year, err := parseLeadingInt(parts[0])
	if err != nil || year < 2025 {
		return false
	}
	if year > 2025 {
		return true
	}
	month, err := parseLeadingInt(parts[1])
	return err == nil && month >= 12
}

func parseLeadingInt(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, fmt.Errorf("empty version component")
	}
	digits := make([]rune, 0, len(raw))
	for _, ch := range raw {
		if unicode.IsDigit(ch) {
			digits = append(digits, ch)
			continue
		}
		break
	}
	if len(digits) == 0 {
		return 0, fmt.Errorf("no digits in %q", raw)
	}
	return strconv.Atoi(string(digits))
}
