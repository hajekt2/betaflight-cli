package commands

import (
	"context"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type ConfigurationStatus struct {
	Source   string                     `json:"source"`
	Summary  ConfigurationSummary       `json:"summary"`
	System   *SystemStatus              `json:"system,omitempty"`
	Runtime  *RuntimeStatus             `json:"runtime,omitempty"`
	Profiles *ProfileStatus             `json:"profiles,omitempty"`
	Guidance ConfigurationWriteGuidance `json:"guidance"`
	Warnings []string                   `json:"warnings,omitempty"`
}

type ConfigurationSummary struct {
	Configured          *bool    `json:"configured,omitempty"`
	ConfigurationState  string   `json:"configuration_state,omitempty"`
	RebootRequired      *bool    `json:"reboot_required,omitempty"`
	ArmingBlocked       *bool    `json:"arming_blocked,omitempty"`
	ArmingDisableNames  []string `json:"arming_disable_names,omitempty"`
	ActiveFlightModes   []string `json:"active_flight_modes,omitempty"`
	PIDProfile          *uint8   `json:"pid_profile,omitempty"`
	PIDProfileCount     *uint8   `json:"pid_profile_count,omitempty"`
	RateProfile         *uint8   `json:"rate_profile,omitempty"`
	RateProfileCount    *uint8   `json:"rate_profile_count,omitempty"`
	BatteryProfile      *uint8   `json:"battery_profile,omitempty"`
	BatteryProfileCount *uint8   `json:"battery_profile_count,omitempty"`
	ConfigUsedBytes     *int     `json:"config_used_bytes,omitempty"`
	ConfigMaxBytes      *int     `json:"config_max_bytes,omitempty"`
}

type ConfigurationWriteGuidance struct {
	ReadOnlySnapshot       bool     `json:"read_only_snapshot"`
	PlanBeforeApply        bool     `json:"plan_before_apply"`
	SaveIsExplicit         bool     `json:"save_is_explicit"`
	SaveMayReboot          bool     `json:"save_may_reboot"`
	RequireConfirmation    bool     `json:"require_confirmation_for_writes"`
	UnsavedChangesKnown    *bool    `json:"unsaved_changes_known,omitempty"`
	RecommendedNextActions []string `json:"recommended_next_actions"`
}

func ReadConfigurationStatus(ctx context.Context, client *connection.Client) (*ConfigurationStatus, []string) {
	status := &ConfigurationStatus{
		Source:   "MSP_STATUS_EX plus CLI status and active profile selectors",
		Warnings: []string{},
	}
	if system, err := ReadSystemStatus(ctx, client); err == nil {
		status.System = system
	} else {
		status.Warnings = append(status.Warnings, "system status unavailable: "+err.Error())
	}
	if runtime, err := ReadRuntimeStatus(ctx, client); err == nil {
		status.Runtime = runtime
	} else {
		status.Warnings = append(status.Warnings, "runtime status unavailable: "+err.Error())
	}
	if profiles, err := ReadProfileStatus(ctx, client); err == nil {
		status.Profiles = profiles
	} else {
		status.Warnings = append(status.Warnings, "profile status unavailable: "+err.Error())
	}
	status.Summary = buildConfigurationSummary(status.System, status.Runtime, status.Profiles)
	status.Guidance = buildConfigurationGuidance(status.Summary)
	return status, status.Warnings
}

func buildConfigurationSummary(system *SystemStatus, runtime *RuntimeStatus, profiles *ProfileStatus) ConfigurationSummary {
	var summary ConfigurationSummary
	if system != nil {
		if system.Config != nil {
			state := system.Config.State
			summary.ConfigurationState = state
			configured := state == "CONFIGURED"
			summary.Configured = &configured
			summary.ConfigUsedBytes = &system.Config.UsedBytes
			summary.ConfigMaxBytes = &system.Config.MaxBytes
		}
		if system.Arming != nil {
			summary.ArmingDisableNames = append([]string(nil), system.Arming.Flags...)
		}
	}
	if runtime != nil {
		if runtime.RebootRequired != nil {
			value := *runtime.RebootRequired
			summary.RebootRequired = &value
		}
		if runtime.Health != nil && runtime.Health.ArmingBlocked != nil {
			value := *runtime.Health.ArmingBlocked
			summary.ArmingBlocked = &value
		}
		if runtime.Arming != nil {
			if summary.ArmingBlocked == nil {
				value := runtime.Arming.Disabled
				summary.ArmingBlocked = &value
			}
			summary.ArmingDisableNames = append([]string(nil), runtime.Arming.ActiveNames...)
		}
		if runtime.FlightModes != nil {
			summary.ActiveFlightModes = append([]string(nil), runtime.FlightModes.ActiveNames...)
		}
	}
	if profiles != nil {
		summary.PIDProfile = &profiles.PIDProfile.Index
		summary.PIDProfileCount = profiles.PIDProfile.Count
		if profiles.RateProfile != nil {
			summary.RateProfile = &profiles.RateProfile.Index
			summary.RateProfileCount = profiles.RateProfile.Count
		}
		if profiles.BatteryProfile != nil {
			summary.BatteryProfile = &profiles.BatteryProfile.Index
			summary.BatteryProfileCount = profiles.BatteryProfile.Count
		}
		if summary.RebootRequired == nil && profiles.RebootRequired != nil {
			value := *profiles.RebootRequired
			summary.RebootRequired = &value
		}
	}
	return summary
}

func buildConfigurationGuidance(summary ConfigurationSummary) ConfigurationWriteGuidance {
	guidance := ConfigurationWriteGuidance{
		ReadOnlySnapshot:       true,
		PlanBeforeApply:        true,
		SaveIsExplicit:         true,
		SaveMayReboot:          true,
		RequireConfirmation:    true,
		RecommendedNextActions: []string{"use plan-only commands before --apply", "run save --yes only after reviewing applied changes"},
	}
	if summary.RebootRequired != nil {
		known := *summary.RebootRequired
		guidance.UnsavedChangesKnown = &known
		if known {
			guidance.RecommendedNextActions = append([]string{"review applied changes and run save --yes when ready"}, guidance.RecommendedNextActions...)
		}
	}
	if summary.ArmingBlocked != nil && *summary.ArmingBlocked {
		guidance.RecommendedNextActions = append(guidance.RecommendedNextActions, "resolve arming-disable flags before flight")
	}
	return guidance
}
