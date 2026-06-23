package blackbox

import (
	"strings"
	"testing"
)

func TestListLogsFindsMultipleLogs(t *testing.T) {
	first := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Firmware revision:Betaflight 2025.12.1 (abc123) STM32F405",
		"H Craft name:Alpha",
		"H Field I name:time",
		"H Field I predictor:0",
		"H Field I encoding:1",
		"H Field P predictor:0",
		"H Field P encoding:0",
		"I\x01",
	}, "\n")
	second := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Firmware revision:Betaflight 2025.12.2 (def456) STM32F722",
		"H Craft name:Bravo",
		"H Field I name:time",
		"H Field I predictor:0",
		"H Field I encoding:1",
		"H Field P predictor:0",
		"H Field P encoding:0",
		"I\x02",
	}, "\n")
	logs, err := ListLogs(strings.NewReader(first + "\n" + second))
	if err != nil {
		t.Fatalf("ListLogs() error = %v", err)
	}
	if len(logs) != 2 {
		t.Fatalf("logs = %+v", logs)
	}
	if logs[0].Index != 0 || logs[0].CraftName != "Alpha" || logs[1].Index != 1 || logs[1].CraftName != "Bravo" {
		t.Fatalf("logs = %+v", logs)
	}
	if logs[1].OffsetBytes <= logs[0].OffsetBytes || logs[0].SizeBytes <= 0 || logs[1].SizeBytes <= 0 {
		t.Fatalf("logs = %+v", logs)
	}
}

func TestListLogsRejectsMissingHeader(t *testing.T) {
	if _, err := ListLogs(strings.NewReader("garbage")); err == nil {
		t.Fatal("ListLogs() error = nil, want error")
	}
}
