package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func TestSchemaCommandDoesNotConnect(t *testing.T) {
	called := false
	env, err := runTestCommand(t, []string{"schema"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
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
		t.Fatalf("schema command should be offline and never connect")
	}
	data := env.Data.(map[string]any)
	schemaData := data["schema"].(map[string]any)
	if schemaData["command"] != "schema" {
		t.Fatalf("command = %v", schemaData["command"])
	}
	schemaVersion := schemaData["schema_version"].(map[string]any)
	if schemaVersion["envelope"] != output.SchemaVersion {
		t.Fatalf("schema_version = %+v", schemaVersion)
	}
	envelopeJSONSchema := schemaData["envelope_json_schema"].(map[string]any)
	if envelopeJSONSchema["$schema"] != "https://json-schema.org/draft/2020-12/schema" {
		t.Fatalf("envelope_json_schema = %+v", envelopeJSONSchema)
	}
	properties := envelopeJSONSchema["properties"].(map[string]any)
	dataTypes := properties["data"].(map[string]any)["type"].([]any)
	if !containsAnyString(dataTypes, "object") || !containsAnyString(dataTypes, "string") {
		t.Fatalf("envelope_json_schema data types = %+v", dataTypes)
	}
	schemaVersionProperty := properties["schema_version"].(map[string]any)
	if schemaVersionProperty["const"] != output.SchemaVersion {
		t.Fatalf("envelope_json_schema schema_version = %+v", schemaVersionProperty)
	}
	required := envelopeJSONSchema["required"].([]any)
	for _, field := range []string{"schema_version", "ok", "command", "data", "warnings", "errors", "side_effects"} {
		if !containsAnyString(required, field) {
			t.Fatalf("envelope_json_schema required = %+v, missing %q", required, field)
		}
	}
	contract := schemaData["command_contracts"].(map[string]any)
	if contract["total_commands"] == nil {
		t.Fatalf("command_contracts = %+v", contract)
	}
	if contract["runnable_commands"] == nil || contract["requires_connection_commands"] == nil {
		t.Fatalf("command_contracts missing counts = %+v", contract)
	}
	contractCommands := contract["commands"].([]any)
	if len(contractCommands) == 0 {
		t.Fatalf("command_contracts.commands = %+v", contractCommands)
	}
	contractByCommand := map[string]map[string]any{}
	for _, item := range contractCommands {
		row := item.(map[string]any)
		contractByCommand[row["command"].(string)] = row
	}
	for command, outputRoot := range map[string]string{
		"betaflight-cli info":              "info",
		"betaflight-cli motors test-apply": "motor_test_plan",
		"betaflight-cli save":              "save",
	} {
		row, ok := contractByCommand[command]
		if !ok {
			t.Fatalf("command_contracts.commands missing %q", command)
		}
		if row["output_root"] != outputRoot || row["operation"] == "" || row["confirmation"] == "" {
			t.Fatalf("command_contracts.commands[%s] = %+v", command, row)
		}
	}
	operations := contract["operations"].([]any)
	if len(operations) < 6 {
		t.Fatalf("operations = %+v", operations)
	}
	operationCounts := contract["operation_counts"].(map[string]any)
	if len(operationCounts) == 0 {
		t.Fatalf("operation_counts = %+v", contract)
	}
	for key, total := range operationCounts {
		if total == nil || total.(float64) <= 0 {
			t.Fatalf("operation_counts[%s]=%v", key, total)
		}
	}
	outputRoots := schemaData["output_roots"].([]any)
	if len(outputRoots) == 0 {
		t.Fatalf("output_roots = %+v", outputRoots)
	}
	caps := schemaData["capabilities"].(map[string]any)
	coverage := caps["coverage"].(map[string]any)
	if coverage["implemented_domains"] == nil || coverage["partial_domains"] == nil || coverage["domain_count"] == nil {
		t.Fatalf("capabilities.coverage = %+v", coverage)
	}
	if _, ok := coverage["next_gaps"].([]any); !ok {
		t.Fatalf("capabilities.coverage.next_gaps = %+v", coverage["next_gaps"])
	}
}

func TestSchemaCommandContractsMatchCapabilities(t *testing.T) {
	schemaEnv, err := runTestCommand(t, []string{"schema"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("schema unexpectedly connected")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("schema command error = %v", err)
	}
	capabilitiesEnv, err := runTestCommand(t, []string{"capabilities"}, func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error) {
		t.Fatal("capabilities unexpectedly connected")
		return nil, connection.TargetInfo{}, nil
	})
	if err != nil {
		t.Fatalf("capabilities command error = %v", err)
	}

	schemaRows := schemaCommandContractRows(t, schemaEnv)
	capabilityRows := capabilityCommandRows(t, capabilitiesEnv)
	if len(schemaRows) != len(capabilityRows) {
		t.Fatalf("schema command rows = %d, capabilities command rows = %d", len(schemaRows), len(capabilityRows))
	}
	for command, schemaRow := range schemaRows {
		capabilityRow, ok := capabilityRows[command]
		if !ok {
			t.Fatalf("schema command %q missing from capabilities", command)
		}
		for _, field := range []string{"short", "runnable", "requires_connection", "operation", "confirmation", "output_root", "input"} {
			if schemaRow[field] != capabilityRow[field] {
				t.Fatalf("%s field %s mismatch: schema=%v capabilities=%v", command, field, schemaRow[field], capabilityRow[field])
			}
		}
		operation, _ := schemaRow["operation"].(string)
		if schemaRow["runnable"] == true && schemaRow["output_root"] == "" && operation != "text_output" {
			t.Fatalf("%s is runnable without output_root: %+v", command, schemaRow)
		}
		confirmation, _ := schemaRow["confirmation"].(string)
		if operationNeedsConfirmation(operation) && !strings.Contains(confirmation, "--yes") {
			t.Fatalf("%s operation %q should advertise --yes confirmation, got %q", command, operation, confirmation)
		}
		if operation == "group" && schemaRow["runnable"] == true {
			t.Fatalf("%s is runnable but classified as group", command)
		}
	}
}
