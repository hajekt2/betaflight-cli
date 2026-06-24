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
	if first.Type != "I" || first.Values["loopIteration"] != 1 || first.Values["time"] != 4 || first.Values["axisP[0]"] != -2 || first.Values["gyroADC[0]"] != -3 || first.Values["motor[0]"] != 100 {
		t.Fatalf("first sample = %+v", first)
	}
	second := decoded.Samples[1]
	if second.Type != "P" || second.Values["loopIteration"] != 2 || second.Values["time"] != 8 || second.Values["axisP[0]"] != 2 || second.Values["gyroADC[0]"] != 3 || second.Values["motor[0]"] != 101 {
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

func TestInspectDecodesNullEncodingWithPreviousPredictor(t *testing.T) {
	log := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Data version:2",
		"H minthrottle:1000",
		"H Field I name:time,axisP[0],motor[0],motor[1]",
		"H Field I signed:0,1,0,0",
		"H Field I predictor:0,0,4,5",
		"H Field I encoding:1,1,1,9",
		"H Field P predictor:10,1,4,5",
		"H Field P encoding:1,1,9,9",
		"I\x02\x04\x06P\x06\x01E",
	}, "\n")
	inspection, err := Inspect(strings.NewReader(log))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	decoded := inspection.DecodedFrames
	if decoded.DecodedCount != 2 || decoded.FailedCount != 0 {
		t.Fatalf("decoded = %+v", decoded)
	}
	if len(decoded.Warnings) != 0 {
		t.Fatalf("warnings = %+v", decoded.Warnings)
	}
	second := decoded.Samples[1]
	if second.Type != "P" {
		t.Fatalf("second sample = %+v", second)
	}
	if second.Values["time"] != 6 || second.Values["axisP[0]"] != -1 || second.Values["motor[0]"] != 1000 || second.Values["motor[1]"] != 1000 {
		t.Fatalf("second sample = %+v", second)
	}
}

func TestInspectReportsDecodeSupportForUnsupportedEncoding(t *testing.T) {
	log := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Field I name:time,unsupported",
		"H Field I signed:0,0",
		"H Field I predictor:0,0",
		"H Field I encoding:1,10",
		"I\x02\x04E",
	}, "\n")
	inspection, err := Inspect(strings.NewReader(log))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	support := inspection.DecodeSupport
	if support.Status != "partial" {
		t.Fatalf("status = %q, want partial", support.Status)
	}
	if !containsString(support.UnsupportedEncodings, "10") {
		t.Fatalf("unsupported encodings = %+v", support.UnsupportedEncodings)
	}
	if !containsString(support.SupportedEncodings, "9") {
		t.Fatalf("supported encodings = %+v", support.SupportedEncodings)
	}
	if len(support.Notes) == 0 {
		t.Fatalf("notes = %+v", support.Notes)
	}
}

func TestInspectRejectsMissingHeader(t *testing.T) {
	if _, err := Inspect(strings.NewReader("not a blackbox log")); err == nil {
		t.Fatal("Inspect() error = nil, want error")
	}
}

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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

func TestInspectDecodesSupportedEvents(t *testing.T) {
	log := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Field I name:time",
		"H Field I predictor:0",
		"H Field I encoding:1",
		"H Field P predictor:0",
		"H Field P encoding:1",
		"E\x00\x7b",
		"E\x0f\x02",
		"E\x1e\x01\x00",
		"E\x0e\x2a\x64",
		"E\xffdone\x00",
	}, "\n")
	inspection, err := Inspect(strings.NewReader(log))
	if err != nil {
		t.Fatalf("Inspect() error = %v", err)
	}
	events := inspection.Events
	if events.AttemptedCount != 5 || events.DecodedCount != 5 || events.FailedCount != 0 {
		t.Fatalf("events = %+v", events)
	}
	if len(events.Samples) != 5 {
		t.Fatalf("samples = %+v", events.Samples)
	}
	if events.Samples[0].Type != "SYNC_BEEP" || events.Samples[0].Fields["time"] != 123 {
		t.Fatalf("sample0 = %+v", events.Samples[0])
	}
	if events.Samples[1].Type != "DISARM" || events.Samples[1].Fields["reason_name"] != "THROTTLE_TIMEOUT" {
		t.Fatalf("sample1 = %+v", events.Samples[1])
	}
	if events.Samples[2].Type != "FLIGHT_MODE" {
		t.Fatalf("sample2 = %+v", events.Samples[2])
	}
	enabled := events.Samples[2].Fields["enabled_modes"].([]string)
	if len(enabled) != 1 || enabled[0] != "ARM" {
		t.Fatalf("enabled = %+v", enabled)
	}
	if events.Samples[3].Type != "LOGGING_RESUME" || events.Samples[3].Fields["log_iteration"] != 42 || events.Samples[3].Fields["current_time"] != 100 {
		t.Fatalf("sample3 = %+v", events.Samples[3])
	}
	if events.Samples[4].Type != "LOG_END" || events.Samples[4].Fields["message"] != "done" {
		t.Fatalf("sample4 = %+v", events.Samples[4])
	}
}
