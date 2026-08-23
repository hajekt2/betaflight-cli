package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

// Waypoint mission domain.
//
// Upstream derivation (Betaflight tag 2026.6.1):
//   - src/main/pg/flight_plan.h:26-28 defines MAX_WAYPOINTS as 30.
//   - src/main/pg/flight_plan.h:44-52 defines the waypoint_t layout:
//     latitude int32 (degrees * 1e7), longitude int32 (degrees * 1e7),
//     altitude int32 (centimeters AMSL, negatives permitted),
//     speed uint16 (cm/s), duration uint16 (deciseconds),
//     type uint8 (waypointType_e), pattern uint8 (waypointPattern_e).
//   - src/main/msp/msp.c at 2026.6.1 contains NO case MSP_WP / case MSP_SET_WP
//     handlers; only legacy #defines remain (src/main/msp/msp_protocol.h:186
//     MSP_WP=118 and :243 MSP_SET_WP=209 with stale INAV-era comments). The raw
//     MSP frames therefore cannot work against real firmware, so this domain is
//     implemented over the firmware CLI text interface instead:
//     cliWaypoint in src/main/cli/cli.c.
//   - src/main/cli/cli.c:2575-2579 waypointTypeNames:
//     FLYOVER FLYBY HOLD LAND TAKEOFF ALT_CHANGE DELAY YAW_RATE.
//   - src/main/cli/cli.c:2581-2584 waypointPatternNames: NONE ORBIT FIGURE8.
//   - src/main/cli/cli.c:2703-2776 printWaypoint emits a bare "waypoint clear"
//     line followed by one "waypoint insert %u %s %s %d %u %s %u %s" line per
//     stored waypoint (index, latitude, longitude, altitude, speed, type name,
//     duration, pattern name) - this exact syntax replays through cliWaypoint.
//   - src/main/cli/cli.c:2682-2701 formatDecimalCoordinate prints coordinates
//     as "%s%d.%07d" (sign, integer degrees, exactly 7 fractional digits).
//   - src/main/cli/cli.c:2825-2829 "waypoint clear" sets waypointCount = 0;
//     inserts shift existing waypoints up (cli.c:3055-3062), so replaying
//     clear + insert lines from index 0 rebuilds the whole mission.

// MaxWaypoints mirrors MAX_WAYPOINTS from src/main/pg/flight_plan.h:26-28 at
// tag 2026.6.1.
const MaxWaypoints = 30

// WaypointTypeNames mirrors waypointTypeNames from src/main/cli/cli.c:2575-2579
// (order matches waypointType_e in src/main/pg/flight_plan.h:31-41).
var WaypointTypeNames = []string{
	"FLYOVER", "FLYBY", "HOLD", "LAND", "TAKEOFF",
	"ALT_CHANGE", "DELAY", "YAW_RATE",
}

// WaypointPatternNames mirrors waypointPatternNames from
// src/main/cli/cli.c:2581-2584 (order matches waypointPattern_e in
// src/main/pg/flight_plan.h:43-47).
var WaypointPatternNames = []string{
	"NONE", "ORBIT", "FIGURE8",
}

// Latitude limits stored as degrees * 1e7, mirroring the range checks in
// src/main/cli/cli.c around line 3035-3042.
const (
	maxLatitudeE7  int64 = 900000000
	maxLongitudeE7 int64 = 1800000000
)

type Waypoint struct {
	Number      uint8  `json:"number"`
	Type        string `json:"type"`
	Pattern     string `json:"pattern"`
	LatitudeE7  int64  `json:"latitude_e7"`
	LongitudeE7 int64  `json:"longitude_e7"`
	AltitudeCM  int32  `json:"altitude_cm"`
	SpeedCMS    uint16 `json:"speed_cm_s"`
	DurationDS  uint16 `json:"duration_ds"`
}

type WaypointList struct {
	Source    string     `json:"source"`
	RawLines  []string   `json:"raw_lines"`
	Waypoints []Waypoint `json:"waypoints"`
	Unparsed  []string   `json:"unparsed,omitempty"`
}

// WaypointTypeName returns the human-readable name for a waypointType_e value,
// falling back to the decimal number for values outside the known table
// (upstream prints UNKNOWN for out-of-range values; keeping the number here
// preserves information for agents inspecting exotic firmware output).
func WaypointTypeName(t uint8) string {
	if int(t) < len(WaypointTypeNames) {
		return WaypointTypeNames[t]
	}
	return strconv.Itoa(int(t))
}

// WaypointPatternName returns the human-readable name for a waypointPattern_e
// value, falling back to the decimal number outside the known table.
func WaypointPatternName(p uint8) string {
	if int(p) < len(WaypointPatternNames) {
		return WaypointPatternNames[p]
	}
	return strconv.Itoa(int(p))
}

