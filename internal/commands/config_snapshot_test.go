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
	if summary.DiffLineCount != 6 || !summary.DiffHasSaveCommand || summary.DiffUnknownCount != 2 || summary.DiffUnknownSettings != 1 {
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
