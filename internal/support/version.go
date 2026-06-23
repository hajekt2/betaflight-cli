package support

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

const SupportedFirmwarePolicy = "official Betaflight 2025.12.x and newer"

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
