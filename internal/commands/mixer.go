package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type MixerStatus struct {
	Source             string      `json:"source"`
	Mixer              MixerMode   `json:"mixer"`
	YawMotorsReversed  *bool       `json:"yaw_motors_reversed,omitempty"`
	ReverseMotorDirRaw *uint8      `json:"reverse_motor_dir_raw,omitempty"`
	CLICommands        []string    `json:"cli_commands"`
	Catalog            []MixerMode `json:"catalog"`
	Warnings           []string    `json:"warnings,omitempty"`
}

type MixerMode struct {
	ID          uint8  `json:"id"`
	CLIName     string `json:"cli_name,omitempty"`
	DisplayName string `json:"display_name,omitempty"`
	MotorCount  uint8  `json:"motor_count,omitempty"`
	UsesServos  bool   `json:"uses_servos,omitempty"`
	Known       bool   `json:"known"`
}

func ReadMixerStatus(ctx context.Context, client *connection.Client) (*MixerStatus, error) {
	frame, err := client.Request(ctx, msp.MSPMixerConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("mixer config unavailable: %w", err)
	}
	status, err := DecodeMixerStatus(frame.Payload)
	if err != nil {
		return nil, err
	}
	status.Source = "MSP_MIXER_CONFIG"
	return status, nil
}

func DecodeMixerStatus(payload []byte) (*MixerStatus, error) {
	r := msp.NewPayloadReader(payload)
	modeID, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "mixer mode")
	}
	status := &MixerStatus{
		Mixer:       lookupMixerMode(modeID),
		CLICommands: []string{},
		Catalog:     mixerCatalog(),
	}
	if status.Mixer.Known {
		status.CLICommands = append(status.CLICommands, "mixer "+status.Mixer.CLIName)
	} else {
		status.Warnings = append(status.Warnings, fmt.Sprintf("unknown mixer mode %d", modeID))
	}
	if r.Remaining() > 0 {
		raw, err := r.U8()
		if err != nil {
			return nil, msp.RequireNoShort(err, "reverse motor direction")
		}
		reversed := raw != 0
		status.ReverseMotorDirRaw = &raw
		status.YawMotorsReversed = &reversed
		value := "OFF"
		if reversed {
			value = "ON"
		}
		status.CLICommands = append(status.CLICommands, "set yaw_motors_reversed = "+value)
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_MIXER_CONFIG returned %d trailing byte(s)", r.Remaining())
	}
	return status, nil
}

func lookupMixerMode(id uint8) MixerMode {
	if id == 0 || int(id) > len(mixerModes) {
		return MixerMode{ID: id, Known: false}
	}
	mode := mixerModes[id-1]
	mode.ID = id
	mode.Known = true
	return mode
}

func mixerCatalog() []MixerMode {
	out := make([]MixerMode, 0, len(mixerModes))
	for i := range mixerModes {
		mode := mixerModes[i]
		mode.ID = uint8(i + 1)
		mode.Known = true
		out = append(out, mode)
	}
	return out
}

var mixerModes = []MixerMode{
	{CLIName: "TRI", DisplayName: "Tricopter", MotorCount: 3, UsesServos: true},
	{CLIName: "QUADP", DisplayName: "Quad +", MotorCount: 4},
	{CLIName: "QUADX", DisplayName: "Quad X", MotorCount: 4},
	{CLIName: "BI", DisplayName: "Bicopter", MotorCount: 2, UsesServos: true},
	{CLIName: "GIMBAL", DisplayName: "Gimbal", UsesServos: true},
	{CLIName: "Y6", DisplayName: "Y6", MotorCount: 6},
	{CLIName: "HEX6", DisplayName: "Hex +", MotorCount: 6},
	{CLIName: "FLYING_WING", DisplayName: "Flying Wing", MotorCount: 1, UsesServos: true},
	{CLIName: "Y4", DisplayName: "Y4", MotorCount: 4},
	{CLIName: "HEX6X", DisplayName: "Hex X", MotorCount: 6},
	{CLIName: "OCTOX8", DisplayName: "Octo X8", MotorCount: 8},
	{CLIName: "OCTOFLATP", DisplayName: "Octo Flat +", MotorCount: 8},
	{CLIName: "OCTOFLATX", DisplayName: "Octo Flat X", MotorCount: 8},
	{CLIName: "AIRPLANE", DisplayName: "Airplane", MotorCount: 1, UsesServos: true},
	{CLIName: "HELI_120_CCPM", DisplayName: "Heli 120", MotorCount: 1, UsesServos: true},
	{CLIName: "HELI_90_DEG", DisplayName: "Heli 90", UsesServos: true},
	{CLIName: "VTAIL4", DisplayName: "V-tail Quad", MotorCount: 4},
	{CLIName: "HEX6H", DisplayName: "Hex H", MotorCount: 6},
	{CLIName: "PPM_TO_SERVO", DisplayName: "PPM to SERVO", UsesServos: true},
	{CLIName: "DUALCOPTER", DisplayName: "Dualcopter", MotorCount: 2, UsesServos: true},
	{CLIName: "SINGLECOPTER", DisplayName: "Singlecopter", MotorCount: 1, UsesServos: true},
	{CLIName: "ATAIL4", DisplayName: "A-tail Quad", MotorCount: 4},
	{CLIName: "CUSTOM", DisplayName: "Custom"},
	{CLIName: "CUSTOMAIRPLANE", DisplayName: "Custom Airplane", MotorCount: 1, UsesServos: true},
	{CLIName: "CUSTOMTRI", DisplayName: "Custom Tricopter", MotorCount: 3, UsesServos: true},
	{CLIName: "QUADX1234", DisplayName: "Quad X 1234", MotorCount: 4},
	{CLIName: "OCTOX8P", DisplayName: "Octo X8 +", MotorCount: 8},
}
