package commands

import (
	"reflect"
	"strings"
	"testing"
)

// waypointListFixture mirrors real printWaypoint output (src/main/cli/cli.c
// :2703-2776): a bare "waypoint clear" line followed by one
// "waypoint insert %u %s %s %d %u %s %u %s" line per stored waypoint, with
// coordinates rendered by formatDecimalCoordinate as "%s%d.%07d".
func waypointListFixture() []string {
	return []string{
		"waypoint clear",
		"waypoint insert 0 -33.5429890 151.6664560 10000 1500 FLYOVER 0 NONE",
		"waypoint insert 1 -33.5000000 151.6000000 5000 1200 HOLD 100 ORBIT",
		"waypoint insert 2 46.5196535 6.6322734 -250 900 LAND 0 NONE",
	}
}

func TestParseDecimalCoordinate(t *testing.T) {
	cases := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{in: "-33.5429890", want: -335429890},
		{in: "151.6664560", want: 1516664560},
		{in: "-90", want: -900000000},
		{in: "180.0", want: 1800000000},
		{in: "0.0000000", want: 0},
		{in: "151", want: 1510000000},
		{in: "-33.54298905", wantErr: true}, // more than 7 fractional digits
		{in: "", wantErr: true},             // empty
		{in: "-", wantErr: true},            // sign only
		{in: "abc", wantErr: true},          // non-numeric
		{in: "1.2.3", wantErr: true},        // malformed
	}
	for _, tc := range cases {
		got, err := ParseDecimalCoordinate(tc.in)
		if tc.wantErr {
			if err == nil {
				t.Fatalf("ParseDecimalCoordinate(%q) = %d, want error", tc.in, got)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseDecimalCoordinate(%q) error = %v", tc.in, err)
		}
		if got != tc.want {
			t.Fatalf("ParseDecimalCoordinate(%q) = %d, want %d", tc.in, got, tc.want)
		}
		if back := FormatCoordinate(got); !strings.HasSuffix(back, ".") && len(strings.Split(back, ".")[1]) != 7 {
			t.Fatalf("FormatCoordinate(%d) = %q, want 7 fractional digits", got, back)
		}
	}
}

func TestFormatCoordinateMatchesFirmwareShape(t *testing.T) {
	// formatDecimalCoordinate prints "%s%d.%07d" (cli.c:2700).
	if got := FormatCoordinate(-335429890); got != "-33.5429890" {
		t.Fatalf("FormatCoordinate(-335429890) = %q", got)
	}
	if got := FormatCoordinate(66322734); got != "6.6322734" {
		t.Fatalf("FormatCoordinate(66322734) = %q", got)
	}
	if got := FormatCoordinate(0); got != "0.0000000" {
		t.Fatalf("FormatCoordinate(0) = %q", got)
	}
}

func TestDecodeWaypointList(t *testing.T) {
	list := DecodeWaypointList(waypointListFixture())
	if len(list.Unparsed) != 0 {
		t.Fatalf("unparsed = %+v", list.Unparsed)
	}
	if len(list.Waypoints) != 3 {
		t.Fatalf("waypoints = %+v", list.Waypoints)
	}
	first := list.Waypoints[0]
	if first.Number != 0 || first.Type != "FLYOVER" || first.Pattern != "NONE" ||
		first.LatitudeE7 != -335429890 || first.LongitudeE7 != 1516664560 ||
		first.AltitudeCM != 10000 || first.SpeedCMS != 1500 || first.DurationDS != 0 {
		t.Fatalf("first = %+v", first)
	}
	third := list.Waypoints[2]
	if third.AltitudeCM != -250 || third.Type != "LAND" || third.LatitudeE7 != 465196535 || third.LongitudeE7 != 66322734 {
		t.Fatalf("third = %+v", third)
	}
}

