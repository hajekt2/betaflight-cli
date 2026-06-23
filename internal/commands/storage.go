package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type StorageStatus struct {
	Dataflash *DataflashSummary `json:"dataflash,omitempty"`
	SDCard    *SDCardSummary    `json:"sdcard,omitempty"`
	Sources   map[string]string `json:"sources,omitempty"`
}

type DataflashSummary struct {
	Flags      uint8  `json:"flags"`
	Ready      bool   `json:"ready"`
	Supported  bool   `json:"supported"`
	Sectors    uint32 `json:"sectors"`
	TotalBytes uint32 `json:"total_bytes"`
	UsedBytes  uint32 `json:"used_bytes"`
	FreeBytes  uint32 `json:"free_bytes"`
}

type SDCardSummary struct {
	Flags          uint8  `json:"flags"`
	Supported      bool   `json:"supported"`
	State          uint8  `json:"state"`
	StateName      string `json:"state_name,omitempty"`
	LastError      uint8  `json:"last_error"`
	FreeKilobytes  uint32 `json:"free_kilobytes"`
	TotalKilobytes uint32 `json:"total_kilobytes"`
}

func ReadStorageStatus(ctx context.Context, client *connection.Client) (*StorageStatus, []string, error) {
	status := &StorageStatus{Sources: map[string]string{}}
	warnings := []string{}
	if frame, err := client.Request(ctx, msp.MSPDataflashSummary, nil); err == nil {
		summary, err := DecodeDataflashSummary(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_DATAFLASH_SUMMARY decode failed: %v", err))
		} else {
			status.Dataflash = summary
			status.Sources["dataflash"] = "MSP_DATAFLASH_SUMMARY"
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_DATAFLASH_SUMMARY unavailable: %v", err))
	}
	if frame, err := client.Request(ctx, msp.MSPSdcardSummary, nil); err == nil {
		summary, err := DecodeSDCardSummary(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_SDCARD_SUMMARY decode failed: %v", err))
		} else {
			status.SDCard = summary
			status.Sources["sdcard"] = "MSP_SDCARD_SUMMARY"
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_SDCARD_SUMMARY unavailable: %v", err))
	}
	if status.Dataflash == nil && status.SDCard == nil {
		return nil, warnings, fmt.Errorf("storage status unavailable")
	}
	return status, warnings, nil
}

func DecodeDataflashSummary(payload []byte) (*DataflashSummary, error) {
	const length = 13
	if len(payload) < length {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_DATAFLASH_SUMMARY size %d", len(payload), length)
	}
	r := msp.NewPayloadReader(payload)
	flags, err := r.U8()
	if err != nil {
		return nil, err
	}
	sectors, err := r.U32()
	if err != nil {
		return nil, err
	}
	total, err := r.U32()
	if err != nil {
		return nil, err
	}
	used, err := r.U32()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_DATAFLASH_SUMMARY returned %d trailing byte(s)", r.Remaining())
	}
	free := uint32(0)
	if total > used {
		free = total - used
	}
	return &DataflashSummary{
		Flags:      flags,
		Ready:      flags&1 != 0,
		Supported:  flags&2 != 0,
		Sectors:    sectors,
		TotalBytes: total,
		UsedBytes:  used,
		FreeBytes:  free,
	}, nil
}

func DecodeSDCardSummary(payload []byte) (*SDCardSummary, error) {
	const length = 11
	if len(payload) < length {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_SDCARD_SUMMARY size %d", len(payload), length)
	}
	r := msp.NewPayloadReader(payload)
	flags, err := r.U8()
	if err != nil {
		return nil, err
	}
	state, err := r.U8()
	if err != nil {
		return nil, err
	}
	lastError, err := r.U8()
	if err != nil {
		return nil, err
	}
	free, err := r.U32()
	if err != nil {
		return nil, err
	}
	total, err := r.U32()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_SDCARD_SUMMARY returned %d trailing byte(s)", r.Remaining())
	}
	return &SDCardSummary{
		Flags:          flags,
		Supported:      flags&1 != 0,
		State:          state,
		StateName:      sdCardStateName(state),
		LastError:      lastError,
		FreeKilobytes:  free,
		TotalKilobytes: total,
	}, nil
}

func sdCardStateName(state uint8) string {
	switch state {
	case 0:
		return "NOT_PRESENT"
	case 1:
		return "FATAL"
	case 2:
		return "CARD_INIT"
	case 3:
		return "FS_INIT"
	case 4:
		return "READY"
	default:
		return ""
	}
}
