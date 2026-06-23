package msp

import "strings"

//go:generate go run ../../internal/generate/cmd/bfmeta -betaflight-src ${BETAFLIGHT_SRC} -source-version=${BETAFLIGHT_VERSION} -out-msp codes_generated.go -out-settings ../../internal/settings/metadata_generated.go

type CommandDirection string

const (
	DirectionUnknown CommandDirection = "unknown"
	DirectionRead    CommandDirection = "read"
	DirectionWrite   CommandDirection = "write"
	DirectionBoth    CommandDirection = "both"
)

type CommandMeta struct {
	Name      string           `json:"name"`
	Code      uint16           `json:"code"`
	Protocol  uint8            `json:"protocol"`
	Direction CommandDirection `json:"direction"`
	Source    string           `json:"source"`
	Line      int              `json:"line"`
}

var commandRegistry = map[uint16]CommandMeta{}

func RegisterCommands(commands []CommandMeta) {
	for _, command := range commands {
		commandRegistry[command.Code] = command
	}
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
		return command.Direction == DirectionWrite || command.Direction == DirectionBoth
	}
	if code >= 200 && code <= 252 {
		return true
	}
	switch code {
	case 11, 33, 35, 37, 39, 41, 43, 45, 47, 49, 51, 53, 55, 57, 60, 62, 65, 68, 72, 76, 78, 81, 85, 87, 89, 91, 93, 95, 97, 99:
		return true
	case 0x100a, 0x3002, 0x3003, 0x3007, 0x3009, 0x300f:
		return true
	default:
		return false
	}
}
