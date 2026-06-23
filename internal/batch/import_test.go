package batch

import "testing"

func TestImportCLIAcceptsJSONWithLines(t *testing.T) {
	result, err := ImportCLI([]byte(`{"data":{"lines":["# version","batch start","feature GPS","set small_angle = 25","save","batch end"]}}`), ImportOptions{})
	if err != nil {
		t.Fatalf("ImportCLI() error = %v", err)
	}
	if len(result.Plan.CLILines) != 2 || result.Plan.CLILines[0] != "feature GPS" || result.Plan.CLILines[1] != "set small_angle = 25" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Skipped) != 4 {
		t.Fatalf("skipped = %+v", result.Skipped)
	}
}

func TestImportCLIAcceptsJSONWithRaw(t *testing.T) {
	result, err := ImportCLI([]byte(`{"data":{"raw":"# version\nbatch start\nfeature GPS\nset small_angle = 25\nsave\nbatch end\n"}}`), ImportOptions{})
	if err != nil {
		t.Fatalf("ImportCLI() error = %v", err)
	}
	if len(result.Plan.CLILines) != 2 || result.Plan.CLILines[0] != "feature GPS" || result.Plan.CLILines[1] != "set small_angle = 25" {
		t.Fatalf("result = %+v", result)
	}
}

func TestImportCLIAcceptsTopLevelJSONCLIAndDefaults(t *testing.T) {
	result, err := ImportCLI([]byte(`{"kind":"restore","source_format":"restore_text","cli_lines":["defaults nosave","feature GPS"]}`), ImportOptions{IncludeDefaults: true})
	if err != nil {
		t.Fatalf("ImportCLI() error = %v", err)
	}
	if result.Plan.Kind != "restore" || result.Plan.SourceFormat != "json" {
		t.Fatalf("result = %+v", result)
	}
	if len(result.Plan.CLILines) != 2 || result.Plan.CLILines[0] != "defaults nosave" {
		t.Fatalf("result = %+v", result)
	}
}
