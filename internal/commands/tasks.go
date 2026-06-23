package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type TaskStatus struct {
	Source         string              `json:"source"`
	RawLines       []string            `json:"raw_lines"`
	Statistics     bool                `json:"statistics"`
	LateStatistics bool                `json:"late_statistics"`
	Tasks          []TaskRow           `json:"tasks"`
	CheckFunctions *CheckFunctionStats `json:"check_functions,omitempty"`
	Total          *TaskTotal          `json:"total,omitempty"`
	Warnings       []string            `json:"warnings,omitempty"`
}

type TaskRow struct {
	ID                  uint8    `json:"id"`
	Name                string   `json:"name"`
	RateHz              int      `json:"rate_hz"`
	MaxExecutionUS      *int     `json:"max_execution_us,omitempty"`
	AverageExecutionUS  *int     `json:"average_execution_us,omitempty"`
	MaxLoadPercent      *float64 `json:"max_load_percent,omitempty"`
	AverageLoadPercent  *float64 `json:"average_load_percent,omitempty"`
	TotalExecutionMS    *int     `json:"total_execution_ms,omitempty"`
	LateCount           *int     `json:"late_count,omitempty"`
	RunCount            *int     `json:"run_count,omitempty"`
	RequiredExecutionUS *int     `json:"required_execution_us,omitempty"`
	Raw                 string   `json:"raw"`
}

type CheckFunctionStats struct {
	MaxExecutionUS     int      `json:"max_execution_us"`
	AverageExecutionUS int      `json:"average_execution_us"`
	AverageLoadPercent *float64 `json:"average_load_percent,omitempty"`
	TotalExecutionMS   int      `json:"total_execution_ms"`
	Raw                string   `json:"raw"`
}

type TaskTotal struct {
	AverageLoadPercent float64 `json:"average_load_percent"`
	Raw                string  `json:"raw"`
}

func ReadTaskStatus(ctx context.Context, client *connection.Client) (*TaskStatus, error) {
	lines, err := client.ExecCLI(ctx, "tasks")
	if err != nil {
		return nil, fmt.Errorf("tasks unavailable: %w", err)
	}
	return ParseTaskStatus(lines), nil
}

func ParseTaskStatus(lines []string) *TaskStatus {
	status := &TaskStatus{
		Source:   "CLI tasks",
		RawLines: lines,
		Tasks:    []TaskRow{},
		Warnings: []string{},
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "":
			continue
		case strings.HasPrefix(trimmed, "Task list"):
			status.Statistics = strings.Contains(trimmed, "max/us")
			status.LateStatistics = strings.Contains(trimmed, "late")
		case strings.HasPrefix(trimmed, "Check Functions"):
			if check, ok := parseCheckFunctionStats(trimmed); ok {
				status.CheckFunctions = check
			} else {
				status.Warnings = append(status.Warnings, "could not parse check function row: "+trimmed)
			}
		case strings.HasPrefix(trimmed, "Total (excluding SERIAL)"):
			if total, ok := parseTaskTotal(trimmed); ok {
				status.Total = total
			} else {
				status.Warnings = append(status.Warnings, "could not parse total row: "+trimmed)
			}
		case looksLikeTaskRow(trimmed):
			task, ok := parseTaskRow(trimmed)
			if ok {
				status.Tasks = append(status.Tasks, task)
			} else {
				status.Warnings = append(status.Warnings, "could not parse task row: "+trimmed)
			}
		}
	}
	return status
}

func looksLikeTaskRow(line string) bool {
	if len(line) < 4 {
		return false
	}
	_, err := strconv.Atoi(line[:2])
	return err == nil && strings.Contains(line, " - (")
}

func parseTaskRow(line string) (TaskRow, bool) {
	id, err := strconv.Atoi(line[:2])
	if err != nil {
		return TaskRow{}, false
	}
	open := strings.Index(line, "(")
	close := strings.Index(line, ")")
	if open < 0 || close <= open {
		return TaskRow{}, false
	}
	name := strings.TrimSpace(line[open+1 : close])
	fields := strings.Fields(line[close+1:])
	if len(fields) < 1 {
		return TaskRow{}, false
	}
	rate, err := strconv.Atoi(fields[0])
	if err != nil {
		return TaskRow{}, false
	}
	row := TaskRow{ID: uint8(id), Name: name, RateHz: rate, Raw: line}
	if len(fields) >= 6 {
		if maxExec, ok := parseIntField(fields[1]); ok {
			row.MaxExecutionUS = &maxExec
		}
		if avgExec, ok := parseIntField(fields[2]); ok {
			row.AverageExecutionUS = &avgExec
		}
		if maxLoad, ok := parsePercentField(fields[3]); ok {
			row.MaxLoadPercent = &maxLoad
		}
		if avgLoad, ok := parsePercentField(fields[4]); ok {
			row.AverageLoadPercent = &avgLoad
		}
		if total, ok := parseIntField(fields[5]); ok {
			row.TotalExecutionMS = &total
		}
	}
	if len(fields) >= 9 {
		if late, ok := parseIntField(fields[6]); ok {
			row.LateCount = &late
		}
		if run, ok := parseIntField(fields[7]); ok {
			row.RunCount = &run
		}
		if required, ok := parseIntField(fields[8]); ok {
			row.RequiredExecutionUS = &required
		}
	}
	return row, true
}

func parseCheckFunctionStats(line string) (*CheckFunctionStats, bool) {
	fields := strings.Fields(line)
	if len(fields) < 7 {
		return nil, false
	}
	maxExec, ok := parseIntField(fields[len(fields)-4])
	if !ok {
		return nil, false
	}
	avgExec, ok := parseIntField(fields[len(fields)-3])
	if !ok {
		return nil, false
	}
	load, ok := parsePercentField(fields[len(fields)-2])
	total, totalOK := parseIntField(fields[len(fields)-1])
	if !totalOK {
		return nil, false
	}
	stats := &CheckFunctionStats{
		MaxExecutionUS:     maxExec,
		AverageExecutionUS: avgExec,
		TotalExecutionMS:   total,
		Raw:                line,
	}
	if ok {
		stats.AverageLoadPercent = &load
	}
	return stats, true
}

func parseTaskTotal(line string) (*TaskTotal, bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return nil, false
	}
	load, ok := parsePercentField(fields[len(fields)-1])
	if !ok {
		return nil, false
	}
	return &TaskTotal{AverageLoadPercent: load, Raw: line}, true
}

func parseIntField(value string) (int, bool) {
	parsed, err := strconv.Atoi(strings.TrimSpace(value))
	return parsed, err == nil
}

func parsePercentField(value string) (float64, bool) {
	clean := strings.TrimSuffix(strings.TrimSpace(value), "%")
	parsed, err := strconv.ParseFloat(clean, 64)
	return parsed, err == nil
}
