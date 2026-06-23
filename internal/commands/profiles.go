package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
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

type ProfileCopyKind string

const (
	ProfileCopyPID  ProfileCopyKind = "pid"
	ProfileCopyRate ProfileCopyKind = "rate"
)

type ProfileCopyRequest struct {
	Kind        ProfileCopyKind `json:"kind"`
	Source      uint8           `json:"source"`
	Destination uint8           `json:"destination"`
}

type ProfileCopyResult struct {
	Request      ProfileCopyRequest `json:"request"`
	MSPCode      uint16             `json:"msp_code"`
	MSPName      string             `json:"msp_name"`
	Acknowledged bool               `json:"acknowledged"`
	SaveRequired bool               `json:"save_required"`
}

func ReadProfileStatus(ctx context.Context, client *connection.Client) (*ProfileStatus, error) {
	status, err := readStatus(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("profile status unavailable: %w", err)
	}
	return profileStatusFromStatus(status), nil
}

func CopyProfile(ctx context.Context, client *connection.Client, request ProfileCopyRequest) (*ProfileCopyResult, error) {
	payload, err := EncodeProfileCopy(request)
	if err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPCopyProfile, payload); err != nil {
		return nil, fmt.Errorf("profile copy request failed: %w", err)
	}
	return &ProfileCopyResult{
		Request:      request,
		MSPCode:      msp.MSPCopyProfile,
		MSPName:      "MSP_COPY_PROFILE",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeProfileCopy(request ProfileCopyRequest) ([]byte, error) {
	var kind byte
	switch request.Kind {
	case ProfileCopyPID:
		kind = 0
	case ProfileCopyRate:
		kind = 1
	default:
		return nil, fmt.Errorf("unsupported profile copy kind %q", request.Kind)
	}
	return []byte{kind, request.Destination, request.Source}, nil
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
