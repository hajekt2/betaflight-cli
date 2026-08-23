package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

// Upstream truth (Betaflight 2026.6.1):
//
//   - src/main/msp/msp.c, case MSP_STATUS_EX / MSP_STATUS (line 1131): after the
//     shared status fields (u16 cycle time, u16 i2c errors, u16 sensor bitmap,
//     u32 flight-mode flags, u8 pid profile, u16 cpu load, u8 pid profile count,
//     u8 rate profile index, u8 extended-flag-byte-count plus those bytes), the
//     EX variant appends u8 ARMING_DISABLE_FLAGS_COUNT then u32
//     getArmingDisableFlags() (sbufWriteU8 at line 1162, sbufWriteU32 at line 1164),
//     followed by u8 config-state flags, u16 CPU temperature and profile counts.
//   - src/main/msp/msp.c, case MSP_SET_ARMING_DISABLED (line 3898): the payload is
//     NOT an absolute mask. It is one u8 command where nonzero disables arming
//     (sets ARMING_DISABLED_MSP) and zero re-enables it (clears ARMING_DISABLED_MSP),
//     optionally followed by a second u8 controlling the runaway-takeoff
//     temporary-disable handling.
//   - src/main/fc/runtime_config.h, armingDisableFlags_e (line 59):
//     ARMING_DISABLED_MSP = 1 << 16. This is the ground-control arming-lock bit.
const (
	// gcsArmingDisableBit is the ARMING_DISABLED_MSP bit index (runtime_config.h:59).
	gcsArmingDisableBit = 16
	// GCSArmingDisableMask selects the MSP/GCS arming-disable flag in a flag word.
	GCSArmingDisableMask = uint32(1) << gcsArmingDisableBit
)

// ArmingLockState reports whether remote (MSP/GCS) arming lock is engaged and
// which arming-disable flags are currently active on the FC.
type ArmingLockState struct {
	Enabled     bool     `json:"enabled"`
	Flags       uint32   `json:"flags"`
	ActiveNames []string `json:"active_names"`
	Count       uint8    `json:"count"`
}

// ArmingLockSetResult is returned by SetArmingLock. Arming lock lives purely in
// runtime state on the FC, so there is no save step (no SaveRequired field).
type ArmingLockSetResult struct {
	Flags        uint32 `json:"flags"`
	MSPCode      uint16 `json:"msp_code"`
	MSPName      string `json:"msp_name"`
	Acknowledged bool   `json:"acknowledged"`
	Changed      bool   `json:"changed"`
}

// ReadArmingLockState fetches MSP_STATUS_EX and derives the arming-lock view.
func ReadArmingLockState(ctx context.Context, client *connection.Client) (*ArmingLockState, error) {
	frame, err := client.Request(ctx, msp.MSPStatusEx, nil)
	if err != nil {
		return nil, fmt.Errorf("arming lock state unavailable: %w", err)
	}
	status, err := DecodeStatusEx(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("arming lock state unavailable: %w", err)
	}
	return armingLockStateFromStatus(status), nil
}

// armingLockStateFromStatus projects the decoded status onto the arming-lock
// shape, reusing DecodeArmingDisableState for flag naming.
func armingLockStateFromStatus(status *Status) *ArmingLockState {
	if status == nil {
		return nil
	}
	state := DecodeArmingDisableState(&Status{
		Source:             status.Source,
		ArmingDisableCount: status.ArmingDisableCount,
		ArmingDisableFlags: status.ArmingDisableFlags,
	})
	if state == nil {
		return nil
	}
	names := state.ActiveNames
	if names == nil {
		names = []string{}
	}
	return &ArmingLockState{
		Enabled:     state.Flags&GCSArmingDisableMask != 0,
		Flags:       state.Flags,
		ActiveNames: names,
		Count:       state.FlagCount,
	}
}

// SetArmingLock toggles the ground-control arming lock through
// MSP_SET_ARMING_DISABLED. Per upstream msp.c the command takes a single u8
// (1 = engage lock, 0 = release); the FC itself manages the ARMING_DISABLED_MSP
// bit, so this is a read-modify-write around that one-bit gate. The change is
// runtime-only: nothing to save on the FC.
func SetArmingLock(ctx context.Context, client *connection.Client, lock bool) (*ArmingLockSetResult, error) {
	before, err := ReadArmingLockState(ctx, client)
	if err != nil {
		return nil, err
	}
	command := uint8(0)
	if lock {
		command = 1
	}
	if _, err := client.Request(ctx, msp.MSPSetArmingDisabled, []byte{command}); err != nil {
		return nil, fmt.Errorf("arming lock request failed: %w", err)
	}
	flags := before.Flags
	if !lock {
		flags &^= GCSArmingDisableMask
	} else {
		flags |= GCSArmingDisableMask
	}
	if after, err := ReadArmingLockState(ctx, client); err == nil && after != nil {
		flags = after.Flags
	}
	return &ArmingLockSetResult{
		Flags:        flags,
		MSPCode:      msp.MSPSetArmingDisabled,
		MSPName:      "MSP_SET_ARMING_DISABLED",
		Acknowledged: true,
		Changed:      (before.Flags&GCSArmingDisableMask != 0) != lock,
	}, nil
}