// ParseWaypointType resolves a case-insensitive type name (or decimal number)
// to a waypointType_e value, mirroring the strcasecmp lookup in cliWaypoint
// (src/main/cli/cli.c around line 3010-3023).
func ParseWaypointType(s string) (uint8, error) {
	for i, name := range WaypointTypeNames {
		if strings.EqualFold(s, name) {
			return uint8(i), nil
		}
	}
	return 0, fmt.Errorf("invalid type %q; use one of: %s", s, strings.Join(WaypointTypeNames, ", "))
}

// ParseWaypointPattern resolves a case-insensitive pattern name (or decimal
// number) to a waypointPattern_e value, mirroring the lookup in cliWaypoint.
func ParseWaypointPattern(s string) (uint8, error) {
	for i, name := range WaypointPatternNames {
		if strings.EqualFold(s, name) {
			return uint8(i), nil
		}
	}
	return 0, fmt.Errorf("invalid pattern %q; use one of: %s", s, strings.Join(WaypointPatternNames, ", "))
}

// ParseDecimalCoordinate parses a decimal coordinate string to degrees * 1e7,
// mirroring parseDecimalCoordinate from src/main/cli/cli.c:2588-2645. Accepts
// forms like "-33.5429890", "151.6664560", "-33.5" and "151"; more than 7
// fractional digits are rejected because the firmware stores exactly 7.
func ParseDecimalCoordinate(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty coordinate")
	}
	negative := false
	switch s[0] {
	case '-':
		negative = true
		s = s[1:]
	case '+':
		s = s[1:]
	}
	intPart := s
	fracPart := ""
	if idx := strings.IndexByte(s, '.'); idx >= 0 {
		intPart, fracPart = s[:idx], s[idx+1:]
	}
	if intPart == "" && fracPart == "" {
		return 0, fmt.Errorf("invalid coordinate format")
	}
	var value int64
	for _, c := range intPart {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid coordinate format")
		}
		value = value*10 + int64(c-'0')
	}
	digits := 0
	for _, c := range fracPart {
		if c < '0' || c > '9' {
			return 0, fmt.Errorf("invalid coordinate format")
		}
		if digits == 7 {
			return 0, fmt.Errorf("too many decimal places; firmware stores 7")
		}
		value = value*10 + int64(c-'0')
		digits++
	}
	for digits < 7 {
		value *= 10
		digits++
	}
	if negative {
		value = -value
	}
	return value, nil
}

// FormatCoordinate renders degrees * 1e7 using the same "%s%d.%07d" shape as
// formatDecimalCoordinate (src/main/cli/cli.c:2682-2701), including the
// INT32_MIN special case behaviour for extreme negative values.
func FormatCoordinate(e7 int64) string {
	negative := e7 < 0
	abs := e7
	if negative {
		abs = -e7
	}
	sign := ""
	if negative {
		sign = "-"
	}
	return fmt.Sprintf("%s%d.%07d", sign, abs/10000000, abs%10000000)
}

// FormatWaypointInsertLine renders one "waypoint insert ..." CLI line in the
// exact argument order of printWaypoint's format string (src/main/cli/cli.c
// :2705): index, latitude, longitude, altitude, speed, type, duration, pattern.
func FormatWaypointInsertLine(w Waypoint) string {
	return fmt.Sprintf("waypoint insert %d %s %s %d %d %s %d %s",
		w.Number,
		FormatCoordinate(w.LatitudeE7),
		FormatCoordinate(w.LongitudeE7),
		w.AltitudeCM,
		w.SpeedCMS,
		w.Type,
		w.DurationDS,
		w.Pattern,
	)
}

// DecodeWaypointList parses printWaypoint output lines (a bare "waypoint
// clear" marker plus "waypoint insert ..." rows) into typed waypoints. Lines
// that do not match are collected under Unparsed so callers can surface or
// degrade gracefully when the firmware lacks ENABLE_FLIGHT_PLAN support.
func DecodeWaypointList(lines []string) *WaypointList {
	list := &WaypointList{
		Source:    "CLI waypoint list",
		RawLines:  lines,
		Waypoints: []Waypoint{},
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || trimmed == "#" || strings.HasPrefix(trimmed, "# ") || trimmed == "waypoint clear" {
			continue
		}
		fields := strings.Fields(trimmed)
		// "waypoint insert INDEX LAT LON ALT SPEED TYPE DURATION PATTERN"
		if len(fields) != 10 || !strings.EqualFold(fields[0], "waypoint") || !strings.EqualFold(fields[1], "insert") {
			list.Unparsed = append(list.Unparsed, line)
			continue
		}
		number, err := strconv.ParseUint(fields[2], 10, 8)
		if err != nil {
			list.Unparsed = append(list.Unparsed, line)
			continue
		}
		lat, err := ParseDecimalCoordinate(fields[3])
		if err != nil {
			list.Unparsed = append(list.Unparsed, line)
			continue
		}
		lon, err := ParseDecimalCoordinate(fields[4])
		if err != nil {
			list.Unparsed = append(list.Unparsed, line)
			continue
		}
		alt, err := strconv.ParseInt(fields[5], 10, 32)
		if err != nil {
			list.Unparsed = append(list.Unparsed, line)
			continue
		}
		speed, err := strconv.ParseUint(fields[6], 10, 16)
		if err != nil {
			list.Unparsed = append(list.Unparsed, line)
			continue
		}
		duration, err := strconv.ParseUint(fields[8], 10, 16)
		if err != nil {
			list.Unparsed = append(list.Unparsed, line)
			continue
		}
		list.Waypoints = append(list.Waypoints, Waypoint{
			Number:      uint8(number),
			Type:        fields[7],
			Pattern:     fields[9],
			LatitudeE7:  lat,
			LongitudeE7: lon,
			AltitudeCM:  int32(alt),
			SpeedCMS:    uint16(speed),
			DurationDS:  uint16(duration),
		})
	}
	return list
}

