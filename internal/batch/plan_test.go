package batch

import "testing"

func TestParseLinePlan(t *testing.T) {
	plan, err := Parse([]byte(`
# comment
feature GPS
set small_angle = 25
`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if plan.SourceFormat != "lines" || len(plan.CLILines) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestParseJSONPlan(t *testing.T) {
	plan, err := Parse([]byte(`{"schema_version":"1.0","kind":"cli_batch","cli_lines":["feature GPS","set small_angle = 25"],"save":true}`))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	if !plan.Save || plan.SourceFormat != "json" || len(plan.CLILines) != 2 {
		t.Fatalf("plan = %+v", plan)
	}
}

func TestParseEmptyPlan(t *testing.T) {
	if _, err := Parse([]byte(" \n\t ")); err == nil {
		t.Fatal("Parse() error = nil, want error")
	}
}

func TestImportCLISkipsBackupWrappers(t *testing.T) {
	result, err := ImportCLI([]byte(`
# version
batch start
defaults nosave
feature GPS
set small_angle = 25
save
batch end
`), ImportOptions{Kind: "restore", SourceFormat: "restore_text"})
	if err != nil {
		t.Fatalf("ImportCLI() error = %v", err)
	}
	if result.Plan.Kind != "restore" || result.Plan.SourceFormat != "restore_text" {
		t.Fatalf("plan = %+v", result.Plan)
	}
	if len(result.Plan.CLILines) != 2 || result.Plan.CLILines[0] != "feature GPS" || result.Plan.CLILines[1] != "set small_angle = 25" {
		t.Fatalf("lines = %+v", result.Plan.CLILines)
	}
	if len(result.Skipped) != 5 {
		t.Fatalf("skipped = %+v", result.Skipped)
	}
}

func TestImportCLIIncludesDefaultsWhenExplicit(t *testing.T) {
	result, err := ImportCLI([]byte("defaults nosave\nfeature GPS\n"), ImportOptions{Kind: "restore", IncludeDefaults: true})
	if err != nil {
		t.Fatalf("ImportCLI() error = %v", err)
	}
	if len(result.Plan.CLILines) != 2 || result.Plan.CLILines[0] != "defaults nosave" {
		t.Fatalf("lines = %+v", result.Plan.CLILines)
	}
	if len(result.Skipped) != 0 {
		t.Fatalf("skipped = %+v", result.Skipped)
	}
}

func TestImportCLIRejectsEmptyImport(t *testing.T) {
	if _, err := ImportCLI([]byte("# only comments\nbatch start\nsave\n"), ImportOptions{}); err == nil {
		t.Fatal("ImportCLI() error = nil, want error")
	}
}
