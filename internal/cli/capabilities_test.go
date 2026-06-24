package cli

import (
	"strings"
	"testing"
)

func TestCapabilityMetadataUsesKnownOperations(t *testing.T) {
	registry := capabilityMetadataRegistry()
	allowed := map[string]bool{
		"offline":                         true,
		"offline_or_read_only_probe":      true,
		"offline_dangerous_plan":          true,
		"read_only":                       true,
		"read_only_or_write_or_dangerous": true,
		"text_output":                     true,
		"write":                           true,
		"write_when_apply_is_set":         true,
		"plan":                            true,
		"plan_or_write":                   true,
		"plan_or_dangerous_execute":       true,
		"dangerous":                       true,
	}

	for path, meta := range registry {
		if meta.Operation == "" {
			t.Fatalf("capability %q missing operation", path)
		}
		if !allowed[meta.Operation] {
			t.Fatalf("capability %q has unknown operation %q", path, meta.Operation)
		}
	}
}

func TestCapabilityMetadataOfflineCommandClassifications(t *testing.T) {
	registry := capabilityMetadataRegistry()
	for _, tt := range []struct {
		path               string
		requiresConnection bool
		operation          string
	}{
		{"betaflight-cli schema", false, "offline"},
		{"betaflight-cli capabilities coverage", false, "offline"},
		{"betaflight-cli cli diff", true, "read_only"},
		{"betaflight-cli cli dump", true, "read_only"},
		{"betaflight-cli telemetry", true, "read_only"},
		{"betaflight-cli msp list", false, "offline"},
		{"betaflight-cli msp metadata", false, "offline"},
		{"betaflight-cli msp request", true, "read_only_or_write_or_dangerous"},
	} {
		meta, ok := registry[tt.path]
		if !ok {
			t.Fatalf("missing metadata for %q", tt.path)
		}
		if meta.RequiresConnection != tt.requiresConnection {
			t.Fatalf("metadata %q requires_connection = %v, want %v", tt.path, meta.RequiresConnection, tt.requiresConnection)
		}
		if meta.Operation != tt.operation {
			t.Fatalf("metadata %q operation = %q, want %q", tt.path, meta.Operation, tt.operation)
		}
	}
}

func TestCapabilityMetadataOperationSafetyContracts(t *testing.T) {
	registry := capabilityMetadataRegistry()
	yesRequired := map[string]bool{
		"write":                     true,
		"plan_or_write":             true,
		"plan_or_dangerous_execute": true,
		"dangerous":                 true,
		"write_when_apply_is_set":   true,
	}

	offlinedOperations := map[string]bool{
		"offline":                    true,
		"offline_or_read_only_probe": true,
		"offline_dangerous_plan":     true,
		"plan":                       true,
	}

	for path, meta := range registry {
		if yesRequired[meta.Operation] && !strings.Contains(meta.Confirmation, "--yes") {
			t.Fatalf("capability %q operation %q should require confirmation, got %q", path, meta.Operation, meta.Confirmation)
		}
		if offlinedOperations[meta.Operation] && meta.RequiresConnection {
			t.Fatalf("capability %q is operation %q but requires_connection=%v", path, meta.Operation, meta.RequiresConnection)
		}
	}
}

func TestCapabilityMetadataGenericSetCommandsAreWritePlans(t *testing.T) {
	registry := capabilityMetadataRegistry()
	for _, path := range []string{
		"betaflight-cli pid set",
		"betaflight-cli rates set",
		"betaflight-cli filters set",
		"betaflight-cli receiver set",
		"betaflight-cli vtx set",
		"betaflight-cli osd set",
		"betaflight-cli gps set",
		"betaflight-cli battery set",
		"betaflight-cli failsafe set",
	} {
		meta, ok := registry[path]
		if !ok {
			t.Fatalf("missing metadata for %q", path)
		}
		if meta.Operation != "plan_or_write" || meta.OutputRoot != "change_plan" || !strings.Contains(meta.Confirmation, "--yes") {
			t.Fatalf("metadata %q = %+v, want plan_or_write change_plan with --yes confirmation", path, meta)
		}
	}
}
