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

// runWatchTest executes the watch command under a minimal root carrying the
// same silence and flag-error behavior as the real root command, so arg and
// flag validation failures still render a JSON envelope. It returns the raw
// output buffer; use decodeEnvelope for single-envelope runs.
func runWatchTest(t *testing.T, args []string, connect connectFunc) (string, error) {
	t.Helper()
	if connect == nil {
		connect = func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
			return nil, connection.TargetInfo{}, context.DeadlineExceeded
		}
	}
	var buf bytes.Buffer
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
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
	})
	root.AddCommand(a.watchCommand())
	root.SetArgs(args)
	err := root.Execute()
	return buf.String(), err
}

// decodeEnvelope parses a rendered (possibly indented) single-envelope buffer.
func decodeEnvelope(t *testing.T, raw string) output.Envelope {
	t.Helper()
	var env output.Envelope
	if err := json.Unmarshal([]byte(strings.TrimSpace(raw)), &env); err != nil {
		t.Fatalf("invalid JSON %q: %v", raw, err)
	}
	return env
}

func TestWatchCommandUnknownProfile(t *testing.T) {
	called := false
	raw, err := runWatchTest(t, []string{"watch", "bogus"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		t.Fatal("watch must not connect for an unknown profile")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("err = %v, want exit error or nil", err)
	}
	env := decodeEnvelope(t, raw)
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connection attempted despite validation failure")
	}
}

func TestWatchCommandMissingProfileArg(t *testing.T) {
	called := false
	raw, err := runWatchTest(t, []string{"watch"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		t.Fatal("watch must not connect without a profile argument")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("err = %v, want exit error or nil", err)
	}
	env := decodeEnvelope(t, raw)
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connection attempted despite missing argument")
	}
}

func TestWatchCommandInvalidIntervalFlag(t *testing.T) {
	called := false
	raw, err := runWatchTest(t, []string{"watch", "telemetry", "--interval", "not-a-duration"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		t.Fatal("watch must not connect when flag parsing fails")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("err = %v, want exit error or nil", err)
	}
	env := decodeEnvelope(t, raw)
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connection attempted despite invalid flag")
	}
}

func TestWatchCommandConnectionFailureEnvelope(t *testing.T) {
	raw, err := runWatchTest(t, []string{"watch", "telemetry", "--count", "1"}, nil)
	if err != nil && !isExitError(err) {
		t.Fatalf("err = %v, want exit error or nil", err)
	}
	env := decodeEnvelope(t, raw)
	if env.OK || len(env.Errors) == 0 {
		t.Fatalf("unexpected envelope: %+v", env)
	}
}

// TestWatchCommandNDJSONStream runs against the fake FC and asserts that each
// sample lands as one compact, independently decodable JSON envelope line with
// a 1-based seq in data and no extra nesting level around the payload.
func TestWatchCommandNDJSONStream(t *testing.T) {
	fakeConnect := func(_ context.Context, _ connection.Config, class connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		if class != connection.ReadOnly {
			t.Fatalf("class = %v, want ReadOnly", class)
		}
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

	raw, err := runWatchTest(t, []string{"watch", "telemetry", "--count", "2", "--interval", "50ms"}, fakeConnect)
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d NDJSON lines (%q), want 2", len(lines), raw)
	}
	for i, line := range lines {
		var envelope struct {
			SchemaVersion string         `json:"schema_version"`
			OK            bool           `json:"ok"`
			Command       string         `json:"command"`
			Data          map[string]any `json:"data"`
		}
		if err := json.Unmarshal([]byte(line), &envelope); err != nil {
			t.Fatalf("line %d %q is not valid JSON: %v", i, line, err)
		}
		if !envelope.OK || !strings.HasSuffix(envelope.Command, "watch") {
			t.Fatalf("line %d envelope = ok:%v command:%q", i, envelope.OK, envelope.Command)
		}
		seq, ok := envelope.Data["seq"].(float64)
		if !ok {
			t.Fatalf("line %d missing data.seq: %s", i, line)
		}
		if int(seq) != i+1 {
			t.Fatalf("line %d data.seq = %v, want %d", i, seq, i+1)
		}
		telemetry, ok := envelope.Data["telemetry"].(map[string]any)
		if !ok {
			t.Fatalf("line %d missing data.telemetry object: %s", i, line)
		}
		if _, nested := telemetry["telemetry"]; nested {
			t.Fatalf("line %d data.telemetry must not be double-nested: %s", i, line)
		}
	}
}

// TestWatchCommandSampleErrorLine verifies that a failing sample emits an
// ok=false failure envelope line while the stream keeps going. The fake FC
// lacks GPS payloads, so a gps watch exercises the failure path on every
// sample while remaining valid NDJSON.
func TestWatchCommandSampleErrorLine(t *testing.T) {
	raw, err := runWatchTest(t, []string{"watch", "gps", "--count", "2", "--interval", "50ms"},
		func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
			client, cerr := connection.NewClient(fakefc.New(), time.Second)
			if cerr != nil {
				return nil, connection.TargetInfo{}, cerr
			}
			target, herr := client.Handshake(context.Background())
			if herr != nil {
				return nil, connection.TargetInfo{}, herr
			}
			target.Port = "fake"
			return client, target, nil
		})
	if err != nil {
		t.Fatalf("Execute = %v", err)
	}

	lines := strings.Split(strings.TrimRight(raw, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("got %d NDJSON lines (%q), want 2 (errors must not abort the stream)", len(lines), raw)
	}
	for i, line := range lines {
		var envelope struct {
			OK     bool `json:"ok"`
			Errors []struct {
				Code string `json:"code"`
			} `json:"errors"`
			Data map[string]any `json:"data"`
		}
		if jsonErr := json.Unmarshal([]byte(line), &envelope); jsonErr != nil {
			t.Fatalf("line %d %q is not valid JSON: %v", i, line, jsonErr)
		}
		seq, _ := envelope.Data["seq"].(float64)
		if int(seq) != i+1 {
			t.Fatalf("line %d data.seq = %v, want %d", i, seq, i+1)
		}
		if envelope.OK {
			continue
		}
		if len(envelope.Errors) == 0 || envelope.Errors[0].Code != "sample_error" {
			t.Fatalf("line %d lacks sample_error code: %s", i, line)
		}
	}
}
