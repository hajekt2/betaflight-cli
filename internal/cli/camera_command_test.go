package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestCameraKeysOffline(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"camera", "keys"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if called {
		t.Fatal("camera keys connected")
	}
	data := env.Data.(map[string]any)
	keys := data["camera_keys"].([]any)
	if len(keys) != 5 || keys[0].(map[string]any)["name"] != "enter" || keys[4].(map[string]any)["code"] != float64(4) {
		t.Fatalf("camera_keys = %+v", keys)
	}
}

func TestCameraPressPlansWithoutConnecting(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"camera", "press", "right"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if called {
		t.Fatal("camera press plan connected")
	}
	data := env.Data.(map[string]any)
	result := data["camera_control"].(map[string]any)
	key := result["key"].(map[string]any)
	if result["applied"] != false || result["confirmation"] != "--yes" || key["name"] != "right" || key["code"] != float64(3) {
		t.Fatalf("camera_control = %+v", result)
	}
}

func TestCameraPressWithYesSendsMSP(t *testing.T) {
	var fc *fakefc.FC
	var observedOp connection.OperationClass
	env, err := runTestCommand(t, []string{"camera", "press", "down", "--yes"}, func(_ context.Context, _ connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		observedOp = op
		fc = fakefc.New()
		client, clientErr := connection.NewClient(fc, time.Second)
		return client, connection.TargetInfo{}, clientErr
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	if observedOp != connection.Write {
		t.Fatalf("operation = %v", observedOp)
	}
	if len(fc.CameraKeys) != 1 || fc.CameraKeys[0] != 4 {
		t.Fatalf("camera keys sent = %+v", fc.CameraKeys)
	}
	data := env.Data.(map[string]any)
	result := data["camera_control"].(map[string]any)
	if result["applied"] != true || result["acknowledged"] != true || result["save_required"] != false {
		t.Fatalf("camera_control = %+v", result)
	}
}

func TestCameraPressReportsUnsupportedTarget(t *testing.T) {
	env, err := runTestCommand(t, []string{"camera", "press", "enter", "--yes"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		fc := fakefc.New()
		fc.Unsupported[msp.MSPCameraControl] = true
		client, clientErr := connection.NewClient(fc, time.Second)
		return client, connection.TargetInfo{}, clientErr
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	result := data["camera_control"].(map[string]any)
	if result["supported"] != false || result["unsupported_reason"] == "" || result["applied"] != false {
		t.Fatalf("camera_control = %+v", result)
	}
	if len(env.Warnings) != 1 || env.Warnings[0].Code != "unsupported_msp" {
		t.Fatalf("warnings = %+v", env.Warnings)
	}
}

func TestCameraPressRejectsUnknownKeyWithoutConnecting(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"camera", "press", "menu"}, func(_ context.Context, _ connection.Config, _ connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("env = %+v", env)
	}
	if called {
		t.Fatal("invalid camera key connected")
	}
}
