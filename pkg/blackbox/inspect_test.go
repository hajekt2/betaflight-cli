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
	if inspection.FrameSummaryApprox.CandidateCount != 4 || inspection.FrameSummaryApprox.IndexedCount != 4 {
		t.Fatalf("frame summary = %+v", inspection.FrameSummaryApprox)
	}
	first := inspection.FrameSummaryApprox.Candidates[0]
	if first.Type != "I" || first.DataOffset != 0 || first.BytesToNext != 2 {
		t.Fatalf("first candidate = %+v", first)
	}
	pStats := inspection.FrameSummaryApprox.ByType["P"]
	if pStats.Count != 2 || pStats.MinSpan != 2 || pStats.MaxSpan != 2 {
		t.Fatalf("P stats = %+v", pStats)
	}
	if len(inspection.Warnings) != 0 {
		t.Fatalf("warnings = %+v", inspection.Warnings)
	}
}

func TestInspectDecodesVariableByteFrameSamples(t *testing.T) {
	log := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Field I name:loopIteration,time,axisP[0],gyroADC[0],motor[0]",
		"H Field I signed:0,0,1,1,0",
		"H Field I predictor:6,0,0,0,0",
		"H Field I encoding:1,1,1,1,1",
		"H Field P predictor:0,10,0,0,0",
		"H Field P encoding:1,1,1,1,1",
		"I\x02\x04\x03\x05\x64P\x06\x08\x04\x06\x65P\x07\x0a\x06\x08\x66E",
	}, "\n")
	inspection, err := Inspect(strings.NewReader(log))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	decoded := inspection.DecodedFrames
	if decoded.AttemptedCount != 3 || decoded.DecodedCount != 3 || decoded.FailedCount != 0 {
		t.Fatalf("decoded = %+v", decoded)
	}
	if len(decoded.Samples) != 3 {
		t.Fatalf("samples = %+v", decoded.Samples)
	}
	first := decoded.Samples[0]
	if first.Type != "I" || first.Values["loopIteration"] != 2 || first.Values["time"] != 4 || first.Values["axisP[0]"] != -2 || first.Values["gyroADC[0]"] != -3 || first.Values["motor[0]"] != 100 {
		t.Fatalf("first sample = %+v", first)
	}
	second := decoded.Samples[1]
	if second.Type != "P" || second.Values["loopIteration"] != 6 || second.Values["time"] != 8 || second.Values["axisP[0]"] != 2 || second.Values["gyroADC[0]"] != 3 || second.Values["motor[0]"] != 101 {
		t.Fatalf("second sample = %+v", second)
	}
	iTime := decoded.Streams["I.time"]
	if iTime.Count != 1 || iTime.First != 4 || iTime.Last != 4 || !iTime.Monotonic {
		t.Fatalf("I.time stream = %+v", iTime)
	}
	pAxis := decoded.Streams["P.axisP[0]"]
	if pAxis.Count != 2 || pAxis.Min != 2 || pAxis.Max != 3 || pAxis.Delta != 1 {
		t.Fatalf("P.axisP[0] stream = %+v", pAxis)
	}
	pTime := decoded.Streams["P.time"]
	if pTime.Count != 2 || pTime.First != 8 || pTime.Last != 10 || pTime.Delta != 2 || !pTime.Monotonic {
		t.Fatalf("P.time stream = %+v", pTime)
	}
	groups := decoded.Groups
	for _, group := range []string{"gyro", "motors", "pid", "timing"} {
		if groups[group].Name != group || len(groups[group].Streams) == 0 {
			t.Fatalf("group %s = %+v", group, groups[group])
		}
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

func TestSummarizeFrameCandidatesTruncatesIndex(t *testing.T) {
	summary := summarizeFrameCandidates([]byte("IPGSEIPGSE"), 10, 3)
	if summary.CandidateCount != 10 || summary.IndexedCount != 3 || !summary.Truncated {
		t.Fatalf("summary = %+v", summary)
	}
	if len(summary.Candidates) != 3 || summary.Candidates[0].Offset != 10 || summary.Candidates[2].Type != "G" {
		t.Fatalf("candidates = %+v", summary.Candidates)
	}
	if summary.ByType["I"].Count != 2 || summary.ByType["E"].Count != 2 {
		t.Fatalf("stats = %+v", summary.ByType)
	}
}
