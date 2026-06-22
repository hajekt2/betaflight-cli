package blackbox

import (
	"strings"
	"testing"
)

func TestInspectHeaderAndFieldDefinitions(t *testing.T) {
	log := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Data version:2",
		"H Firmware revision:Betaflight 2025.12.1 (abc123) STM32F405",
		"H Firmware date:Jan 01 2026 00:00:00",
		"H Board information:FAKEF405",
		"H Craft name:Test Quad",
		"H Log start datetime:2026-01-01T00:00:00.000",
		"H Field I name:loopIteration,time,axisP[0]",
		"H Field I signed:0,0,1",
		"H Field I predictor:6,0,0",
		"H Field I encoding:1,1,0",
		"H Field P predictor:0,10,0",
		"H Field P encoding:0,0,0",
		"I\x00P\x00P\x00E",
	}, "\n")
	inspection, err := Inspect(strings.NewReader(log))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if inspection.Product == "" || inspection.FirmwareRevision == "" || inspection.CraftName != "Test Quad" {
		t.Fatalf("inspection = %+v", inspection)
	}
	if inspection.HeaderBytes == 0 || inspection.DataBytes == 0 {
		t.Fatalf("header/data bytes = %d/%d", inspection.HeaderBytes, inspection.DataBytes)
	}
	iDef := inspection.FieldDefinitions["I"]
	if len(iDef.Names) != 3 || iDef.Names[1] != "time" || len(iDef.Encoding) != 3 {
		t.Fatalf("I definition = %+v", iDef)
	}
	if inspection.FrameMarkerCountsApprox["I"] != 1 || inspection.FrameMarkerCountsApprox["P"] != 2 || inspection.FrameMarkerCountsApprox["E"] != 1 {
		t.Fatalf("frame counts = %+v", inspection.FrameMarkerCountsApprox)
	}
	if len(inspection.Warnings) != 0 {
		t.Fatalf("warnings = %+v", inspection.Warnings)
	}
}

func TestInspectRejectsMissingHeader(t *testing.T) {
	if _, err := Inspect(strings.NewReader("not a blackbox log")); err == nil {
		t.Fatal("Inspect() error = nil, want error")
	}
}

func TestInspectSkipsLeadingGarbageLines(t *testing.T) {
	inspection, err := Inspect(strings.NewReader("garbage\nH Product:Blackbox flight data recorder by Nicholas Sherlock\nH Field I name:time\nH Field I predictor:0\nH Field I encoding:1\nH Field P predictor:0\nH Field P encoding:0\nI"))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if inspection.Product == "" {
		t.Fatalf("inspection = %+v", inspection)
	}
}

func TestInspectWarnsOnIncompleteDefinitions(t *testing.T) {
	inspection, err := Inspect(strings.NewReader("H Product:Blackbox flight data recorder by Nicholas Sherlock\nH Field I name:time\nI"))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	if len(inspection.Warnings) == 0 {
		t.Fatalf("warnings = %+v", inspection.Warnings)
	}
}
