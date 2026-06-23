package commands

import (
	"context"
	"fmt"
	"sort"
	"strings"

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

type ConfigurationCompare struct {
	Source             string                      `json:"source"`
	Reference          ConfigurationDocument       `json:"reference"`
	Current            ConfigurationDocument       `json:"current"`
	Summary            ConfigurationCompareSummary `json:"summary"`
	LineDiff           ConfigurationLineDiff       `json:"line_diff"`
	SettingDiff        ConfigurationSettingDiff    `json:"setting_diff"`
	RecommendedActions []string                    `json:"recommended_actions"`
}

type ConfigurationCompareSummary struct {
	Matches                   bool `json:"matches"`
	ReviewRequired            bool `json:"review_required"`
	ReferenceLineCount        int  `json:"reference_line_count"`
	CurrentLineCount          int  `json:"current_line_count"`
	OnlyInReferenceCount      int  `json:"only_in_reference_count"`
	OnlyInCurrentCount        int  `json:"only_in_current_count"`
	ChangedSettingCount       int  `json:"changed_setting_count"`
	OnlyReferenceSettingCount int  `json:"only_reference_setting_count"`
	OnlyCurrentSettingCount   int  `json:"only_current_setting_count"`
	ReferenceUnknownCount     int  `json:"reference_unknown_count"`
	CurrentUnknownCount       int  `json:"current_unknown_count"`
}

type ConfigurationLineDiff struct {
	OnlyInReference []string `json:"only_in_reference"`
	OnlyInCurrent   []string `json:"only_in_current"`
}

type ConfigurationSettingDiff struct {
	Changed         []ConfigurationSettingChange `json:"changed"`
	OnlyInReference []bfconfig.Setting           `json:"only_in_reference"`
	OnlyInCurrent   []bfconfig.Setting           `json:"only_in_current"`
}

type ConfigurationSettingChange struct {
	Name      string           `json:"name"`
	Reference bfconfig.Setting `json:"reference"`
	Current   bfconfig.Setting `json:"current"`
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

func ReadConfigurationCompare(ctx context.Context, client *connection.Client, referenceLines []string) (*ConfigurationCompare, error) {
	currentLines, err := client.ExecCLI(ctx, "dump all")
	if err != nil {
		return nil, fmt.Errorf("dump all unavailable: %w", err)
	}
	return BuildConfigurationCompare("reference", referenceLines, "dump all", currentLines), nil
}

func BuildConfigurationCompare(referenceSource string, referenceLines []string, currentSource string, currentLines []string) *ConfigurationCompare {
	referenceDoc := bfconfig.Parse(referenceLines, settings.DefaultRegistry)
	currentDoc := bfconfig.Parse(currentLines, settings.DefaultRegistry)
	reference := buildConfigurationDocument(referenceSource, referenceLines, referenceDoc)
	current := buildConfigurationDocument(currentSource, currentLines, currentDoc)
	lineDiff := diffConfigurationLines(referenceLines, currentLines)
	settingDiff := diffConfigurationSettings(referenceDoc.Settings, currentDoc.Settings)
	summary := ConfigurationCompareSummary{
		Matches:                   len(lineDiff.OnlyInReference) == 0 && len(lineDiff.OnlyInCurrent) == 0,
		ReferenceLineCount:        reference.LineCount,
		CurrentLineCount:          current.LineCount,
		OnlyInReferenceCount:      len(lineDiff.OnlyInReference),
		OnlyInCurrentCount:        len(lineDiff.OnlyInCurrent),
		ChangedSettingCount:       len(settingDiff.Changed),
		OnlyReferenceSettingCount: len(settingDiff.OnlyInReference),
		OnlyCurrentSettingCount:   len(settingDiff.OnlyInCurrent),
		ReferenceUnknownCount:     reference.UnknownCount,
		CurrentUnknownCount:       current.UnknownCount,
	}
	summary.ReviewRequired = !summary.Matches || summary.ReferenceUnknownCount > 0 || summary.CurrentUnknownCount > 0
	return &ConfigurationCompare{
		Source:             "local reference and current dump all",
		Reference:          reference,
		Current:            current,
		Summary:            summary,
		LineDiff:           lineDiff,
		SettingDiff:        settingDiff,
		RecommendedActions: buildConfigurationCompareActions(summary),
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

func diffConfigurationLines(referenceLines, currentLines []string) ConfigurationLineDiff {
	reference := countComparableLines(referenceLines)
	current := countComparableLines(currentLines)
	return ConfigurationLineDiff{
		OnlyInReference: sortedLineDelta(reference, current),
		OnlyInCurrent:   sortedLineDelta(current, reference),
	}
}

func countComparableLines(lines []string) map[string]int {
	out := map[string]int{}
	for _, line := range lines {
		normalized, ok := comparableConfigurationLine(line)
		if !ok {
			continue
		}
		out[normalized]++
	}
	return out
}

func comparableConfigurationLine(line string) (string, bool) {
	trimmed := strings.TrimSpace(line)
	if trimmed == "" || strings.HasPrefix(trimmed, "#") {
		return "", false
	}
	lower := strings.ToLower(trimmed)
	switch {
	case lower == "batch start" || lower == "batch end" || lower == "save":
		return "", false
	default:
		return trimmed, true
	}
}

func sortedLineDelta(left, right map[string]int) []string {
	var out []string
	for line, count := range left {
		delta := count - right[line]
		for i := 0; i < delta; i++ {
			out = append(out, line)
		}
	}
	sort.Strings(out)
	return out
}

func diffConfigurationSettings(reference, current []bfconfig.Setting) ConfigurationSettingDiff {
	refByName := settingsByName(reference)
	curByName := settingsByName(current)
	var changed []ConfigurationSettingChange
	var onlyReference []bfconfig.Setting
	var onlyCurrent []bfconfig.Setting
	for name, ref := range refByName {
		cur, ok := curByName[name]
		if !ok {
			onlyReference = append(onlyReference, ref)
			continue
		}
		if ref.Value != cur.Value {
			changed = append(changed, ConfigurationSettingChange{Name: name, Reference: ref, Current: cur})
		}
	}
	for name, cur := range curByName {
		if _, ok := refByName[name]; !ok {
			onlyCurrent = append(onlyCurrent, cur)
		}
	}
	sort.Slice(changed, func(i, j int) bool { return changed[i].Name < changed[j].Name })
	sort.Slice(onlyReference, func(i, j int) bool { return onlyReference[i].Name < onlyReference[j].Name })
	sort.Slice(onlyCurrent, func(i, j int) bool { return onlyCurrent[i].Name < onlyCurrent[j].Name })
	return ConfigurationSettingDiff{Changed: changed, OnlyInReference: onlyReference, OnlyInCurrent: onlyCurrent}
}

func settingsByName(settings []bfconfig.Setting) map[string]bfconfig.Setting {
	out := map[string]bfconfig.Setting{}
	for _, setting := range settings {
		out[setting.Name] = setting
	}
	return out
}

func buildConfigurationCompareActions(summary ConfigurationCompareSummary) []string {
	if summary.Matches && !summary.ReviewRequired {
		return []string{"no configuration drift detected"}
	}
	actions := []string{"review line_diff before planning writes"}
	if summary.ChangedSettingCount > 0 || summary.OnlyReferenceSettingCount > 0 || summary.OnlyCurrentSettingCount > 0 {
		actions = append(actions, "review setting_diff for machine-readable setting changes")
	}
	if summary.ReferenceUnknownCount > 0 || summary.CurrentUnknownCount > 0 {
		actions = append(actions, "inspect unknown lines before applying a restore or batch plan")
	}
	return actions
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
