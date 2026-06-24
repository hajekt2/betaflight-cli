package cli

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestTextStatusWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"text", "status"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	text := data["text"].(map[string]any)
	if text["source"] != "MSP2_GET_TEXT" {
		t.Fatalf("text = %+v", text)
	}
	byKey := text["by_key"].(map[string]any)
	if byKey["craft_name"] != "BetaFlight" || byKey["pilot_name"] != "BF pilot" || byKey["release_name"] != "2025.12.1" {
		t.Fatalf("by_key = %+v", byKey)
	}
	fields := text["fields"].([]any)
	if len(fields) != 7 {
		t.Fatalf("fields = %+v", fields)
	}
}

func TestTextSetRejectsReadOnlyFieldDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"text", "set", "build_key", "abc", "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after read-only text field validation failure")
	}
}

func TestTextSetRequiresYesDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"text", "set", "craft_name", "Quad"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after text set confirmation failure")
	}
}

func TestTextSetJSONRejectsReadOnlyFieldDoesNotConnect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "text.json")
	if err := os.WriteFile(path, []byte(`{"field":"build_key","value":"abc"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	called := false
	env, err := runTestCommand(t, []string{"text", "set-json", path, "--yes"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want validation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "validation_error" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after read-only text field validation failure")
	}
}

func TestTextSetJSONRequiresYesDoesNotConnect(t *testing.T) {
	path := filepath.Join(t.TempDir(), "text.json")
	if err := os.WriteFile(path, []byte(`{"field":"craft_name","value":"Quad"}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	called := false
	env, err := runTestCommand(t, []string{"text", "set-json", path}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		called = true
		return nil, connection.TargetInfo{}, nil
	})
	if err == nil {
		t.Fatal("command error = nil, want confirmation failure")
	}
	if env.OK || len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected envelope: %+v", env)
	}
	if called {
		t.Fatal("connector was called after text set confirmation failure")
	}
}

func TestTextSetWithFakeFC(t *testing.T) {
	env, err := runTestCommand(t, []string{"text", "set", "craft_name", "Quad", "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	text := data["text"].(map[string]any)
	request := text["request"].(map[string]any)
	if request["key"] != "craft_name" || request["value"] != "Quad" || text["msp_name"] != "MSP2_SET_TEXT" || text["save_required"] != true {
		t.Fatalf("text set = %+v", text)
	}
	if len(env.SideEffects) != 1 || env.SideEffects[0].Type != "text_set" || env.SideEffects[0].Command != "MSP2_SET_TEXT" {
		t.Fatalf("side effects = %+v", env.SideEffects)
	}
}

func TestTextSetJSONWithFakeFC(t *testing.T) {
	path := filepath.Join(t.TempDir(), "text.json")
	if err := os.WriteFile(path, []byte(`{"request":{"key":"craft_name","value":"Quad JSON"}}`), 0o600); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	env, err := runTestCommand(t, []string{"text", "set-json", path, "--yes"}, nil)
	if err != nil {
		t.Fatalf("command error = %v", err)
	}
	if !env.OK {
		t.Fatalf("env.OK = false: %+v", env.Errors)
	}
	data := env.Data.(map[string]any)
	text := data["text"].(map[string]any)
	request := text["request"].(map[string]any)
	if request["key"] != "craft_name" || request["value"] != "Quad JSON" || text["msp_name"] != "MSP2_SET_TEXT" || text["save_required"] != true {
		t.Fatalf("text set = %+v", text)
	}
}
