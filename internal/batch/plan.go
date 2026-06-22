package batch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const SchemaVersion = "1.0"

type Plan struct {
	SchemaVersion string   `json:"schema_version"`
	Kind          string   `json:"kind"`
	CLILines      []string `json:"cli_lines"`
	Save          bool     `json:"save,omitempty"`
	SourceFormat  string   `json:"source_format,omitempty"`
}

func Parse(data []byte) (Plan, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return Plan{}, fmt.Errorf("batch plan is empty")
	}
	if trimmed[0] == '{' {
		return parseJSON(trimmed)
	}
	return parseLines(trimmed), nil
}

func parseJSON(data []byte) (Plan, error) {
	var plan Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		return Plan{}, err
	}
	if plan.SchemaVersion == "" {
		plan.SchemaVersion = SchemaVersion
	}
	if plan.Kind == "" {
		plan.Kind = "cli_batch"
	}
	plan.SourceFormat = "json"
	plan.CLILines = cleanLines(plan.CLILines)
	if len(plan.CLILines) == 0 {
		return Plan{}, fmt.Errorf("json batch plan must include cli_lines")
	}
	return plan, nil
}

func parseLines(data []byte) Plan {
	var lines []string
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		lines = append(lines, line)
	}
	return Plan{
		SchemaVersion: SchemaVersion,
		Kind:          "cli_batch",
		CLILines:      lines,
		SourceFormat:  "lines",
	}
}

func cleanLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}
