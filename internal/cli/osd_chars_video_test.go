package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func osdCharRowsJSON() string {
	var rows []string
	for range 18 {
		row := make([]string, 12)
		for x := range row {
			row[x] = "2"
		}
		rows = append(rows, "["+strings.Join(row, ",")+"]")
	}
	return `{"index":5,"rows":[` + strings.Join(rows, ",") + `]}`
}

func TestOSDCharSetRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"osd", "char-set", "5", "-"}, osdCharRowsJSON(), func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("envelope = %+v", env)
	}
}

func TestOSDCharSetValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"osd", "char-set", "5", "-", "--yes"}, `{"rows":[]}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("envelope = %+v", env)
	}
}

func TestOSDCharGetDegradesToWarningWithoutHandler(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "char-get", "5"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("char-get should degrade to warnings, envelope = %+v", env)
	}
	data := env.Data.(map[string]any)
	warnings, ok := data["warnings"].([]any)
	if !ok || len(warnings) == 0 {
		t.Fatalf("warnings missing from data = %+v", data)
	}
}

func TestOSDVideoConfigRequiresConfirmationBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"osd", "set-video-config-json", "-"}, `{"video_system":2,"units":1}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called without --yes")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("envelope = %+v", env)
	}
}

func TestOSDVideoConfigValidationBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"osd", "set-video-config-json", "-", "--yes"}, `{"video_system":4,"units":1}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid args")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("envelope = %+v", env)
	}
}

func TestOSDVideoConfigReadDegradesToWarningWithoutHandler(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "video-config"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("video-config should degrade to warnings, envelope = %+v", env)
	}
	data := env.Data.(map[string]any)
	if _, ok := data["warnings"].([]any); !ok {
		t.Fatalf("warnings missing from data = %+v", data)
	}
}
