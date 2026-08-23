package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

// waypointCLIConnect returns a connector backed by a fake FC whose CLI map is
// preloaded with canned responses for the waypoint commands, plus the slice of
// operation classes observed by the connector.
func waypointCLIConnect(t *testing.T, cli map[string][]string) (connectFunc, *[]connection.OperationClass) {
	t.Helper()
	var seenOps []connection.OperationClass
	fc := fakefc.New()
	for command, lines := range cli {
		fc.CLI[command] = lines
	}
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOps = append(seenOps, op)
		client, err := connection.NewClient(fc, time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(context.Background())
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	}
	return connect, &seenOps
}

var waypointListCLIFixture = []string{
	"waypoint clear",
	"waypoint insert 0 -33.5429890 151.6664560 10000 1500 FLYOVER 0 NONE",
	"waypoint insert 1 -33.5000000 151.6000000 5000 1200 HOLD 100 ORBIT",
}

// runWaypointTestCommand mounts a.waypointCommand() under a minimal parent
// because registration into rootCommand is owned by the orchestrator.
func runWaypointTestCommand(t *testing.T, args []string, input string, connect connectFunc) (output.Envelope, error) {
	t.Helper()
	var buf bytes.Buffer
	if connect == nil {
		connect = func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
			client, err := connection.NewClient(fakefc.New(), time.Second)
			if err != nil {
				return nil, connection.TargetInfo{}, err
			}
			target, err := client.Handshake(context.Background())
			if err != nil {
				return nil, connection.TargetInfo{}, err
			}
			target.Port = "fake"
			return client, target, nil
		}
	}
	a := &app{
		build:   BuildInfo{Version: "test", Commit: "test", Date: "test"},
		out:     &buf,
		in:      strings.NewReader(input),
		connect: connect,
	}
	root := &cobra.Command{
		Use:           "betaflight-cli",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&a.opts.port, "port", "", "USB serial port")
	root.PersistentFlags().BoolVar(&a.opts.autoPort, "auto-port", true, "auto-select port")
	root.PersistentFlags().BoolVar(&a.opts.yes, "yes", false, "confirm write action")
	root.AddCommand(a.waypointCommand())
	root.SetArgs(args)
	err := root.Execute()
	var env output.Envelope
	if decodeErr := json.Unmarshal(buf.Bytes(), &env); decodeErr != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), decodeErr)
	}
	return env, err
}

// requireOK fails when a command that should succeed returned an error other
// than the envelope exit for ok=false results (ADR-0019).
func requireOK(t *testing.T, err error) {
	t.Helper()
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
}

func planLines(value any) []string {
	raw, ok := value.([]any)
	if !ok {
		return nil
	}
	lines := make([]string, 0, len(raw))
	for _, item := range raw {
		lines = append(lines, item.(string))
	}
	return lines
}

func TestWPListParsesFirmwareOutput(t *testing.T) {
	connect, _ := waypointCLIConnect(t, map[string][]string{
		"waypoint list": waypointListCLIFixture,
	})
	env, err := runWaypointTestCommand(t, []string{"wp", "list"}, "", connect)
	requireOK(t, err)
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	list := data["waypoints"].(map[string]any)
	points := list["waypoints"].([]any)
	if len(points) != 2 {
		t.Fatalf("waypoints = %+v", points)
	}
	first := points[0].(map[string]any)
	if first["number"] != float64(0) || first["type"] != "FLYOVER" || first["latitude_e7"] != float64(-335429890) || first["altitude_cm"] != float64(10000) {
		t.Fatalf("first = %+v", first)
	}
	second := points[1].(map[string]any)
	if second["pattern"] != "ORBIT" || second["duration_ds"] != float64(100) {
		t.Fatalf("second = %+v", second)
	}
	if len(env.Warnings) != 0 {
		t.Fatalf("warnings = %+v", env.Warnings)
	}
}

func TestWPListDegradesToWarningWithoutFlightPlanSupport(t *testing.T) {
	// Plain fakefc answers "unknown command: waypoint list"; the read must
	// still succeed with a warning instead of failing.
	connect, _ := waypointCLIConnect(t, nil)
	env, err := runWaypointTestCommand(t, []string{"wp", "list"}, "", connect)
	requireOK(t, err)
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if len(env.Warnings) == 0 {
		t.Fatal("expected degradation warning")
	}
}

func TestWPGetReturnsSingleWaypoint(t *testing.T) {
	connect, _ := waypointCLIConnect(t, map[string][]string{
		"waypoint list": waypointListCLIFixture,
	})
	env, err := runWaypointTestCommand(t, []string{"wp", "get", "1"}, "", connect)
	requireOK(t, err)
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	wp := env.Data.(map[string]any)["waypoint"].(map[string]any)
	if wp["number"] != float64(1) || wp["type"] != "HOLD" || wp["speed_cm_s"] != float64(1200) {
		t.Fatalf("waypoint = %+v", wp)
	}
}

func TestWPGetRejectsUnknownIndex(t *testing.T) {
	connect, _ := waypointCLIConnect(t, map[string][]string{
		"waypoint list": waypointListCLIFixture,
	})
	env, err := runWaypointTestCommand(t, []string{"wp", "get", "5"}, "", connect)
	requireOK(t, err)
	if env.OK || len(env.Errors) == 0 {
		t.Fatalf("env = %+v", env)
	}
}

