package msp

import (
	"sort"
	"strings"
)

//go:generate go run ../../internal/generate/cmd/bfmeta -betaflight-src ${BETAFLIGHT_SRC} -source-version=${BETAFLIGHT_VERSION} -out-msp codes_generated.go -out-settings ../../internal/settings/metadata_generated.go

type CommandDirection string

const (
	DirectionUnknown CommandDirection = "unknown"
	DirectionRead    CommandDirection = "read"
	DirectionWrite   CommandDirection = "write"
	DirectionBoth    CommandDirection = "both"
)

type CommandMeta struct {
	Name          string           `json:"name"`
	Code          uint16           `json:"code"`
	Protocol      uint8            `json:"protocol"`
	Direction     CommandDirection `json:"direction"`
	SourceVersion string           `json:"source_version"`
	Source        string           `json:"source"`
	Line          int              `json:"line"`
}

var commandRegistry = map[uint16]CommandMeta{}

func RegisterCommands(commands []CommandMeta) {
	for _, command := range commands {
		commandRegistry[command.Code] = command
	}
}

func ListCommands() []CommandMeta {
	commands := make([]CommandMeta, 0, len(commandRegistry))
	for code, command := range commandRegistry {
		command.Code = code
		commands = append(commands, command)
	}
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Code == commands[j].Code {
			return commands[i].Name < commands[j].Name
		}
		return commands[i].Code < commands[j].Code
	})
	return commands
}

func LookupCommand(code uint16) (CommandMeta, bool) {
	command, ok := commandRegistry[code]
	return command, ok
}

func LookupCommandByName(name string) (CommandMeta, bool) {
	for code, command := range commandRegistry {
		if strings.EqualFold(command.Name, name) {
			command.Code = code
			return command, true
		}
	}
	return CommandMeta{}, false
}

func IsLikelyWriteCode(code uint16) bool {
	if command, ok := LookupCommand(code); ok {
		switch command.Direction {
		case DirectionWrite, DirectionBoth:
			return true
		case DirectionRead:
			return false
		}
	}
	if code >= 200 && code <= 252 {
		return true
	}
	switch code {
	case 11, 33, 35, 37, 39, 41, 43, 45, 47, 49, 51, 53, 55, 57, 60, 62, 65, 68, 72, 76, 78, 81, 85, 87, 89, 91, 93, 95, 97, 99:
		return true
	// The pinned upstream MSP2 headers do not encode direction metadata.
	// Keep explicit setters and actuators conservative until generation can infer it.
	case MSP2CommonSetSerialConfig, MSP2BetaflightBind, MSP2SetMotorOutputReordering,
		MSP2SendDshotCommand, MSP2SetText, MSP2SetLedStripConfigValues, MSP2SetBatteryProfile:
		return true
	default:
		return false
	}
}
