package batch

import (
	"bufio"
	"bytes"
	"encoding/json"
	"strings"
)

type ImportOptions struct {
	Kind            string
	SourceFormat    string
	IncludeDefaults bool
}

type ImportResult struct {
	Plan    Plan          `json:"plan"`
	Skipped []SkippedLine `json:"skipped,omitempty"`
}

type SkippedLine struct {
	Line   string `json:"line"`
	Reason string `json:"reason"`
}

func ImportCLI(data []byte, opts ImportOptions) (ImportResult, error) {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return ImportResult{}, errEmptyPlan()
	}
	if trimmed[0] == '{' {
		var raw map[string]any
		if err := json.Unmarshal(trimmed, &raw); err == nil {
			if v, ok := raw["kind"].(string); ok && opts.Kind == "" {
				opts.Kind = v
			}
			if v, ok := raw["source_format"].(string); ok && opts.SourceFormat == "" {
				opts.SourceFormat = v
			}
		}
	}
	if opts.Kind == "" {
		opts.Kind = "cli_import"
	}
	if opts.SourceFormat == "" {
		opts.SourceFormat = "cli_text"
	}
	rawLines, sourceFormat := splitImportLines(trimmed)
	lines, skipped := normalizeImportLines(rawLines, opts.IncludeDefaults)
	if err := checkPlanHasLines(lines); err != nil {
		return ImportResult{}, err
	}
	return ImportResult{
		Plan: Plan{
			SchemaVersion: SchemaVersion,
			Kind:          opts.Kind,
			CLILines:      lines,
			SourceFormat:  sourceOrDefault(opts.SourceFormat, sourceFormat),
		},
		Skipped: skipped,
	}, nil
}

func splitImportLines(raw []byte) ([]string, string) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) > 0 && trimmed[0] == '{' {
		if lines := importLinesFromJSON(trimmed); len(lines) > 0 {
			return lines, "json"
		}
	}
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	var out []string
	for scanner.Scan() {
		rawLine := strings.TrimSpace(scanner.Text())
		if rawLine == "" {
			continue
		}
		out = append(out, rawLine)
	}
	return out, "cli_text"
}

func importLinesFromJSON(raw []byte) []string {
	var rawMap map[string]any
	if err := json.Unmarshal(raw, &rawMap); err != nil {
		return nil
	}
	if lines, ok := normalizedLinesFromJSONMap(rawMap); ok {
		return lines
	}
	if nested, ok := rawMap["data"].(map[string]any); ok {
		if lines, ok := normalizedLinesFromJSONMap(nested); ok {
			return lines
		}
	}
	return nil
}

func normalizedLinesFromJSONMap(payload map[string]any) ([]string, bool) {
	if lines, ok := payload["cli_lines"].([]any); ok {
		return stringSliceOrEmpty(lines)
	}
	if lines, ok := payload["lines"].([]any); ok {
		return stringSliceOrEmpty(lines)
	}
	if raw, ok := payload["raw"].(string); ok {
		return cleanRawJSONLines(raw), true
	}
	return nil, false
}

func stringSliceOrEmpty(lines []any) ([]string, bool) {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if text, ok := line.(string); ok {
			out = append(out, strings.TrimSpace(text))
		}
	}
	return out, len(out) > 0
}

func cleanRawJSONLines(raw string) []string {
	parts := strings.Split(strings.TrimSpace(raw), "\n")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		line := strings.TrimSpace(part)
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}

func normalizeImportLines(rawLines []string, includeDefaults bool) ([]string, []SkippedLine) {
	var lines []string
	var skipped []SkippedLine
	for _, raw := range rawLines {
		if raw == "" {
			continue
		}
		line, ok, reason := normalizeImportLine(raw, includeDefaults)
		if !ok {
			skipped = append(skipped, SkippedLine{Line: raw, Reason: reason})
			continue
		}
		lines = append(lines, line)
	}
	return lines, skipped
}

func checkPlanHasLines(lines []string) error {
	if len(lines) == 0 {
		return errNoLines()
	}
	return nil
}

func normalizeImportLine(line string, includeDefaults bool) (string, bool, string) {
	lower := strings.ToLower(strings.TrimSpace(line))
	switch {
	case strings.HasPrefix(lower, "#"):
		return "", false, "comment"
	case lower == "batch start" || lower == "batch end":
		return "", false, "batch_marker"
	case lower == "save":
		return "", false, "save_is_explicit"
	case isTargetIdentityLine(lower):
		return "", false, "target_identity_metadata"
	case strings.HasPrefix(lower, "defaults"):
		if includeDefaults {
			return line, true, ""
		}
		return "", false, "defaults_require_include_defaults"
	default:
		return line, true, ""
	}
}

func isTargetIdentityLine(lower string) bool {
	fields := strings.Fields(lower)
	if len(fields) == 0 {
		return false
	}
	switch fields[0] {
	case "board_name", "manufacturer_id", "mcu_id", "signature":
		return true
	default:
		return false
	}
}

func errEmptyPlan() error {
	return &parseError{message: "batch plan is empty"}
}

func errNoLines() error {
	return &parseError{message: "import contains no applicable CLI lines"}
}

type parseError struct {
	message string
}

func (e *parseError) Error() string {
	return e.message
}
func sourceOrDefault(explicit, detected string) string {
	if detected == "json" {
		return detected
	}
	if explicit != "" {
		return explicit
	}
	return detected
}
