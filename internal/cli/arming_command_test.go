package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

// runArmingTestCommand executes the arming command subtree the same way
// rootCommand does. Registration into root.go is owned by the orchestrator,
// so these tests mount a.armingCommand() under a minimal parent with the
// persistent flags the command depends on.
func runArmingTestCommand(t *testing.T, args []string, connect connectFunc) (output.Envelope, error) {
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
		in:      strings.NewReader(""),
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
	root.AddCommand(a.armingCommand())
	root.SetArgs(args)
	err := root.Execute()
	var env output.Envelope
	if decodeErr := json.Unmarshal(buf.Bytes(), &env); decodeErr != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), decodeErr)
	}
	return env, err
}

func TestArmingStatusWithFakeFC(t *testing.T) {
	env, err := runArmingTestCommand(t, []string{"arming", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	state := data["arming_lock"].(map[string]any)
	// fakefc serves MSP_STATUS_EX with arming-disable flags 0x1234 (bits
	// RXLOSS, BOXFAILSAFE, RUNAWAY, BOOTGRACE, CALIB) and count 29.
	if state["enabled"] != false || state["flags"] != float64(0x1234) || state["count"] != float64(29) {
		t.Fatalf("arming_lock = %+v", state)
	}
	names := state["active_names"].([]any)
	want := []any{"RXLOSS", "BOXFAILSAFE", "RUNAWAY", "BOOTGRACE", "CALIB"}
	if len(names) != len(want) {
		t.Fatalf("active_names = %v, want %v", names, want)
	}
	for i := range want {
		if names[i] != want[i] {
			t.Fatalf("active_names = %v, want %v", names, want)
		}
	}
}

func TestArmingLockRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runArmingTestCommand(t, []string{"arming", "lock"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}

func TestArmingUnlockRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runArmingTestCommand(t, []string{"arming", "unlock"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("env = %+v", env)
	}
}
