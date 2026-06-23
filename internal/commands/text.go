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

type TextSetRequest struct {
	TextField
	Value string `json:"value"`
}

type TextSetResult struct {
	Request      TextSetRequest `json:"request"`
	MSPCode      uint16         `json:"msp_code"`
	MSPName      string         `json:"msp_name"`
	Acknowledged bool           `json:"acknowledged"`
	SaveRequired bool           `json:"save_required"`
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

func SetText(ctx context.Context, client *connection.Client, request TextSetRequest) (*TextSetResult, error) {
	payload, err := EncodeTextSet(request)
	if err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSP2SetText, payload); err != nil {
		return nil, fmt.Errorf("text set request failed: %w", err)
	}
	return &TextSetResult{
		Request:      request,
		MSPCode:      msp.MSP2SetText,
		MSPName:      "MSP2_SET_TEXT",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeTextSet(request TextSetRequest) ([]byte, error) {
	if request.ReadOnly {
		return nil, fmt.Errorf("%s is read-only", request.Key)
	}
	maxLen := textFieldMaxLength(request.Type)
	if maxLen == 0 {
		return nil, fmt.Errorf("unsupported text type %d", request.Type)
	}
	if len(request.Value) > maxLen {
		return nil, fmt.Errorf("%s is %d byte(s), maximum is %d", request.Key, len(request.Value), maxLen)
	}
	payload := []byte{request.Type, byte(len(request.Value))}
	return append(payload, []byte(request.Value)...), nil
}

func TextFieldByKey(key string) (TextField, bool) {
	for _, field := range defaultTextFields {
		if field.Key == key {
			return field, true
		}
	}
	return TextField{}, false
}

func WritableTextFields() []TextField {
	fields := []TextField{}
	for _, field := range defaultTextFields {
		if !field.ReadOnly {
			fields = append(fields, field)
		}
	}
	return fields
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

func textFieldMaxLength(textType uint8) int {
	switch textType {
	case msp.MSP2TextPilotName, msp.MSP2TextCraftName:
		return 16
	case msp.MSP2TextPIDProfileName, msp.MSP2TextRateProfileName, msp.MSP2TextBatteryProfileName:
		return 8
	default:
		return 0
	}
}
