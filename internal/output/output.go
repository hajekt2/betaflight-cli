package output

import (
	"encoding/json"
	"fmt"
	"io"
)

const SchemaVersion = "1.0"

type Envelope struct {
	SchemaVersion string       `json:"schema_version"`
	OK            bool         `json:"ok"`
	Command       string       `json:"command"`
	Target        *Target      `json:"target,omitempty"`
	Data          any          `json:"data"`
	Warnings      []Warning    `json:"warnings"`
	Errors        []Error      `json:"errors"`
	SideEffects   []SideEffect `json:"side_effects"`
}

type Target struct {
	Port            string `json:"port,omitempty"`
	AutoDetected    bool   `json:"auto_detected,omitempty"`
	SelectionReason string `json:"selection_reason,omitempty"`
	Variant         string `json:"variant,omitempty"`
	FirmwareVersion string `json:"firmware_version,omitempty"`
	MSPAPIVersion   string `json:"msp_api_version,omitempty"`
	MSPProtocol     uint8  `json:"msp_protocol_version"`
}

type Warning struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type SideEffect struct {
	Type    string `json:"type"`
	Detail  string `json:"detail,omitempty"`
	Command string `json:"command,omitempty"`
}

func Success(command string, target *Target, data any) Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion,
		OK:            true,
		Command:       command,
		Target:        target,
		Data:          data,
		Warnings:      []Warning{},
		Errors:        []Error{},
		SideEffects:   []SideEffect{},
	}
}

func Failure(command string, target *Target, code, message string) Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion,
		OK:            false,
		Command:       command,
		Target:        target,
		Data:          map[string]any{},
		Warnings:      []Warning{},
		Errors: []Error{{
			Code:    code,
			Message: message,
		}},
		SideEffects: []SideEffect{},
	}
}

func Render(w io.Writer, format string, env Envelope) error {
	switch format {
	case "", "json":
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(env)
	case "text":
		return renderText(w, env)
	default:
		return fmt.Errorf("unknown output format %q", format)
	}
}

func renderText(w io.Writer, env Envelope) error {
	if !env.OK {
		for _, err := range env.Errors {
			fmt.Fprintf(w, "error: %s: %s\n", err.Code, err.Message)
		}
		return nil
	}
	switch data := env.Data.(type) {
	case string:
		_, err := fmt.Fprintln(w, data)
		return err
	case []string:
		for _, line := range data {
			fmt.Fprintln(w, line)
		}
		return nil
	default:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(env.Data)
	}
}