func TestDecodeWaypointListCollectsUnrecognizedLines(t *testing.T) {
	lines := append([]string{
		"# Betaflight / STM32F405 2026.6.1",
		"",
	}, waypointListFixture()...)
	lines = append(lines,
		"unknown command: waypoint",
		"###ERROR###",
	)
	list := DecodeWaypointList(lines)
	if len(list.Waypoints) != 3 {
		t.Fatalf("waypoints = %+v", list.Waypoints)
	}
	if !reflect.DeepEqual(list.Unparsed, []string{"unknown command: waypoint", "###ERROR###"}) {
		t.Fatalf("unparsed = %+v", list.Unparsed)
	}
}

func TestDecodeWaypointListRejectsMalformedRows(t *testing.T) {
	list := DecodeWaypointList([]string{
		"waypoint insert x -33.5 151.6 100 100 FLYOVER 0 NONE", // bad index
		"waypoint insert 0 lat 151.6 100 100 FLYOVER 0 NONE",   // bad latitude
		"waypoint update 0 -33.5 151.6 100 100 FLYOVER 0 NONE", // wrong verb
	})
	if len(list.Waypoints) != 0 || len(list.Unparsed) != 3 {
		t.Fatalf("waypoints = %+v, unparsed = %+v", list.Waypoints, list.Unparsed)
	}
}

func TestWaypointTypeAndPatternNames(t *testing.T) {
	// Tables must match cli.c name tables exactly (cli.c:2575-2584).
	wantTypes := []string{"FLYOVER", "FLYBY", "HOLD", "LAND", "TAKEOFF", "ALT_CHANGE", "DELAY", "YAW_RATE"}
	wantPatterns := []string{"NONE", "ORBIT", "FIGURE8"}
	if !reflect.DeepEqual(WaypointTypeNames, wantTypes) {
		t.Fatalf("types = %+v", WaypointTypeNames)
	}
	if !reflect.DeepEqual(WaypointPatternNames, wantPatterns) {
		t.Fatalf("patterns = %+v", WaypointPatternNames)
	}
	if got := WaypointTypeName(200); got != "200" {
		t.Fatalf("WaypointTypeName(200) = %q, want numeric fallback", got)
	}
	if typ, err := ParseWaypointType("hold"); err != nil || typ != 2 {
		t.Fatalf("ParseWaypointType(hold) = %d, %v", typ, err)
	}
	if _, err := ParseWaypointType("LOITER"); err == nil {
		t.Fatal("ParseWaypointType(LOITER) accepted")
	}
	if pat, err := ParseWaypointPattern("figure8"); err != nil || pat != 2 {
		t.Fatalf("ParseWaypointPattern(figure8) = %d, %v", pat, err)
	}
}

func TestBuildWaypointSetPlan(t *testing.T) {
	points := []Waypoint{
		{Number: 0, Type: "FLYOVER", Pattern: "NONE", LatitudeE7: -335429890, LongitudeE7: 1516664560, AltitudeCM: 10000, SpeedCMS: 1500},
		{Number: 1, Type: "HOLD", Pattern: "ORBIT", LatitudeE7: -335000000, LongitudeE7: 1516000000, AltitudeCM: 5000, DurationDS: 100},
	}
	lines, err := BuildWaypointSetPlan(points)
	if err != nil {
		t.Fatalf("BuildWaypointSetPlan() error = %v", err)
	}
	want := []string{
		"waypoint clear",
		"waypoint insert 0 -33.5429890 151.6664560 10000 1500 FLYOVER 0 NONE",
		"waypoint insert 1 -33.5000000 151.6000000 5000 0 HOLD 100 ORBIT",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %+v, want %+v", lines, want)
	}
}

