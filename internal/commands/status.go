package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type RuntimeStatus struct {
	Runtime           *Status             `json:"runtime"`
	ActiveSensorNames []string            `json:"active_sensor_names,omitempty"`
	FlightModes       *FlightModeState    `json:"flight_modes,omitempty"`
	Arming            *ArmingDisableState `json:"arming,omitempty"`
	RebootRequired    *bool               `json:"reboot_required,omitempty"`
}

type FlightModeState struct {
	Source      string           `json:"source"`
	Flags       uint32           `json:"flags"`
	FlagBytes   []int            `json:"flag_bytes,omitempty"`
	ActiveNames []string         `json:"active_names"`
	ActiveModes []FlightModeFlag `json:"active_modes"`
	ModeCatalog []FlightModeFlag `json:"mode_catalog,omitempty"`
	UnknownMask uint32           `json:"unknown_mask,omitempty"`
}

type FlightModeFlag struct {
	Bit       uint8  `json:"bit"`
	ByteIndex uint8  `json:"byte_index"`
	BitIndex  uint8  `json:"bit_index"`
	Mask      uint32 `json:"mask,omitempty"`
	ID        uint8  `json:"id"`
	Name      string `json:"name,omitempty"`
	Active    bool   `json:"active"`
}

func ReadRuntimeStatus(ctx context.Context, client *connection.Client) (*RuntimeStatus, error) {
	status, err := readStatus(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("runtime status unavailable: %w", err)
	}
	out := &RuntimeStatus{
		Runtime:           status,
		ActiveSensorNames: activeSensorNames(status.ActiveSensors),
		Arming:            DecodeArmingDisableState(status),
	}
	if definitions, err := readModeDefinitions(ctx, client); err == nil {
		out.FlightModes = DecodeFlightModeState(status, definitions)
	}
	if status.ConfigStateFlag != nil {
		rebootRequired := *status.ConfigStateFlag&1 != 0
		out.RebootRequired = &rebootRequired
	}
	return out, nil
}

func DecodeFlightModeState(status *Status, definitions []ModeDefinition) *FlightModeState {
	if status == nil {
		return nil
	}
	flagBytes := status.ModeFlagsBytes
	if len(flagBytes) == 0 {
		flagBytes = legacyModeFlagBytes(status.ModeFlags)
	}
	out := &FlightModeState{
		Source:      status.Source,
		Flags:       status.ModeFlags,
		FlagBytes:   flagBytes,
		ActiveNames: []string{},
		ActiveModes: []FlightModeFlag{},
		ModeCatalog: []FlightModeFlag{},
	}
	for bit, definition := range definitions {
		if bit >= len(flagBytes)*8 {
			break
		}
		mask, active := modeFlagBit(flagBytes, bit)
		flag := FlightModeFlag{
			Bit:       uint8(bit),
			ByteIndex: uint8(bit / 8),
			BitIndex:  uint8(bit % 8),
			Mask:      mask,
			ID:        definition.ID,
			Name:      definition.Name,
			Active:    active,
		}
		out.ModeCatalog = append(out.ModeCatalog, flag)
		if active {
			out.ActiveModes = append(out.ActiveModes, flag)
			out.ActiveNames = append(out.ActiveNames, definition.Name)
		}
	}
	out.UnknownMask = unknownLegacyModeMask(status.ModeFlags, len(definitions))
	return out
}

func legacyModeFlagBytes(flags uint32) []int {
	return []int{
		int(byte(flags)),
		int(byte(flags >> 8)),
		int(byte(flags >> 16)),
		int(byte(flags >> 24)),
	}
}

func modeFlagBit(flagBytes []int, bit int) (uint32, bool) {
	if bit < 0 || bit >= len(flagBytes)*8 {
		return 0, false
	}
	mask := uint32(0)
	if bit < 32 {
		mask = uint32(1) << bit
	}
	return mask, flagBytes[bit/8]&(1<<uint(bit%8)) != 0
}

func unknownLegacyModeMask(flags uint32, knownModes int) uint32 {
	if knownModes >= 32 {
		return 0
	}
	if knownModes <= 0 {
		return flags
	}
	return flags &^ ((uint32(1) << knownModes) - 1)
}
