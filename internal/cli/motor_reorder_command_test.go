package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

// runMotorReorderCommand executes the motor-reorder command tree without
// depending on root registration order; it works before and after the
// orchestrator wires motorReorderCommand into rootCommand.
func runMotorReorderCommand(t *testing.T, args []string, input string, connect connectFunc) (output.Envelope, error) {
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
	root := &cobra.Command{Use: "root"}
	root.PersistentFlags().BoolVar(&a.opts.yes, "yes", false, "confirm non-interactive write or dangerous action")
	root.AddCommand(a.motorReorderCommand())
	root.SetArgs(args)
	root.SetIn(strings.NewReader(input))
	err := root.Execute()
	var env output.Envelope
	if decodeErr := json.Unmarshal(buf.Bytes(), &env); decodeErr != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), decodeErr)
	}
	return env, err
}

func TestMotorReorderSetRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runMotorReorderCommand(t, []string{"motor-reorder", "set", "-"}, `{"order":[0,1,3,2]}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestMotorReorderSetValidationBeforeConnect(t *testing.T) {
	cases := []struct {
		name  string
		input string
	}{
		{name: "duplicate targets", input: `{"order":[0,1,1]}`},
		{name: "out of range target", input: `{"order":[0,9]}`},
		{name: "missing order array", input: `{}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env, err := runMotorReorderCommand(t, []string{"motor-reorder", "set", "-", "--yes"}, tc.input, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
				t.Fatal("connector should not be called for invalid input")
				return nil, connection.TargetInfo{}, nil
			})
			if err != nil && !isExitError(err) {
				t.Fatalf("command error = %v", err)
			}
			if env.OK || env.Errors[0].Code != "validation_error" {
				t.Fatalf("env = %+v", env)
			}
		})
	}
}

func TestMotorReorderShowWithFakeFC(t *testing.T) {
	env, err := runMotorReorderCommand(t, []string{"motor-reorder", "show"}, "", nil)
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env = %+v", env)
	}
	result := env.Data.(map[string]any)["motor_output_reordering"].(map[string]any)
	order := result["order"].([]any)
	// fakefc canned read-back: count 4 with order 0,1,2,3.
	if len(order) != 4 || order[0] != float64(0) || order[3] != float64(3) {
		t.Fatalf("motor_output_reordering = %+v", result)
	}
}

func TestMotorReorderSetWithFakeFC(t *testing.T) {
	env, err := runMotorReorderCommand(t, []string{"motor-reorder", "set", "-", "--yes"}, `{"order":[0,1,3,2]}`, nil)
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env = %+v", env)
	}
	result := env.Data.(map[string]any)["motor_reorder"].(map[string]any)
	if result["msp_code"] != float64(0x3002) || result["msp_name"] != "MSP2_SET_MOTOR_OUTPUT_REORDERING" ||
		result["acknowledged"] != true || result["save_required"] != true {
		t.Fatalf("motor_reorder = %+v", result)
	}
	order := result["order"].([]any)
	if len(order) != 4 || order[2] != float64(3) || order[3] != float64(2) {
		t.Fatalf("order = %+v", order)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "motor_reorder" || env.SideEffects[0].Command != "MSP2_SET_MOTOR_OUTPUT_REORDERING" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}