func TestBuildWaypointClearPlan(t *testing.T) {
	// Upstream clear sets waypointCount = 0 (cli.c:2825-2829).
	if plan := BuildWaypointClearPlan(); !reflect.DeepEqual(plan, []string{"waypoint clear"}) {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestValidateWaypointsRejectsBadMissions(t *testing.T) {
	tooMany := make([]Waypoint, MaxWaypoints+1)
	for i := range tooMany {
		tooMany[i] = Waypoint{Number: uint8(i), Type: "FLYOVER"}
	}
	if err := ValidateWaypoints(tooMany); err == nil || !strings.Contains(err.Error(), "at most") {
		t.Fatalf("ValidateWaypoints(too many) = %v", err)
	}
	gap := []Waypoint{
		{Number: 0, Type: "FLYOVER", LatitudeE7: 0, LongitudeE7: 0},
		{Number: 2, Type: "FLYOVER", LatitudeE7: 0, LongitudeE7: 0},
	}
	if err := ValidateWaypoints(gap); err == nil || !strings.Contains(err.Error(), "contiguous") {
		t.Fatalf("ValidateWaypoints(gap) = %v", err)
	}
	badLat := []Waypoint{{Number: 0, Type: "FLYOVER", LatitudeE7: 900000001, LongitudeE7: 0}}
	if err := ValidateWaypoints(badLat); err == nil || !strings.Contains(err.Error(), "latitude out of range") {
		t.Fatalf("ValidateWaypoints(latitude) = %v", err)
	}
	badLon := []Waypoint{{Number: 0, Type: "FLYOVER", LatitudeE7: 0, LongitudeE7: -1800000001}}
	if err := ValidateWaypoints(badLon); err == nil || !strings.Contains(err.Error(), "longitude out of range") {
		t.Fatalf("ValidateWaypoints(longitude) = %v", err)
	}
	badType := []Waypoint{{Number: 0, Type: "LOITER", LatitudeE7: 0, LongitudeE7: 0}}
	if err := ValidateWaypoints(badType); err == nil || !strings.Contains(err.Error(), "invalid type") {
		t.Fatalf("ValidateWaypoints(type) = %v", err)
	}
	// Boundary values from the firmware checks must be accepted.
	edge := []Waypoint{{Number: 0, Type: "TAKEOFF", Pattern: "FIGURE8", LatitudeE7: 900000000, LongitudeE7: -1800000000, AltitudeCM: -2147483648}}
	if err := ValidateWaypoints(edge); err != nil {
		t.Fatalf("ValidateWaypoints(edge) = %v", err)
	}
}

func TestValidateWaypointsDoesNotMutateInput(t *testing.T) {
	points := []Waypoint{
		{Number: 0, Type: "FLYOVER", LatitudeE7: -335429890, LongitudeE7: 1516664560},
		{Number: 1, Type: "HOLD", Pattern: "ORBIT", LatitudeE7: 0, LongitudeE7: 0},
	}
	want := append([]Waypoint(nil), points...)
	if err := ValidateWaypoints(points); err != nil {
		t.Fatalf("ValidateWaypoints() error = %v", err)
	}
	if !reflect.DeepEqual(points, want) {
		t.Fatalf("ValidateWaypoints mutated input: got %+v, want %+v", points, want)
	}
}

func TestBuildWaypointSetPlanRejectsEmptyMission(t *testing.T) {
	_, err := BuildWaypointSetPlan(nil)
	if err == nil || !strings.Contains(err.Error(), "use 'wp clear' to remove the mission") {
		t.Fatalf("BuildWaypointSetPlan(empty) = %v", err)
	}
}

func TestBuildWaypointSetPlanDefaultsPatternWithoutMutatingInput(t *testing.T) {
	points := []Waypoint{{Number: 0, Type: "FLYOVER", LatitudeE7: -335429890, LongitudeE7: 1516664560}}
	lines, err := BuildWaypointSetPlan(points)
	if err != nil {
		t.Fatalf("BuildWaypointSetPlan() error = %v", err)
	}
	want := []string{
		"waypoint clear",
		"waypoint insert 0 -33.5429890 151.6664560 0 0 FLYOVER 0 NONE",
	}
	if !reflect.DeepEqual(lines, want) {
		t.Fatalf("lines = %+v, want %+v", lines, want)
	}
	if points[0].Pattern != "" {
		t.Fatalf("BuildWaypointSetPlan mutated input pattern to %q", points[0].Pattern)
	}
}
