package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type RebootMode uint8

const (
	RebootFirmware RebootMode = iota
	RebootBootloaderROM
	RebootMSC
	RebootMSCUTC
	RebootBootloaderFlash
)

type RebootResult struct {
	Mode         RebootMode `json:"mode"`
	ModeName     string     `json:"mode_name"`
	MSPCode      uint16     `json:"msp_code"`
	Acknowledged bool       `json:"acknowledged"`
	MSCReady     *bool      `json:"msc_ready,omitempty"`
}

func SendReboot(ctx context.Context, client *connection.Client, mode RebootMode) (*RebootResult, error) {
	payload := []byte{byte(mode)}
	frame, err := client.Request(ctx, msp.MSPReboot, payload)
	if err != nil {
		return nil, fmt.Errorf("reboot request failed: %w", err)
	}
	result := &RebootResult{
		Mode:         mode,
		ModeName:     rebootModeName(mode),
		MSPCode:      msp.MSPReboot,
		Acknowledged: true,
	}
	if len(frame.Payload) > 0 {
		result.Mode = RebootMode(frame.Payload[0])
		result.ModeName = rebootModeName(result.Mode)
	}
	if len(frame.Payload) > 1 {
		ready := frame.Payload[1] != 0
		result.MSCReady = &ready
	}
	return result, nil
}

func rebootModeName(mode RebootMode) string {
	switch mode {
	case RebootFirmware:
		return "firmware"
	case RebootBootloaderROM:
		return "bootloader_rom"
	case RebootMSC:
		return "msc"
	case RebootMSCUTC:
		return "msc_utc"
	case RebootBootloaderFlash:
		return "bootloader_flash"
	default:
		return "unknown"
	}
}
