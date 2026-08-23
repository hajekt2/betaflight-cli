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

func TestOSDCharSetRejectsInvalidBitmapLengthBeforeConnect(t *testing.T) {
	env, err := runTestCommandWithInput(t, []string{"osd", "char-set", "5", "-", "--yes"}, `{"index":5,"bitmap":[0,1,2]}`, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("connector should not be called for invalid bitmap length")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK || env.Errors[0].Code != "validation_error" {
		t.Fatalf("envelope = %+v", env)
	}
}

func TestOSDCharGetFailsWithoutHandler(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "char-get", "5"}, nil)
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK {
		t.Fatalf("char-get should fail when the read fails, envelope = %+v", env)
	}
	if len(env.Errors) == 0 || env.Errors[0].Code == "" {
		t.Fatalf("expected coded error, envelope = %+v", env)
	}
	data, ok := env.Data.(map[string]any)
	if ok {
		if _, present := data["osd_char"]; present {
			t.Fatalf("failed read must not carry osd_char, envelope = %+v", env)
		}
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

func TestOSDVideoConfigReadFailsWithoutHandler(t *testing.T) {
	env, err := runTestCommand(t, []string{"osd", "video-config"}, nil)
	if err != nil && !isExitError(err) {
		t.Fatalf("command error = %v", err)
	}
	if env.OK {
		t.Fatalf("video-config should fail when the read fails, envelope = %+v", env)
	}
	if len(env.Errors) == 0 || env.Errors[0].Code == "" {
		t.Fatalf("expected coded error, envelope = %+v", env)
	}
	data, ok := env.Data.(map[string]any)
	if ok {
		if _, present := data["osd_video_config"]; present {
			t.Fatalf("failed read must not carry osd_video_config, envelope = %+v", env)
		}
	}
}