// ListWaypoints reads the full mission via the framed non-interactive CLI
// ("waypoint list"), which upstream answers with printWaypoint output.
// Warnings report degraded results, such as firmware builds without
// ENABLE_FLIGHT_PLAN where the command is not recognized.
func ListWaypoints(ctx context.Context, client *connection.Client) (*WaypointList, []string, error) {
	lines, err := client.ExecCLI(ctx, "waypoint list")
	if err != nil {
		return nil, nil, fmt.Errorf("waypoint list failed: %w", err)
	}
	list := DecodeWaypointList(lines)
	warnings := []string{}
	if len(list.Waypoints) == 0 && len(list.Unparsed) > 0 {
		warnings = append(warnings, "flight controller returned no recognizable 'waypoint' output; the flight plan feature may be unavailable on this target")
	}
	return list, warnings, nil
}

// GetWaypoint returns the single waypoint stored at index by listing the
// mission (the firmware CLI has no single-waypoint read verb).
func GetWaypoint(ctx context.Context, client *connection.Client, index uint8) (*Waypoint, []string, error) {
	list, warnings, err := ListWaypoints(ctx, client)
	if err != nil {
		return nil, warnings, err
	}
	for i := range list.Waypoints {
		if list.Waypoints[i].Number == index {
			wp := list.Waypoints[i]
			return &wp, warnings, nil
		}
	}
	return nil, warnings, fmt.Errorf("waypoint %d not found; %d waypoint(s) stored", index, len(list.Waypoints))
}

// ValidateWaypoints checks a complete mission before planning: at most
// MAX_WAYPOINTS entries with contiguous numbering starting at 0 (insert
// semantics shift rows, so contiguous indices rebuild deterministically),
// recognized type/pattern names, and coordinate ranges matching the firmware
// checks in cliWaypoint (latitude +-90 deg, longitude +-180 deg).
func ValidateWaypoints(points []Waypoint) error {
	if len(points) > MaxWaypoints {
		return fmt.Errorf("at most %d waypoints are supported, got %d", MaxWaypoints, len(points))
	}
	for i, wp := range points {
		if wp.Number != uint8(i) {
			return fmt.Errorf("waypoint numbers must be contiguous starting at 0: entry %d has number %d", i, wp.Number)
		}
	}
	for i := range points {
		wp := &points[i]
		if wp.Pattern == "" {
			// Firmware defaults pattern to waypointPattern_e 0 (NONE).
			wp.Pattern = "NONE"
		}
		if _, err := ParseWaypointType(wp.Type); err != nil {
			return fmt.Errorf("waypoint %d: %v", wp.Number, err)
		}
		if _, err := ParseWaypointPattern(wp.Pattern); err != nil {
			return fmt.Errorf("waypoint %d: %v", wp.Number, err)
		}
		if wp.LatitudeE7 < -maxLatitudeE7 || wp.LatitudeE7 > maxLatitudeE7 {
			return fmt.Errorf("waypoint %d latitude out of range; use -90.0 to 90.0 degrees", wp.Number)
		}
		if wp.LongitudeE7 < -maxLongitudeE7 || wp.LongitudeE7 > maxLongitudeE7 {
			return fmt.Errorf("waypoint %d longitude out of range; use -180.0 to 180.0 degrees", wp.Number)
		}
	}
	return nil
}

// BuildWaypointSetPlan renders the full change plan as firmware CLI lines:
// "waypoint clear" first (so replaying clears persisted waypoints, matching
// printWaypoint's dump ordering, src/main/cli/cli.c:2726-2728) followed by one
// insert per waypoint at its contiguous index.
func BuildWaypointSetPlan(points []Waypoint) ([]string, error) {
	if err := ValidateWaypoints(points); err != nil {
		return nil, err
	}
	lines := make([]string, 0, len(points)+1)
	lines = append(lines, "waypoint clear")
	for _, wp := range points {
		lines = append(lines, FormatWaypointInsertLine(wp))
	}
	return lines, nil
}

// BuildWaypointClearPlan renders the plan that empties the mission; upstream
// implements it by resetting waypointCount to zero (src/main/cli/cli.c
// :2825-2829).
func BuildWaypointClearPlan() []string {
	return []string{"waypoint clear"}
}
