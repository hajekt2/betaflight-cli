package generate

import (
	"strings"
	"testing"
)

func TestParseMSPCommands(t *testing.T) {
	const source = `
#define MSP_API_VERSION                 1    // out message: Get API version
#define MSP_SET_NAME                    11   // in message:  Sets board name
#define MSP_CAMERA_CONTROL              98   // in/out message: Camera control
#define MSP2_CLI_SETTING                0x3010
`
	commands, err := ParseMSPCommands("msp_protocol.h", strings.NewReader(source))
	if err != nil {
		t.Fatalf("ParseMSPCommands() error = %v", err)
	}
	if len(commands) != 4 {
		t.Fatalf("len(commands) = %d", len(commands))
	}
	tests := map[string]string{
		"MSP_API_VERSION":    "read",
		"MSP_SET_NAME":       "write",
		"MSP_CAMERA_CONTROL": "both",
		"MSP2_CLI_SETTING":   "unknown",
	}
	for _, command := range commands {
		if command.Direction != tests[command.Name] {
			t.Fatalf("%s direction = %q", command.Name, command.Direction)
		}
	}
	if commands[3].Code != 0x3010 || commands[3].Protocol != 2 {
		t.Fatalf("MSP2 command = %+v", commands[3])
	}
}

func TestWriteMSPRegistry(t *testing.T) {
	out, err := WriteMSPRegistry([]MSPCommand{{
		Name:      "MSP_API_VERSION",
		Code:      1,
		Protocol:  1,
		Direction: "read",
		Source:    "src/main/msp/msp_protocol.h",
		Line:      94,
	}}, "2025.12.0")
	if err != nil {
		t.Fatalf("WriteMSPRegistry() error = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "MSPAPIVersion uint16 = 0x1") {
		t.Fatalf("generated registry missing constant:\n%s", got)
	}
	if !strings.Contains(got, "Direction: DirectionRead") {
		t.Fatalf("generated registry missing direction:\n%s", got)
	}
}
