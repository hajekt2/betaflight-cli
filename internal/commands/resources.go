package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type ResourceDiagnostics struct {
	Source    string               `json:"source"`
	RawLines  []string             `json:"raw_lines"`
	Resources []ResourceAssignment `json:"resources"`
	Timers    []TimerAssignment    `json:"timers"`
	DMA       []DMAAssignment      `json:"dma"`
	Comments  []string             `json:"comments,omitempty"`
	Unparsed  []string             `json:"unparsed,omitempty"`
}

type ResourceAssignment struct {
	Kind   string `json:"kind"`
	Index  string `json:"index,omitempty"`
	Target string `json:"target"`
	Free   bool   `json:"free"`
	Raw    string `json:"raw"`
}

type TimerAssignment struct {
	Timer             string `json:"timer,omitempty"`
	Channel           string `json:"channel,omitempty"`
	Pin               string `json:"pin,omitempty"`
	Function          string `json:"function,omitempty"`
	Index             string `json:"index,omitempty"`
	AlternateFunction string `json:"alternate_function,omitempty"`
	Free              bool   `json:"free"`
	None              bool   `json:"none"`
	Raw               string `json:"raw"`
}

type DMAAssignment struct {
	Scope         string `json:"scope,omitempty"`
	Device        string `json:"device,omitempty"`
	Index         string `json:"index,omitempty"`
	Option        string `json:"option,omitempty"`
	Controller    string `json:"controller,omitempty"`
	Stream        string `json:"stream,omitempty"`
	Function      string `json:"function,omitempty"`
	FunctionIndex string `json:"function_index,omitempty"`
	Free          bool   `json:"free"`
	None          bool   `json:"none"`
	Raw           string `json:"raw"`
}

func ReadResourceDiagnostics(ctx context.Context, client *connection.Client) (*ResourceDiagnostics, error) {
	lines, err := client.ExecCLI(ctx, "resource show all")
	if err != nil {
		return nil, fmt.Errorf("resource diagnostics unavailable: %w", err)
	}
	return ParseResourceDiagnostics(lines), nil
}

func ParseResourceDiagnostics(lines []string) *ResourceDiagnostics {
	diagnostics := &ResourceDiagnostics{
		Source:    "CLI resource show all",
		RawLines:  lines,
		Resources: []ResourceAssignment{},
		Timers:    []TimerAssignment{},
		DMA:       []DMAAssignment{},
		Comments:  []string{},
		Unparsed:  []string{},
	}
	currentTimer := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		fields := strings.Fields(trimmed)
		if strings.HasPrefix(trimmed, "#") {
			diagnostics.Comments = append(diagnostics.Comments, trimmed)
			continue
		}
		if isResourceDiagnosticsHeader(trimmed) {
			diagnostics.Comments = append(diagnostics.Comments, trimmed)
			continue
		}
		if len(fields) >= 4 && fields[0] == "resource" {
			diagnostics.Resources = append(diagnostics.Resources, ResourceAssignment{
				Kind:   fields[1],
				Index:  fields[2],
				Target: fields[3],
				Free:   strings.EqualFold(fields[1], "FREE"),
				Raw:    trimmed,
			})
			continue
		}
		if resource, ok := parseActiveIOResource(trimmed); ok {
			diagnostics.Resources = append(diagnostics.Resources, resource)
			continue
		}
		if len(fields) >= 3 && fields[0] == "timer" {
			af := fields[2]
			diagnostics.Timers = append(diagnostics.Timers, TimerAssignment{
				Pin:               fields[1],
				AlternateFunction: af,
				Function:          af,
				Free:              strings.EqualFold(af, "FREE"),
				None:              strings.EqualFold(af, "NONE"),
				Raw:               trimmed,
			})
			continue
		}
		if timer, ok := parseActiveTimer(trimmed, currentTimer); ok {
			if timer.Timer != "" && timer.Channel == "" {
				currentTimer = timer.Timer
			}
			diagnostics.Timers = append(diagnostics.Timers, timer)
			continue
		}
		if len(fields) >= 4 && fields[0] == "dma" {
			diagnostics.DMA = append(diagnostics.DMA, parseDMAAssignment(fields, trimmed))
			continue
		}
		if dma, ok := parseActiveDMA(trimmed); ok {
			diagnostics.DMA = append(diagnostics.DMA, dma)
			continue
		}
		diagnostics.Unparsed = append(diagnostics.Unparsed, trimmed)
	}
	return diagnostics
}

