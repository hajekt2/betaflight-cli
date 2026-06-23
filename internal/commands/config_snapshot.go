package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

type ConfigurationSnapshot struct {
	Source   string                        `json:"source"`
	Full     ConfigurationDocument         `json:"full"`
	Diff     ConfigurationDocument         `json:"diff"`
	Summary  ConfigurationSnapshotSummary  `json:"summary"`
	Guidance ConfigurationSnapshotGuidance `json:"guidance"`
}

type ConfigurationDocument struct {
	SourceCommand   string            `json:"source_command"`
	LineCount       int               `json:"line_count"`
	SectionCounts   map[string]int    `json:"section_counts"`
	UnknownCount    int               `json:"unknown_count"`
	KnownSettings   int               `json:"known_settings"`
	UnknownSettings int               `json:"unknown_settings"`
	Document        bfconfig.Document `json:"document"`
}

type ConfigurationSnapshotSummary struct {
	FullLineCount       int            `json:"full_line_count"`
	DiffLineCount       int            `json:"diff_line_count"`
	DiffHasSaveCommand  bool           `json:"diff_has_save_command"`
	DiffUnknownCount    int            `json:"diff_unknown_count"`
	DiffUnknownSettings int            `json:"diff_unknown_settings"`
	DiffSectionCounts   map[string]int `json:"diff_section_counts"`
	ReviewRequired      bool           `json:"review_required"`
}

type ConfigurationSnapshotGuidance struct {
	ReadOnlySnapshot       bool     `json:"read_only_snapshot"`
	RawCLIIsAuthoritative  bool     `json:"raw_cli_is_authoritative"`
	ReviewUnknownLines     bool     `json:"review_unknown_lines"`
	ReviewUnknownSettings  bool     `json:"review_unknown_settings"`
	StripSaveBeforeRestore bool     `json:"strip_save_before_restore"`
	RecommendedNextActions []string `json:"recommended_next_actions"`
}

func ReadConfigurationSnapshot(ctx context.Context, client *connection.Client) (*ConfigurationSnapshot, error) {
	fullLines, err := client.ExecCLI(ctx, "dump all")
	if err != nil {
		return nil, fmt.Errorf("dump all unavailable: %w", err)
	}
	diffLines, err := client.ExecCLI(ctx, "diff all")
	if err != nil {
		return nil, fmt.Errorf("diff all unavailable: %w", err)
	}
	fullDoc := bfconfig.Parse(fullLines, settings.DefaultRegistry)
	diffDoc := bfconfig.Parse(diffLines, settings.DefaultRegistry)
	full := buildConfigurationDocument("dump all", fullLines, fullDoc)
	diff := buildConfigurationDocument("diff all", diffLines, diffDoc)
	summary := buildConfigurationSnapshotSummary(full, diff)
	return &ConfigurationSnapshot{
		Source:   "CLI dump all and diff all",
		Full:     full,
		Diff:     diff,
		Summary:  summary,
		Guidance: buildConfigurationSnapshotGuidance(summary),
	}, nil
}

func buildConfigurationDocument(sourceCommand string, lines []string, doc bfconfig.Document) ConfigurationDocument {
	knownSettings := 0
	unknownSettings := 0
	for _, setting := range doc.Settings {
		if setting.Known {
			knownSettings++
		} else {
			unknownSettings++
		}
	}
	return ConfigurationDocument{
		SourceCommand:   sourceCommand,
		LineCount:       len(nonEmptyLines(lines)),
		SectionCounts:   countSections(doc.Sections),
		UnknownCount:    len(doc.Unknown),
		KnownSettings:   knownSettings,
		UnknownSettings: unknownSettings,
		Document:        doc,
	}
}

func buildConfigurationSnapshotSummary(full, diff ConfigurationDocument) ConfigurationSnapshotSummary {
	summary := ConfigurationSnapshotSummary{
		FullLineCount:       full.LineCount,
		DiffLineCount:       diff.LineCount,
		DiffUnknownCount:    diff.UnknownCount,
		DiffUnknownSettings: diff.UnknownSettings,
		DiffSectionCounts:   diff.SectionCounts,
	}
	for _, command := range diff.Document.Commands {
		if command.Kind == "save" {
			summary.DiffHasSaveCommand = true
			break
		}
	}
	summary.ReviewRequired = summary.DiffUnknownCount > 0 || summary.DiffUnknownSettings > 0 || summary.DiffHasSaveCommand
	return summary
}

func buildConfigurationSnapshotGuidance(summary ConfigurationSnapshotSummary) ConfigurationSnapshotGuidance {
	guidance := ConfigurationSnapshotGuidance{
		ReadOnlySnapshot:       true,
		RawCLIIsAuthoritative:  true,
		ReviewUnknownLines:     summary.DiffUnknownCount > 0,
		ReviewUnknownSettings:  summary.DiffUnknownSettings > 0,
		StripSaveBeforeRestore: summary.DiffHasSaveCommand,
		RecommendedNextActions: []string{"review diff buckets before planning writes", "use restore plan or batch plan before applying CLI lines"},
	}
	if summary.DiffHasSaveCommand {
		guidance.RecommendedNextActions = append([]string{"remove save from imported plans and run save --yes explicitly after review"}, guidance.RecommendedNextActions...)
	}
	if summary.DiffUnknownCount > 0 || summary.DiffUnknownSettings > 0 {
		guidance.RecommendedNextActions = append(guidance.RecommendedNextActions, "inspect unknown lines before applying or restoring")
	}
	return guidance
}

func countSections(sections map[string][]string) map[string]int {
	counts := map[string]int{}
	for name, lines := range sections {
		counts[name] = len(lines)
	}
	return counts
}

func nonEmptyLines(lines []string) []string {
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if line != "" {
			out = append(out, line)
		}
	}
	return out
}
