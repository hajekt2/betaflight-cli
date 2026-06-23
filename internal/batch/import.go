package batch

import (
	"bufio"
	"bytes"
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
	if opts.Kind == "" {
		opts.Kind = "cli_import"
	}
	if opts.SourceFormat == "" {
		opts.SourceFormat = "cli_text"
	}
	var lines []string
	var skipped []SkippedLine
	scanner := bufio.NewScanner(bytes.NewReader(trimmed))
	for scanner.Scan() {
		raw := strings.TrimSpace(scanner.Text())
		if raw == "" {
			continue
		}
		line, ok, reason := normalizeImportLine(raw, opts.IncludeDefaults)
		if !ok {
			skipped = append(skipped, SkippedLine{Line: raw, Reason: reason})
			continue
		}
		lines = append(lines, line)
	}
	if err := scanner.Err(); err != nil {
		return ImportResult{}, err
	}
	if len(lines) == 0 {
		return ImportResult{}, errNoLines()
	}
	return ImportResult{
		Plan: Plan{
			SchemaVersion: SchemaVersion,
			Kind:          opts.Kind,
			CLILines:      lines,
			SourceFormat:  opts.SourceFormat,
		},
		Skipped: skipped,
	}, nil
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
