package commands

import (
	"context"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type TransponderConfig struct {
	Source       string                `json:"source"`
	Available    bool                  `json:"available"`
	Providers    []TransponderProvider `json:"providers"`
	Provider     uint8                 `json:"provider"`
	ProviderName string                `json:"provider_name,omitempty"`
	Data         []uint8               `json:"data,omitempty"`
	DataHex      string                `json:"data_hex,omitempty"`
	CLICommands  []string              `json:"cli_commands"`
	Warnings     []string              `json:"warnings,omitempty"`
}

type TransponderProvider struct {
	ID         uint8  `json:"id"`
	Name       string `json:"name,omitempty"`
	DataLength uint8  `json:"data_length"`
}

func ReadTransponderConfig(ctx context.Context, client *connection.Client) (*TransponderConfig, error) {
	frame, err := client.Request(ctx, msp.MSPTransponderConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("transponder config unavailable: %w", err)
	}
	return DecodeTransponderConfig(frame.Payload)
}

func DecodeTransponderConfig(payload []byte) (*TransponderConfig, error) {
	if len(payload) == 0 {
		return unavailableTransponderConfig(), nil
	}
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "transponder provider count")
	}
	if count == 0 && r.Remaining() == 0 {
		return unavailableTransponderConfig(), nil
	}
	providers := make([]TransponderProvider, 0, count)
	lengths := map[uint8]uint8{}
	for i := 0; i < int(count); i++ {
		provider, err := r.U8()
		if err != nil {
			return nil, msp.RequireNoShort(err, fmt.Sprintf("transponder provider %d", i))
		}
		length, err := r.U8()
		if err != nil {
			return nil, msp.RequireNoShort(err, fmt.Sprintf("transponder provider %d data length", i))
		}
		providers = append(providers, TransponderProvider{
			ID:         provider,
			Name:       transponderProviderName(provider),
			DataLength: length,
		})
		lengths[provider] = length
	}
	provider, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "active transponder provider")
	}
	config := &TransponderConfig{
		Source:       "MSP_TRANSPONDER_CONFIG",
		Available:    true,
		Providers:    providers,
		Provider:     provider,
		ProviderName: transponderProviderName(provider),
	}
	if provider != 0 {
		length, ok := lengths[provider]
		if !ok {
			return nil, fmt.Errorf("active transponder provider %d was not listed in provider requirements", provider)
		}
		data, err := r.Bytes(int(length))
		if err != nil {
			return nil, msp.RequireNoShort(err, "active transponder data")
		}
		config.Data = append([]uint8(nil), data...)
		config.DataHex = strings.ToUpper(hex.EncodeToString(data))
	} else if r.Remaining() > 0 {
		config.Warnings = append(config.Warnings, fmt.Sprintf("disabled transponder provider returned %d unexpected data byte(s)", r.Remaining()))
		data, err := r.Bytes(r.Remaining())
		if err != nil {
			return nil, err
		}
		config.Data = append([]uint8(nil), data...)
		config.DataHex = strings.ToUpper(hex.EncodeToString(data))
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_TRANSPONDER_CONFIG returned %d trailing byte(s)", r.Remaining())
	}
	config.CLICommands = transponderCLICommands(config)
	return config, nil
}

func unavailableTransponderConfig() *TransponderConfig {
	return &TransponderConfig{
		Source:      "MSP_TRANSPONDER_CONFIG",
		Available:   false,
		Providers:   []TransponderProvider{},
		Provider:    0,
		CLICommands: []string{"set transponder_provider = NONE"},
		Warnings:    []string{"transponder support is unavailable or not compiled into this firmware target"},
	}
}

func transponderCLICommands(config *TransponderConfig) []string {
	commands := []string{fmt.Sprintf("set transponder_provider = %s", transponderCLIProvider(config.Provider))}
	if len(config.Data) > 0 {
		values := make([]string, 0, len(config.Data))
		for _, b := range config.Data {
			values = append(values, fmt.Sprintf("%d", b))
		}
		commands = append(commands, "set transponder_data = "+strings.Join(values, ","))
	}
	return commands
}

func transponderProviderName(provider uint8) string {
	switch provider {
	case 0:
		return "NONE"
	case 1:
		return "ILAP"
	case 2:
		return "ARCITIMER"
	case 3:
		return "ERLT"
	default:
		return ""
	}
}

func transponderCLIProvider(provider uint8) string {
	if name := transponderProviderName(provider); name != "" {
		return name
	}
	return fmt.Sprintf("%d", provider)
}
