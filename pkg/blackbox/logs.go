package blackbox

import (
	"bytes"
	"fmt"
	"io"
)

type LogSummary struct {
	Index            int        `json:"index"`
	OffsetBytes      int64      `json:"offset_bytes"`
	SizeBytes        int64      `json:"size_bytes"`
	Product          string     `json:"product,omitempty"`
	FirmwareRevision string     `json:"firmware_revision,omitempty"`
	CraftName        string     `json:"craft_name,omitempty"`
	LogStartDatetime string     `json:"log_start_datetime,omitempty"`
	Inspection       Inspection `json:"inspection"`
}

func ListLogs(r io.Reader) ([]LogSummary, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	offsets := findLogOffsets(data)
	if len(offsets) == 0 {
		return nil, fmt.Errorf("blackbox header not found")
	}
	logs := make([]LogSummary, 0, len(offsets))
	for i, start := range offsets {
		end := len(data)
		if i+1 < len(offsets) {
			end = offsets[i+1]
		}
		inspection, err := Inspect(bytes.NewReader(data[start:end]))
		if err != nil {
			return nil, fmt.Errorf("log %d at offset %d: %w", i, start, err)
		}
		logs = append(logs, LogSummary{
			Index:            i,
			OffsetBytes:      int64(start),
			SizeBytes:        int64(end - start),
			Product:          inspection.Product,
			FirmwareRevision: inspection.FirmwareRevision,
			CraftName:        inspection.CraftName,
			LogStartDatetime: inspection.LogStartDatetime,
			Inspection:       inspection,
		})
	}
	return logs, nil
}

func findLogOffsets(data []byte) []int {
	const marker = "H Product:Blackbox"
	var offsets []int
	if bytes.HasPrefix(data, []byte(marker)) {
		offsets = append(offsets, 0)
	}
	searchFrom := 0
	for {
		index := bytes.Index(data[searchFrom:], []byte("\n"+marker))
		if index < 0 {
			break
		}
		start := searchFrom + index + 1
		offsets = append(offsets, start)
		searchFrom = start + len(marker)
	}
	return offsets
}
