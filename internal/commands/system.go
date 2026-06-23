package commands

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type SystemStatus struct {
	Source    string             `json:"source"`
	RawLines  []string           `json:"raw_lines"`
	Config    *SystemConfigLine  `json:"config,omitempty"`
	Devices   *SystemDevicesLine `json:"devices,omitempty"`
	GyroLine  string             `json:"gyro_line,omitempty"`
	GPSLine   string             `json:"gps_line,omitempty"`
	OSDLine   string             `json:"osd_line,omitempty"`
	FlashLine string             `json:"flash_line,omitempty"`
	BuildKey  *BuildKeyLine      `json:"build_key,omitempty"`
	Uptime    *SystemUptimeLine  `json:"uptime,omitempty"`
	Runtime   *SystemRuntimeLine `json:"runtime,omitempty"`
	Voltage   *SystemVoltageLine `json:"voltage,omitempty"`
	Arming    *SystemArmingLine  `json:"arming,omitempty"`
	Unparsed  []string           `json:"unparsed,omitempty"`
}

type SystemConfigLine struct {
	State     string `json:"state"`
	UsedBytes int    `json:"used_bytes"`
	MaxBytes  int    `json:"max_bytes"`
	Raw       string `json:"raw"`
}

type SystemDevicesLine struct {
	SPI       *int   `json:"spi,omitempty"`
	I2C       *int   `json:"i2c,omitempty"`
	I2CErrors *int   `json:"i2c_errors,omitempty"`
	Raw       string `json:"raw"`
}

type BuildKeyLine struct {
	Key     string `json:"key"`
	Release string `json:"release,omitempty"`
	Raw     string `json:"raw"`
}

type SystemUptimeLine struct {
	Seconds     int    `json:"seconds"`
	CurrentTime string `json:"current_time,omitempty"`
	Raw         string `json:"raw"`
}

type SystemRuntimeLine struct {
	CPULoadPercent int    `json:"cpu_load_percent"`
	CycleTimeUS    int    `json:"cycle_time_us"`
	GyroRateHz     int    `json:"gyro_rate_hz"`
	RXRateHz       int    `json:"rx_rate_hz"`
	SystemRateHz   int    `json:"system_rate_hz"`
	Raw            string `json:"raw"`
}

type SystemVoltageLine struct {
	VoltageV     float64 `json:"voltage_v"`
	CellCount    int     `json:"cell_count"`
	BatteryState string  `json:"battery_state"`
	Raw          string  `json:"raw"`
}

type SystemArmingLine struct {
	Flags []string `json:"flags"`
	Raw   string   `json:"raw"`
}

func ReadSystemStatus(ctx context.Context, client *connection.Client) (*SystemStatus, error) {
	lines, err := client.ExecCLI(ctx, "status")
	if err != nil {
		return nil, fmt.Errorf("system status unavailable: %w", err)
	}
	return ParseSystemStatus(lines), nil
}

func ParseSystemStatus(lines []string) *SystemStatus {
	status := &SystemStatus{Source: "CLI status", RawLines: lines, Unparsed: []string{}}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		switch {
		case strings.HasPrefix(trimmed, "CONFIG:"):
			if parsed, ok := parseSystemConfigLine(trimmed); ok {
				status.Config = parsed
			} else {
				status.Unparsed = append(status.Unparsed, trimmed)
			}
		case strings.HasPrefix(trimmed, "DEVICES DETECTED:"):
			status.Devices = parseSystemDevicesLine(trimmed)
		case strings.HasPrefix(trimmed, "GYRO:") || strings.HasPrefix(trimmed, "Gyro:"):
			status.GyroLine = trimmed
		case strings.HasPrefix(trimmed, "GPS:"):
			status.GPSLine = trimmed
		case strings.HasPrefix(trimmed, "OSD:"):
			status.OSDLine = trimmed
		case strings.HasPrefix(trimmed, "FLASH:"):
			status.FlashLine = trimmed
		case strings.HasPrefix(trimmed, "BUILD KEY:"):
			status.BuildKey = parseBuildKeyLine(trimmed)
		case strings.HasPrefix(trimmed, "System Uptime:"):
			if parsed, ok := parseSystemUptimeLine(trimmed); ok {
				status.Uptime = parsed
			} else {
				status.Unparsed = append(status.Unparsed, trimmed)
			}
		case strings.HasPrefix(trimmed, "CPU:"):
			if parsed, ok := parseSystemRuntimeLine(trimmed); ok {
				status.Runtime = parsed
			} else {
				status.Unparsed = append(status.Unparsed, trimmed)
			}
		case strings.HasPrefix(trimmed, "Voltage:"):
			if parsed, ok := parseSystemVoltageLine(trimmed); ok {
				status.Voltage = parsed
			} else {
				status.Unparsed = append(status.Unparsed, trimmed)
			}
		case strings.HasPrefix(trimmed, "Arming disable flags:"):
			status.Arming = parseSystemArmingLine(trimmed)
		default:
			status.Unparsed = append(status.Unparsed, trimmed)
		}
	}
	return status
}

