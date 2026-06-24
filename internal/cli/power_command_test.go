package cli

import (
	"context"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/fakefc"
)

func TestSaveRefusalDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"save"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	var ee exitError
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if _, ok := err.(exitError); !ok {
		t.Fatalf("command error = %T %v, want exitError", err, err)
	}
	_ = ee
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called for refused save")
	}
}

func TestRebootRefusalDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"reboot", "firmware"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want non-zero exit")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called for refused reboot")
	}
}

func TestRebootFirmwareWithFakeFC(t *testing.T) {
	var operation connection.OperationClass
	env, err := runTestCommand(t, []string{"reboot", "firmware", "--yes"}, func(ctx context.Context, cfg connection.Config, op connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		operation = op
		client, err := connection.NewClient(fakefc.New(), time.Second)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target, err := client.Handshake(ctx)
		if err != nil {
			return nil, connection.TargetInfo{}, err
		}
		target.Port = "fake"
		return client, target, nil
	})
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if operation != connection.Dangerous {
		t.Fatalf("operation = %v, want Dangerous", operation)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	reboot := data["reboot"].(map[string]any)
	if reboot["mode_name"] != "firmware" || reboot["acknowledged"] != true {
		t.Fatalf("reboot = %+v", reboot)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "firmware_reboot" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestRebootMSCWithFakeFCReportsReady(t *testing.T) {
	env, err := runTestCommand(t, []string{"reboot", "msc", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	reboot := data["reboot"].(map[string]any)
	if reboot["mode_name"] != "msc" || reboot["msc_ready"] != true {
		t.Fatalf("reboot = %+v", reboot)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "msc_reboot" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}
