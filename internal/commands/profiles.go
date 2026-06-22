package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type ProfileStatus struct {
	Source          string            `json:"source"`
	PIDProfile      ProfileSelection  `json:"pid_profile"`
	RateProfile     *ProfileSelection `json:"rate_profile,omitempty"`
	BatteryProfile  *ProfileSelection `json:"battery_profile,omitempty"`
	ConfigStateFlag *uint8            `json:"config_state_flag,omitempty"`
	RebootRequired  *bool             `json:"reboot_required,omitempty"`
}

type ProfileSelection struct {
	Kind       string `json:"kind"`
	Index      uint8  `json:"index"`
	Count      *uint8 `json:"count,omitempty"`
	CLICommand string `json:"cli_command"`
}

func ReadProfileStatus(ctx context.Context, client *connection.Client) (*ProfileStatus, error) {
	status, err := readStatus(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("profile status unavailable: %w", err)
	}
	return profileStatusFromStatus(status), nil
}

func profileStatusFromStatus(status *Status) *ProfileStatus {
	out := &ProfileStatus{
		Source: status.Source,
		PIDProfile: ProfileSelection{
			Kind:       "pid",
			Index:      status.Profile,
			Count:      status.ProfileCount,
			CLICommand: fmt.Sprintf("profile %d", status.Profile),
		},
		ConfigStateFlag: status.ConfigStateFlag,
	}
	if status.RateProfile != nil {
		out.RateProfile = &ProfileSelection{
			Kind:       "rate",
			Index:      *status.RateProfile,
			Count:      status.RateProfileCount,
			CLICommand: fmt.Sprintf("rateprofile %d", *status.RateProfile),
		}
	}
	if status.BatteryProfile != nil {
		out.BatteryProfile = &ProfileSelection{
			Kind:       "battery",
			Index:      *status.BatteryProfile,
			Count:      status.BatteryProfileCount,
			CLICommand: fmt.Sprintf("battery_profile %d", *status.BatteryProfile),
		}
	}
	if status.ConfigStateFlag != nil {
		rebootRequired := *status.ConfigStateFlag&1 != 0
		out.RebootRequired = &rebootRequired
	}
	return out
}