func parseSystemConfigLine(line string) (*SystemConfigLine, bool) {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return nil, false
	}
	state := fields[1]
	open := strings.Index(line, "(")
	close := strings.LastIndex(line, ")")
	if open < 0 || close <= open {
		return nil, false
	}
	size := line[open+1 : close]
	parts := strings.Split(strings.ReplaceAll(size, "b", ""), "/")
	if len(parts) != 2 {
		return nil, false
	}
	used, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, false
	}
	maximum, err := strconv.Atoi(strings.TrimSpace(parts[1]))
	if err != nil {
		return nil, false
	}
	return &SystemConfigLine{State: state, UsedBytes: used, MaxBytes: maximum, Raw: line}, true
}

func parseSystemDevicesLine(line string) *SystemDevicesLine {
	devices := &SystemDevicesLine{Raw: line}
	after := strings.TrimSpace(strings.TrimPrefix(line, "DEVICES DETECTED:"))
	parts := strings.Split(after, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "SPI=") {
			if value, ok := parseLeadingInt(strings.TrimPrefix(part, "SPI=")); ok {
				devices.SPI = &value
			}
		}
		if strings.HasPrefix(part, "I2C=") {
			rest := strings.TrimPrefix(part, "I2C=")
			if value, ok := parseLeadingInt(rest); ok {
				devices.I2C = &value
			}
			if open := strings.Index(rest, "("); open >= 0 {
				if errors, ok := parseLeadingInt(rest[open+1:]); ok {
					devices.I2CErrors = &errors
				}
			}
		}
	}
	return devices
}

func parseBuildKeyLine(line string) *BuildKeyLine {
	value := strings.TrimSpace(strings.TrimPrefix(line, "BUILD KEY:"))
	out := &BuildKeyLine{Key: value, Raw: line}
	if open := strings.LastIndex(value, "("); open >= 0 && strings.HasSuffix(value, ")") {
		out.Key = strings.TrimSpace(value[:open])
		out.Release = strings.TrimSuffix(value[open+1:], ")")
	}
	return out
}

func parseSystemUptimeLine(line string) (*SystemUptimeLine, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(line, "System Uptime:"))
	parts := strings.SplitN(value, " seconds", 2)
	seconds, err := strconv.Atoi(strings.TrimSpace(parts[0]))
	if err != nil {
		return nil, false
	}
	out := &SystemUptimeLine{Seconds: seconds, Raw: line}
	if len(parts) > 1 {
		out.CurrentTime = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(parts[1]), ", Current Time:"))
	}
	return out, true
}

func parseSystemRuntimeLine(line string) (*SystemRuntimeLine, bool) {
	normalized := strings.NewReplacer("CPU:", "", "%", "", ",", "").Replace(line)
	fields := strings.Fields(normalized)
	if len(fields) < 13 {
		return nil, false
	}
	cpu, ok := parseIntAt(fields, 0)
	if !ok {
		return nil, false
	}
	cycle, ok := parseIntAt(fields, 3)
	if !ok {
		return nil, false
	}
	gyro, ok := parseIntAt(fields, 6)
	if !ok {
		return nil, false
	}
	rx, ok := parseIntAt(fields, 9)
	if !ok {
		return nil, false
	}
	system, ok := parseIntAt(fields, 12)
	if !ok {
		return nil, false
	}
	return &SystemRuntimeLine{CPULoadPercent: cpu, CycleTimeUS: cycle, GyroRateHz: gyro, RXRateHz: rx, SystemRateHz: system, Raw: line}, true
}

func parseSystemVoltageLine(line string) (*SystemVoltageLine, bool) {
	value := strings.TrimSpace(strings.TrimPrefix(line, "Voltage:"))
	parts := strings.SplitN(value, "V", 2)
	voltage, err := strconv.ParseFloat(strings.TrimSpace(parts[0]), 64)
	if err != nil || len(parts) < 2 {
		return nil, false
	}
	rest := parts[1]
	open := strings.Index(rest, "(")
	close := strings.Index(rest, ")")
	if open < 0 || close <= open {
		return nil, false
	}
	inside := rest[open+1 : close]
	insideParts := strings.SplitN(inside, " battery - ", 2)
	if len(insideParts) != 2 {
		return nil, false
	}
	cellText := strings.TrimSuffix(strings.TrimSpace(insideParts[0]), "S")
	cells, err := strconv.Atoi(cellText)
	if err != nil {
		return nil, false
	}
	return &SystemVoltageLine{VoltageV: voltage, CellCount: cells, BatteryState: strings.TrimSpace(insideParts[1]), Raw: line}, true
}

func parseSystemArmingLine(line string) *SystemArmingLine {
	value := strings.TrimSpace(strings.TrimPrefix(line, "Arming disable flags:"))
	flags := []string{}
	if value != "" {
		flags = strings.Fields(value)
	}
	return &SystemArmingLine{Flags: flags, Raw: line}
}

func parseLeadingInt(value string) (int, bool) {
	fields := strings.Fields(strings.TrimSpace(value))
	if len(fields) == 0 {
		return 0, false
	}
	return parseIntAt(fields, 0)
}

func parseIntAt(fields []string, index int) (int, bool) {
	if index >= len(fields) {
		return 0, false
	}
	value, err := strconv.Atoi(fields[index])
	return value, err == nil
}
