package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

var beeperModeNames = []string{
	"GYRO_CALIBRATED",
	"RX_LOST",
	"RX_LOST_LANDING",
	"DISARMING",
	"ARMING",
	"ARMING_GPS_FIX",
	"BAT_CRIT_LOW",
	"BAT_LOW",
	"GPS_STATUS",
	"RX_SET",
	"ACC_CALIBRATION",
	"ACC_CALIBRATION_FAIL",
	"READY_BEEP",
	"MULTI_BEEPS",
	"DISARM_REPEAT",
	"ARMED",
	"SYSTEM_INIT",
	"USB",
	"BLACKBOX_ERASE",
	"CRASHFLIP_MODE",
	"CAM_CONNECTION_OPEN",
	"CAM_CONNECTION_CLOSE",
	"ARMING_GPS_NO_FIX",
}

// BeeperModeNames returns the known beeper condition names supported by this CLI's
// BeeperConfig decoder.
func BeeperModeNames() []string {
	return append([]string(nil), beeperModeNames...)
}

// ValidateBeeperModeName validates and normalizes a beeper mode name.
func ValidateBeeperModeName(name string) (string, error) {
	if name == "" {
		return "", fmt.Errorf("mode name must not be empty")
	}
	mode := strings.ToUpper(name)
	for _, known := range beeperModeNames {
		if mode == known {
			return mode, nil
		}
	}
	return "", fmt.Errorf("unknown beeper mode %q", name)
}

type BeeperConfig struct {
	DisabledMask            uint32   `json:"disabled_mask"`
	Disabled                []string `json:"disabled"`
	DShotBeaconTone         uint8    `json:"dshot_beacon_tone"`
	DShotBeaconDisabledMask uint32   `json:"dshot_beacon_disabled_mask"`
	DShotBeaconDisabled     []string `json:"dshot_beacon_disabled"`
}

func ReadBeeperConfig(ctx context.Context, client *connection.Client) (*BeeperConfig, error) {
	frame, err := client.Request(ctx, msp.MSPBeeperConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("beeper config unavailable: %w", err)
	}
	return DecodeBeeperConfig(frame.Payload)
}

func DecodeBeeperConfig(payload []byte) (*BeeperConfig, error) {
	r := msp.NewPayloadReader(payload)
	disabled, err := r.U32()
	if err != nil {
		return nil, err
	}
	tone, err := r.U8()
	if err != nil {
		return nil, err
	}
	dshotDisabled, err := r.U32()
	if err != nil {
		return nil, err
	}
	return &BeeperConfig{
		DisabledMask:            disabled,
		Disabled:                beeperNamesForMask(disabled),
		DShotBeaconTone:         tone,
		DShotBeaconDisabledMask: dshotDisabled,
		DShotBeaconDisabled:     beeperNamesForMask(dshotDisabled),
	}, nil
}

func beeperNamesForMask(mask uint32) []string {
	out := []string{}
	for i, name := range beeperModeNames {
		if mask&(1<<i) != 0 {
			out = append(out, name)
		}
	}
	return out
}
