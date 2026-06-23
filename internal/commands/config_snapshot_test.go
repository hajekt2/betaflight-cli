package commands

import (
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

func TestBuildConfigurationSnapshotSummaryAndGuidance(t *testing.T) {
	lines := []string{
		"# version",
		"feature GPS",
		"set gyro_lpf1_static_hz = 0",
		"set imaginary_setting = value",
		"unknown command",
		"save",
	}
	fullDoc := bfconfig.Parse(lines, settings.DefaultRegistry)
	diff := buildConfigurationDocument("diff all", lines, fullDoc)
	full := buildConfigurationDocument("dump all", lines, fullDoc)
	summary := buildConfigurationSnapshotSummary(full, diff)
	if summary.DiffLineCount != 6 || !summary.DiffHasSaveCommand || summary.DiffUnknownCount != 1 || summary.DiffUnknownSettings != 1 {
		t.Fatalf("summary = %+v", summary)
	}
	if !summary.ReviewRequired {
		t.Fatalf("review required = false: %+v", summary)
	}
	guidance := buildConfigurationSnapshotGuidance(summary)
	if !guidance.ReadOnlySnapshot || !guidance.RawCLIIsAuthoritative || !guidance.ReviewUnknownLines || !guidance.ReviewUnknownSettings || !guidance.StripSaveBeforeRestore {
		t.Fatalf("guidance = %+v", guidance)
	}
	if len(guidance.RecommendedNextActions) < 4 {
		t.Fatalf("actions = %+v", guidance.RecommendedNextActions)
	}
}

func TestBuildConfigurationCompare(t *testing.T) {
	reference := []string{
		"# version",
		"batch start",
		"feature GPS",
		"set gyro_lpf1_static_hz = 0",
		"set p_roll = 46",
		"save",
	}
	current := []string{
		"# version",
		"batch start",
		"feature GPS",
		"set gyro_lpf1_static_hz = 0",
		"set p_roll = 45",
		"set roll_rc_rate = 7",
		"save",
	}
	compare := BuildConfigurationCompare("reference", reference, "dump all", current)
	if compare.Summary.Matches || !compare.Summary.ReviewRequired {
		t.Fatalf("summary = %+v", compare.Summary)
	}
	if compare.Summary.ChangedSettingCount != 1 || compare.SettingDiff.Changed[0].Name != "p_roll" {
		t.Fatalf("setting diff = %+v", compare.SettingDiff)
	}
	if compare.Summary.OnlyCurrentSettingCount != 1 || compare.SettingDiff.OnlyInCurrent[0].Name != "roll_rc_rate" {
		t.Fatalf("current settings = %+v", compare.SettingDiff.OnlyInCurrent)
	}
	if len(compare.LineDiff.OnlyInReference) != 1 || len(compare.LineDiff.OnlyInCurrent) != 2 {
		t.Fatalf("line diff = %+v", compare.LineDiff)
	}
}