func TestWPSetJSONPlansOfflineWithoutConnecting(t *testing.T) {
	called := false
	connect := func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	}
	input := `{"waypoints":[
		{"number":0,"type":"FLYOVER","latitude_degrees":"-33.5429890","longitude_degrees":"151.6664560","altitude_cm":10000,"speed_cm_s":1500},
		{"number":1,"type":"HOLD","pattern":"ORBIT","latitude_e7":-335000000,"longitude_e7":1516000000,"altitude_cm":5000,"duration_ds":100}
	]}`
	env, err := runWaypointTestCommand(t, []string{"wp", "set-json", "-"}, input, connect)
	requireOK(t, err)
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if called {
		t.Fatal("planning connected to the flight controller")
	}
	plan := env.Data.(map[string]any)["change_plan"].(map[string]any)
	if plan["applied"] != false || plan["kind"] != "waypoints" {
		t.Fatalf("plan = %+v", plan)
	}
	wantLines := []string{
		"waypoint clear",
		"waypoint insert 0 -33.5429890 151.6664560 10000 1500 FLYOVER 0 NONE",
		"waypoint insert 1 -33.5000000 151.6000000 5000 0 HOLD 100 ORBIT",
	}
	gotLines := planLines(plan["cli_lines"])
	if gotLines == nil || !reflect.DeepEqual(gotLines, wantLines) {
		t.Fatalf("cli_lines = %#v, want %#v", plan["cli_lines"], wantLines)
	}
}

func TestWPSetJSONApplyRequiresYes(t *testing.T) {
	input := `{"waypoints":[{"number":0,"type":"LAND","latitude_e7":-335429890,"longitude_e7":1516664560}]}`
	env, err := runWaypointTestCommand(t, []string{"wp", "set-json", "-", "--apply"}, input, nil)
	requireOK(t, err)
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env.Errors)
	}
}

func TestWPSetJSONAppliesViaWrite(t *testing.T) {
	var seenOp connection.OperationClass
	fc := fakefc.New()
	connect := func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		seenOp = op
		client, err := connection.NewClient(fc, time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(context.Background())
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	}
	input := `{"waypoints":[{"number":0,"type":"TAKEOFF","latitude_e7":-335429890,"longitude_e7":1516664560,"altitude_cm":5000}]}`
	env, err := runWaypointTestCommand(t, []string{"wp", "set-json", "-", "--apply", "--yes"}, input, connect)
	requireOK(t, err)
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if seenOp != connection.Write {
		t.Fatalf("operation = %v, want Write", seenOp)
	}
	plan := env.Data.(map[string]any)["change_plan"].(map[string]any)
	if plan["applied"] != true {
		t.Fatalf("plan = %+v", plan)
	}
	appliedLines := planLines(plan["applied_cli_lines"])
	if len(appliedLines) != 2 || appliedLines[0] != "waypoint clear" {
		t.Fatalf("applied_cli_lines = %+v", plan["applied_cli_lines"])
	}
	if len(env.SideEffects) != 2 || env.SideEffects[0].Type != "cli_command" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestWPSetJSONValidationErrors(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "empty mission", input: `{"waypoints":[]}`},
		{name: "bare empty array", input: `[]`},
		{name: "missing coordinates", input: `{"waypoints":[{"number":0,"type":"FLYOVER"}]}`},
		{name: "unknown type", input: `{"waypoints":[{"number":0,"type":"LOITER","latitude_e7":0,"longitude_e7":0}]}`},
		{name: "non-contiguous numbering", input: `{"waypoints":[
			{"number":0,"type":"FLYOVER","latitude_e7":0,"longitude_e7":0},
			{"number":2,"type":"FLYOVER","latitude_e7":0,"longitude_e7":0}]}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			called := false
			connect := func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				called = true
				return nil, connection.TargetInfo{}, nil
			}
			env, err := runWaypointTestCommand(t, []string{"wp", "set-json", "-"}, tc.input, connect)
			requireOK(t, err)
			if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
				t.Fatalf("env = %+v", env.Errors)
			}
			if called {
				t.Fatal("validation failure connected to the flight controller")
			}
		})
	}
}

func TestWPSetJSONEmptyMissionNamesWPClear(t *testing.T) {
	for _, input := range []string{`[]`, `{"waypoints":[]}`} {
		env, err := runWaypointTestCommand(t, []string{"wp", "set-json", "-"}, input, nil)
		requireOK(t, err)
		if env.OK || len(env.Errors) != 1 || !strings.Contains(env.Errors[0].Message, "use 'wp clear' to remove the mission") {
			t.Fatalf("input %s: env = %+v", input, env.Errors)
		}
	}
}

func TestWPClearPlansAndRequiresConfirmationToApply(t *testing.T) {
	connect, seenOps := waypointCLIConnect(t, nil)

	env, err := runWaypointTestCommand(t, []string{"wp", "clear"}, "", connect)
	requireOK(t, err)
	if !env.OK {
		t.Fatalf("plan env.OK = false: %+v", env.Errors)
	}
	plan := env.Data.(map[string]any)["change_plan"].(map[string]any)
	if plan["applied"] != false || !reflect.DeepEqual(planLines(plan["cli_lines"]), []string{"waypoint clear"}) {
		t.Fatalf("plan = %+v", plan)
	}

	env, err = runWaypointTestCommand(t, []string{"wp", "clear", "--apply"}, "", connect)
	requireOK(t, err)
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env.Errors)
	}

	env, err = runWaypointTestCommand(t, []string{"wp", "clear", "--apply", "--yes"}, "", connect)
	requireOK(t, err)
	if !env.OK {
		t.Fatalf("apply env.OK = false: %+v", env.Errors)
	}
	if len(*seenOps) == 0 || (*seenOps)[len(*seenOps)-1] != connection.Dangerous {
		t.Fatalf("operations = %+v, want last Dangerous", *seenOps)
	}
	plan = env.Data.(map[string]any)["change_plan"].(map[string]any)
	if plan["applied"] != true {
		t.Fatalf("plan = %+v", plan)
	}
}
