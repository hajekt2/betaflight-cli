package commands

import "testing"

func TestProfileStatusFromStatus(t *testing.T) {
	pidCount := uint8(4)
	rate := uint8(2)
	rateCount := uint8(6)
	battery := uint8(1)
	batteryCount := uint8(3)
	configState := uint8(1)
	status := &Status{
		Source:              "MSP_STATUS_EX",
		Profile:             3,
		ProfileCount:        &pidCount,
		RateProfile:         &rate,
		RateProfileCount:    &rateCount,
		BatteryProfile:      &battery,
		BatteryProfileCount: &batteryCount,
		ConfigStateFlag:     &configState,
	}
	out := profileStatusFromStatus(status)
	if out.PIDProfile.Index != 3 || out.PIDProfile.CLICommand != "profile 3" || out.PIDProfile.Count == nil || *out.PIDProfile.Count != 4 {
		t.Fatalf("pid profile = %+v", out.PIDProfile)
	}
	if out.RateProfile == nil || out.RateProfile.Index != 2 || out.RateProfile.CLICommand != "rateprofile 2" || out.RateProfile.Count == nil || *out.RateProfile.Count != 6 {
		t.Fatalf("rate profile = %+v", out.RateProfile)
	}
	if out.BatteryProfile == nil || out.BatteryProfile.Index != 1 || out.BatteryProfile.CLICommand != "battery_profile 1" || out.BatteryProfile.Count == nil || *out.BatteryProfile.Count != 3 {
		t.Fatalf("battery profile = %+v", out.BatteryProfile)
	}
	if out.RebootRequired == nil || *out.RebootRequired != true {
		t.Fatalf("reboot required = %+v", out.RebootRequired)
	}
}
