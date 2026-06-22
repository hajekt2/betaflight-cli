package generate

import (
	"bufio"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type MSPCommand struct {
	Name      string
	Code      uint16
	Protocol  uint8
	Direction string
	Source    string
	Line      int
}

var defineRE = regexp.MustCompile(`^\s*#define\s+(MSP2?_[A-Z0-9_]+)\s+((?:0x)?[0-9A-Fa-f]+)\b(?:\s*//\s*(.*))?$`)

func ParseMSPCommands(source string, r io.Reader) ([]MSPCommand, error) {
	var commands []MSPCommand
	scanner := bufio.NewScanner(r)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		match := defineRE.FindStringSubmatch(scanner.Text())
		if match == nil {
			continue
		}
		if match[1] == "MSP_PROTOCOL_VERSION" {
			continue
		}
		code, err := strconv.ParseUint(match[2], 0, 16)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: parse MSP code %q: %w", source, lineNumber, match[2], err)
		}
		protocol := uint8(1)
		if strings.HasPrefix(match[1], "MSP2_") {
			protocol = 2
		}
		commands = append(commands, MSPCommand{
			Name:      match[1],
			Code:      uint16(code),
			Protocol:  protocol,
			Direction: directionFromComment(match[3]),
			Source:    source,
			Line:      lineNumber,
		})
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	sort.Slice(commands, func(i, j int) bool {
		if commands[i].Code == commands[j].Code {
			return commands[i].Name < commands[j].Name
		}
		return commands[i].Code < commands[j].Code
	})
	return commands, nil
}

func directionFromComment(comment string) string {
	comment = strings.ToLower(comment)
	switch {
	case strings.Contains(comment, "in/out message"):
		return "both"
	case strings.Contains(comment, "in message"):
		return "write"
	case strings.Contains(comment, "out message"):
		return "read"
	default:
		return "unknown"
	}
}
