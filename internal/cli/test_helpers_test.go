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
)

func isExitError(err error) bool {
	_, ok := err.(exitError)
	return ok
}

func runTestCommand(t *testing.T, args []string, connect connectFunc) (output.Envelope, error) {
	return runTestCommandWithInput(t, args, "", connect)
}

func runTestCommandWithInput(t *testing.T, args []string, input string, connect connectFunc) (output.Envelope, error) {
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
	root := a.rootCommand()
	root.SetArgs(args)
	err := root.Execute()
	var env output.Envelope
	if decodeErr := json.Unmarshal(buf.Bytes(), &env); decodeErr != nil {
		t.Fatalf("invalid JSON %q: %v", buf.String(), decodeErr)
	}
	return env, err
}
