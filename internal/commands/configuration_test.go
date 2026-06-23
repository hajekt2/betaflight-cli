package commands

import "testing"

func TestBuildConfigurationSummaryAndGuidance(t *testing.T) {
	profileCount := uint8(4)
	rate := uint8(2)
	rateCount := uint8(6)
	battery := uint8(1)
	batteryCount := uint8(3)
	rebootRequired := true
	armingBlocked := true
	system := &SystemStatus{
		Config: &SystemConfigLine{State: "CONFIGURED", UsedBytes: 3820, MaxBytes: 16384},
		Arming: &SystemArmingLine{Flags: []string{"RXLOSS"}},
	}
	runtime := &RuntimeStatus{
		Health: &RuntimeHealth{ArmingBlocked: &armingBlocked},
		Arming: &ArmingDisableState{
			Disabled:    true,
			ActiveNames: []string{"RXLOSS", "THROTTLE"},
		},
		FlightModes:    &FlightModeState{ActiveNames: []string{"ANGLE"}},
		RebootRequired: &rebootRequired,
	}
	profiles := &ProfileStatus{
		PIDProfile: ProfileSelection{Index: 3, Count: &profileCount},
		RateProfile: &ProfileSelection{
			Index: rate,
			Count: &rateCount,
		},
		BatteryProfile: &ProfileSelection{
			Index: battery,
			Count: &batteryCount,
		},
	}
	summary := buildConfigurationSummary(system, runtime, profiles)
	if summary.Configured == nil || !*summary.Configured || summary.ConfigurationState != "CONFIGURED" {
		t.Fatalf("config summary = %+v", summary)
	}
	if summary.RebootRequired == nil || !*summary.RebootRequired || summary.ArmingBlocked == nil || !*summary.ArmingBlocked {
		t.Fatalf("runtime summary = %+v", summary)
	}
	if len(summary.ArmingDisableNames) != 2 || summary.ArmingDisableNames[0] != "RXLOSS" || summary.ActiveFlightModes[0] != "ANGLE" {
		t.Fatalf("mode/arming summary = %+v", summary)
	}
	if summary.PIDProfile == nil || *summary.PIDProfile != 3 || summary.RateProfile == nil || *summary.RateProfile != 2 || summary.BatteryProfile == nil || *summary.BatteryProfile != 1 {
		t.Fatalf("profile summary = %+v", summary)
	}
	guidance := buildConfigurationGuidance(summary)
	if !guidance.ReadOnlySnapshot || !guidance.PlanBeforeApply || !guidance.SaveIsExplicit || !guidance.SaveMayReboot || !guidance.RequireConfirmation {
		t.Fatalf("guidance = %+v", guidance)
	}
	if guidance.UnsavedChangesKnown == nil || !*guidance.UnsavedChangesKnown || len(guidance.RecommendedNextActions) < 3 {
		t.Fatalf("guidance recommendations = %+v", guidance)
	}
}
