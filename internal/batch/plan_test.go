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
