package cli

import (
	"sort"

	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) schemaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Show the stable JSON contract metadata for AI integrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			capabilities := collectCapabilityCommands(cmd.Root())
			commandCount := len(capabilities)
			operationCounts := make(map[string]int)
			outputRootSet := map[string]struct{}{}
			runnableCount := 0
			requiresConnectionCount := 0
			for _, capability := range capabilities {
				operationCounts[capability.Operation]++
				if capability.OutputRoot != "" {
					outputRootSet[capability.OutputRoot] = struct{}{}
				}
				if capability.Runnable {
					runnableCount++
				}
				if capability.RequiresConnection {
					requiresConnectionCount++
				}
			}
			operations := make([]string, 0, len(operationCounts))
			for operation := range operationCounts {
				operations = append(operations, operation)
			}
			sort.Strings(operations)
			outputRoots := make([]string, 0, len(outputRootSet))
			for root := range outputRootSet {
				outputRoots = append(outputRoots, root)
			}
			sort.Strings(outputRoots)

			coverage := buildCoverageReport(cmd.Root())
			envelopeFields := map[string]any{
				"schema_version": map[string]any{
					"type":        "string",
					"description": "envelope schema version",
				},
				"ok": map[string]any{
					"type":        "boolean",
					"description": "command success status",
				},
				"command": map[string]any{
					"type":        "string",
					"description": "canonical command that produced this envelope",
				},
				"target": map[string]any{
					"type":        "object",
					"description": "connected target metadata when a serial transport was involved",
				},
				"data": map[string]any{
					"type":        "object",
					"description": "command-specific payload",
				},
				"warnings": map[string]any{
					"type":        "array",
					"description": "non-blocking warnings",
				},
				"errors": map[string]any{
					"type":        "array",
					"description": "structured errors when ok is false",
				},
				"side_effects": map[string]any{
					"type":        "array",
					"description": "externally visible effects such as writes, reboots, or network calls",
				},
			}
			envelopeJSONSchema := map[string]any{
				"$schema":              "https://json-schema.org/draft/2020-12/schema",
				"$id":                  "https://hajekt2.github.io/betaflight-cli/schema/envelope-1.0.json",
				"title":                "betaflight-cli response envelope",
				"type":                 "object",
				"additionalProperties": false,
				"required": []string{
					"schema_version",
					"ok",
					"command",
					"data",
					"warnings",
					"errors",
					"side_effects",
				},
				"properties": map[string]any{
					"schema_version": map[string]any{
						"type":  "string",
						"const": output.SchemaVersion,
					},
					"ok": map[string]any{
						"type": "boolean",
					},
					"command": map[string]any{
						"type":      "string",
						"minLength": 1,
					},
					"target": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"port":             map[string]any{"type": "string"},
							"auto_detected":    map[string]any{"type": "boolean"},
							"selection_reason": map[string]any{"type": "string"},
							"variant":          map[string]any{"type": "string"},
							"firmware_version": map[string]any{"type": "string"},
							"msp_api_version":  map[string]any{"type": "string"},
							"msp_protocol_version": map[string]any{
								"type":    "integer",
								"minimum": 0,
								"maximum": 255,
							},
						},
					},
					"data": map[string]any{
						"type": "object",
					},
					"warnings": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"code", "message"},
							"properties": map[string]any{
								"code":    map[string]any{"type": "string"},
								"message": map[string]any{"type": "string"},
							},
						},
					},
					"errors": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"code", "message"},
							"properties": map[string]any{
								"code":    map[string]any{"type": "string"},
								"message": map[string]any{"type": "string"},
							},
						},
					},
					"side_effects": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"type"},
							"properties": map[string]any{
								"type":    map[string]any{"type": "string"},
								"detail":  map[string]any{"type": "string"},
								"command": map[string]any{"type": "string"},
							},
						},
					},
				},
			}
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"schema": map[string]any{
					"command": "schema",
					"schema_version": map[string]any{
						"envelope":         output.SchemaVersion,
						"inversion_level":  "command-output-schema-first",
						"description":      "stable JSON envelope contract currently used for all machine-facing output",
						"supported_minors": []string{"stable"},
					},
					"envelope":             envelopeFields,
					"envelope_json_schema": envelopeJSONSchema,
					"command_contracts": map[string]any{
						"total_commands":               commandCount,
						"runnable_commands":            runnableCount,
						"requires_connection_commands": requiresConnectionCount,
						"operation_counts":             operationCounts,
						"operations":                   operations,
						"requires_connection_default":  "when command touches transport",
						"commands":                     capabilities,
					},
					"capabilities": map[string]any{
						"coverage": map[string]any{
							"implemented_domains": coverage.Summary.ImplementedCount,
							"partial_domains":     coverage.Summary.PartialCount,
							"domain_count":        coverage.Summary.DomainCount,
							"next_gaps":           coverage.NextGaps,
						},
					},
					"output_roots":  outputRoots,
					"command_count": commandCount,
				},
			}))
		},
	}
}
