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
	ActiveNames []string         `json:"active_names"`
	ActiveModes []FlightModeFlag `json:"active_modes"`
	ModeCatalog []FlightModeFlag `json:"mode_catalog,omitempty"`
	UnknownMask uint32           `json:"unknown_mask,omitempty"`
}

type FlightModeFlag struct {
	Bit    uint8  `json:"bit"`
	Mask   uint32 `json:"mask"`
	ID     uint8  `json:"id"`
	Name   string `json:"name,omitempty"`
	Active bool   `json:"active"`
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
	out := &FlightModeState{
		Source:      status.Source,
		Flags:       status.ModeFlags,
		ActiveNames: []string{},
		ActiveModes: []FlightModeFlag{},
		ModeCatalog: []FlightModeFlag{},
	}
	var knownMask uint32
	for bit, definition := range definitions {
		if bit >= 32 {
			break
		}
		mask := uint32(1) << bit
		active := status.ModeFlags&mask != 0
		flag := FlightModeFlag{
			Bit:    uint8(bit),
			Mask:   mask,
			ID:     definition.ID,
			Name:   definition.Name,
			Active: active,
		}
		out.ModeCatalog = append(out.ModeCatalog, flag)
		knownMask |= mask
		if active {
			out.ActiveModes = append(out.ActiveModes, flag)
			out.ActiveNames = append(out.ActiveNames, definition.Name)
		}
	}
	out.UnknownMask = status.ModeFlags &^ knownMask
	return out
}
