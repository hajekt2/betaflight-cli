package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type FirmwareSetting struct {
	Supported         bool   `json:"supported"`
	UnsupportedReason string `json:"unsupported_reason,omitempty"`
	Name              string `json:"name,omitempty"`
	Value             string `json:"value,omitempty"`
	Raw               string `json:"raw,omitempty"`
	Source            string `json:"source"`
	MSPCode           uint16 `json:"msp_code"`
	MSPName           string `json:"msp_name"`
	WriteScope        string `json:"write_scope"`
}

type FirmwareSettingInfo struct {
	Supported         bool   `json:"supported"`
	UnsupportedReason string `json:"unsupported_reason,omitempty"`
	Name              string `json:"name,omitempty"`
	Offset            uint16 `json:"offset"`
	TotalBytes        uint16 `json:"total_bytes,omitempty"`
	ChunkBytes        int    `json:"chunk_bytes,omitempty"`
	Complete          bool   `json:"complete"`
	Text              string `json:"text,omitempty"`
	MSPCode           uint16 `json:"msp_code"`
	MSPName           string `json:"msp_name"`
}

func ReadFirmwareSetting(ctx context.Context, client *connection.Client, name string) (*FirmwareSetting, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("setting name is required")
	}
	frame, err := client.Request(ctx, msp.MSP2CLISetting, []byte(name))
	if err != nil {
		var coded *connection.CodedError
		if errors.As(err, &coded) && coded.Code == "unsupported_msp" {
			return &FirmwareSetting{
				Supported:         false,
				UnsupportedReason: coded.Message,
				Name:              name,
				Source:            "MSP2_CLI_SETTING",
				MSPCode:           msp.MSP2CLISetting,
				MSPName:           "MSP2_CLI_SETTING",
				WriteScope:        "read_only_request",
			}, nil
		}
		return nil, fmt.Errorf("firmware setting %q unavailable: %w", name, err)
	}
	setting, err := DecodeFirmwareSetting(frame.Payload)
	if err != nil {
		return nil, err
	}
	setting.Supported = true
	setting.MSPCode = msp.MSP2CLISetting
	setting.MSPName = "MSP2_CLI_SETTING"
	setting.WriteScope = "read_only_request"
	return setting, nil
}

func ReadFirmwareSettingInfo(ctx context.Context, client *connection.Client, name string, offset uint16) (*FirmwareSettingInfo, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, fmt.Errorf("setting name is required")
	}
	payload := append([]byte(name), 0)
	payload = appendU16Payload(payload, offset)
	frame, err := client.Request(ctx, msp.MSP2CLISettingInfo, payload)
	if err != nil {
		var coded *connection.CodedError
		if errors.As(err, &coded) && coded.Code == "unsupported_msp" {
			return &FirmwareSettingInfo{
				Supported:         false,
				UnsupportedReason: coded.Message,
				Name:              name,
				Offset:            offset,
				MSPCode:           msp.MSP2CLISettingInfo,
				MSPName:           "MSP2_CLI_SETTING_INFO",
			}, nil
		}
		return nil, fmt.Errorf("firmware setting info %q unavailable: %w", name, err)
	}
	info, err := DecodeFirmwareSettingInfo(frame.Payload)
	if err != nil {
		return nil, err
	}
	info.Supported = true
	info.Name = name
	info.Offset = offset
	info.Complete = int(offset)+info.ChunkBytes >= int(info.TotalBytes)
	info.MSPCode = msp.MSP2CLISettingInfo
	info.MSPName = "MSP2_CLI_SETTING_INFO"
	return info, nil
}

func DecodeFirmwareSetting(payload []byte) (*FirmwareSetting, error) {
	raw := strings.TrimSpace(string(payload))
	if raw == "" {
		return nil, fmt.Errorf("MSP2_CLI_SETTING returned an empty response")
	}
	name, value, ok := strings.Cut(raw, "=")
	if !ok {
		return &FirmwareSetting{Supported: true, Raw: raw, Source: "MSP2_CLI_SETTING"}, nil
	}
	return &FirmwareSetting{
		Supported: true,
		Name:      strings.TrimSpace(name),
		Value:     strings.TrimSpace(value),
		Raw:       raw,
		Source:    "MSP2_CLI_SETTING",
	}, nil
}

func DecodeFirmwareSettingInfo(payload []byte) (*FirmwareSettingInfo, error) {
	if len(payload) < 2 {
		return nil, fmt.Errorf("MSP2_CLI_SETTING_INFO payload is shorter than total-size header")
	}
	r := msp.NewPayloadReader(payload)
	total, err := r.U16()
	if err != nil {
		return nil, msp.RequireNoShort(err, "MSP2_CLI_SETTING_INFO total size")
	}
	chunk, err := r.Bytes(r.Remaining())
	if err != nil {
		return nil, err
	}
	return &FirmwareSettingInfo{
		Supported:  true,
		TotalBytes: total,
		ChunkBytes: len(chunk),
		Complete:   len(chunk) >= int(total),
		Text:       string(chunk),
	}, nil
}
