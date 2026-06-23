package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

var defaultTextFields = []TextField{
	{Type: msp.MSP2TextPilotName, Key: "pilot_name", Name: "Pilot name"},
	{Type: msp.MSP2TextCraftName, Key: "craft_name", Name: "Craft name"},
	{Type: msp.MSP2TextPIDProfileName, Key: "pid_profile_name", Name: "PID profile name"},
	{Type: msp.MSP2TextRateProfileName, Key: "rate_profile_name", Name: "Rate profile name"},
	{Type: msp.MSP2TextBuildKey, Key: "build_key", Name: "Build key", ReadOnly: true},
	{Type: msp.MSP2TextReleaseName, Key: "release_name", Name: "Release name", ReadOnly: true},
	{Type: msp.MSP2TextBatteryProfileName, Key: "battery_profile_name", Name: "Battery profile name"},
}

type TextStatus struct {
	Source   string            `json:"source"`
	Fields   []TextValue       `json:"fields"`
	ByKey    map[string]string `json:"by_key"`
	Warnings []string          `json:"warnings,omitempty"`
}

type TextField struct {
	Type     uint8  `json:"type"`
	Key      string `json:"key"`
	Name     string `json:"name"`
	ReadOnly bool   `json:"read_only,omitempty"`
}

type TextValue struct {
	TextField
	Value string `json:"value"`
}

func ReadTextStatus(ctx context.Context, client *connection.Client) (*TextStatus, error) {
	status := &TextStatus{
		Source: "MSP2_GET_TEXT",
		Fields: []TextValue{},
		ByKey:  map[string]string{},
	}
	for _, field := range defaultTextFields {
		value, err := readText(ctx, client, field.Type)
		if err != nil {
			status.Warnings = append(status.Warnings, fmt.Sprintf("%s unavailable: %v", field.Key, err))
			continue
		}
		item := TextValue{TextField: field, Value: value}
		status.Fields = append(status.Fields, item)
		status.ByKey[field.Key] = value
	}
	if len(status.Fields) == 0 {
		return nil, fmt.Errorf("no MSP2 text fields available")
	}
	return status, nil
}

func readText(ctx context.Context, client *connection.Client, textType uint8) (string, error) {
	frame, err := client.Request(ctx, msp.MSP2GetText, []byte{textType})
	if err != nil {
		return "", err
	}
	value, err := DecodeTextResponse(frame.Payload, textType)
	if err != nil {
		return "", err
	}
	return value, nil
}

func DecodeTextResponse(payload []byte, wantType uint8) (string, error) {
	r := msp.NewPayloadReader(payload)
	gotType, err := r.U8()
	if err != nil {
		return "", msp.RequireNoShort(err, "text type")
	}
	if gotType != wantType {
		return "", fmt.Errorf("MSP2_GET_TEXT returned text type %d, want %d", gotType, wantType)
	}
	text, err := r.PString()
	if err != nil {
		return "", msp.RequireNoShort(err, "text value")
	}
	if r.Remaining() != 0 {
		return "", fmt.Errorf("MSP2_GET_TEXT returned %d trailing byte(s)", r.Remaining())
	}
	return text, nil
}