func isResourceDiagnosticsHeader(line string) bool {
	return line == "(reboot to update)" ||
		strings.HasPrefix(line, "Currently active ") ||
		strings.Trim(line, "-") == ""
}

func parseActiveIOResource(line string) (ResourceAssignment, bool) {
	left, right, ok := strings.Cut(line, ":")
	if !ok || !looksLikeIOPin(left) {
		return ResourceAssignment{}, false
	}
	fields := strings.Fields(strings.TrimSpace(right))
	if len(fields) == 0 {
		return ResourceAssignment{}, false
	}
	assignment := ResourceAssignment{
		Kind:   fields[0],
		Target: strings.TrimSpace(left),
		Free:   strings.EqualFold(fields[0], "FREE"),
		Raw:    line,
	}
	if len(fields) > 1 {
		assignment.Index = fields[1]
	}
	return assignment, true
}

func looksLikeIOPin(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 3 {
		return false
	}
	return value[0] >= 'A' && value[0] <= 'Z' && value[1] >= '0' && value[1] <= '9' && value[2] >= '0' && value[2] <= '9'
}

func parseActiveTimer(line, currentTimer string) (TimerAssignment, bool) {
	left, right, ok := strings.Cut(line, ":")
	if !ok {
		return TimerAssignment{}, false
	}
	left = strings.TrimSpace(left)
	right = strings.TrimSpace(right)
	if strings.HasPrefix(left, "TIM") {
		timer := TimerAssignment{Timer: left, Raw: line}
		if right != "" {
			fields := strings.Fields(right)
			timer.Function = fields[0]
			timer.Free = strings.EqualFold(fields[0], "FREE")
			timer.None = strings.EqualFold(fields[0], "NONE")
			if len(fields) > 1 {
				timer.Index = fields[1]
			}
		}
		return timer, true
	}
	if strings.HasPrefix(left, "CH") && currentTimer != "" {
		fields := strings.Fields(right)
		if len(fields) == 0 {
			return TimerAssignment{}, false
		}
		timer := TimerAssignment{
			Timer:    currentTimer,
			Channel:  left,
			Function: fields[0],
			Free:     strings.EqualFold(fields[0], "FREE"),
			None:     strings.EqualFold(fields[0], "NONE"),
			Raw:      line,
		}
		if len(fields) > 1 {
			timer.Index = fields[1]
		}
		return timer, true
	}
	return TimerAssignment{}, false
}

func parseDMAAssignment(fields []string, raw string) DMAAssignment {
	assignment := DMAAssignment{
		Scope:  fields[1],
		Device: fields[2],
		Option: fields[len(fields)-1],
		Free:   strings.EqualFold(fields[len(fields)-1], "FREE"),
		None:   strings.EqualFold(fields[len(fields)-1], "NONE"),
		Raw:    raw,
	}
	if len(fields) >= 5 {
		assignment.Index = fields[3]
	}
	return assignment
}

func parseActiveDMA(line string) (DMAAssignment, bool) {
	left, right, ok := strings.Cut(line, ":")
	if !ok {
		return DMAAssignment{}, false
	}
	leftFields := strings.Fields(left)
	if len(leftFields) != 3 || !strings.HasPrefix(leftFields[0], "DMA") || leftFields[1] != "Stream" {
		return DMAAssignment{}, false
	}
	rightFields := strings.Fields(strings.TrimSpace(right))
	if len(rightFields) == 0 {
		return DMAAssignment{}, false
	}
	assignment := DMAAssignment{
		Controller: leftFields[0],
		Stream:     leftFields[2],
		Function:   rightFields[0],
		Free:       strings.EqualFold(rightFields[0], "FREE"),
		None:       strings.EqualFold(rightFields[0], "NONE"),
		Raw:        line,
	}
	if len(rightFields) > 1 {
		assignment.FunctionIndex = rightFields[1]
	}
	return assignment, true
}
