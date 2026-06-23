package commands

import "testing"

func TestDecodePIDGainsAndNames(t *testing.T) {
	names := DecodePIDNames([]byte("ROLL;PITCH;YAW;LEVEL;MAG;"))
	gains, err := DecodePIDGains([]byte{45, 80, 30, 47, 84, 34}, names)
	if err != nil {
		t.Fatalf("DecodePIDGains() error = %v", err)
	}
	if len(gains) != 2 || gains[0].Name != "ROLL" || gains[0].P != 45 || gains[1].Name != "PITCH" || gains[1].D != 34 {
		t.Fatalf("gains = %+v", gains)
	}
}

func TestDecodePIDGainsRejectsPartialTriplet(t *testing.T) {
	if _, err := DecodePIDGains([]byte{45, 80}, defaultPIDNames); err == nil {
		t.Fatal("DecodePIDGains() error = nil, want partial triplet error")
	}
}

func TestEncodePIDGains(t *testing.T) {
	payload := EncodePIDGains([]PIDGain{
		{P: 45, I: 80, D: 30},
		{P: 47, I: 84, D: 34},
	})
	want := []byte{45, 80, 30, 47, 84, 34}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestDecodeRateProfile(t *testing.T) {
	payload := []byte{7, 10, 70, 72, 65, 0, 50, 20}
	payload = appendU16Test(payload, 0)
	payload = append(payload, 5, 8, 7, 9, 1, 80)
	payload = appendU16Test(payload, 900)
	payload = appendU16Test(payload, 850)
	payload = appendU16Test(payload, 800)
	payload = append(payload, 3, 45)

	profile, err := DecodeRateProfile(payload)
	if err != nil {
		t.Fatalf("DecodeRateProfile() error = %v", err)
	}
	if profile.RatesTypeName != "ACTUAL" || profile.Throttle.LimitTypeName != "SCALE" || profile.Throttle.HoverValue != 0.45 {
		t.Fatalf("profile = %+v", profile)
	}
	if profile.Axes[0].Axis != "roll" || profile.Axes[0].RCRate != 7 || profile.Axes[0].RateLimitDPS != 900 {
		t.Fatalf("roll axis = %+v", profile.Axes[0])
	}
	if profile.Axes[1].Axis != "pitch" || profile.Axes[1].Expo != 9 || profile.Axes[2].Axis != "yaw" || profile.Axes[2].Rate != 65 {
		t.Fatalf("axes = %+v", profile.Axes)
	}
}

func TestDecodePIDAdvanced(t *testing.T) {
	advanced, err := DecodePIDAdvanced(pidAdvancedTestPayload())
	if err != nil {
		t.Fatalf("DecodePIDAdvanced() error = %v", err)
	}
	if advanced.FeedforwardTransition != 20 || advanced.AntiGravityGain != 3500 || advanced.FeedforwardPitch != 125 {
		t.Fatalf("advanced = %+v", advanced)
	}
	if advanced.DMaxRoll != 40 || advanced.MotorOutputLimit != 95 || advanced.TPA.Mode != 2 || advanced.TPA.Rate != 15 || advanced.TPA.Breakpoint != 1350 {
		t.Fatalf("advanced = %+v", advanced)
	}
}

func TestDecodePIDAdvancedRejectsShortPayload(t *testing.T) {
	if _, err := DecodePIDAdvanced([]byte{1, 2, 3}); err == nil {
		t.Fatal("DecodePIDAdvanced() error = nil, want short payload error")
	}
}

func pidAdvancedTestPayload() []byte {
	payload := appendU16Test(nil, 0)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 0)
	payload = append(payload, 0, 0, 20, 0, 0, 0, 0)
	payload = appendU16Test(payload, 100)
	payload = appendU16Test(payload, 120)
	payload = append(payload, 55, 0)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 3500)
	payload = appendU16Test(payload, 0)
	payload = append(payload, 1, 0, 2, 1, 0, 5, 20)
	payload = appendU16Test(payload, 120)
	payload = appendU16Test(payload, 125)
	payload = appendU16Test(payload, 120)
	payload = append(payload, 0, 40, 42, 0, 35, 50, 0, 20, 15, 95, 0, 30, 2, 10, 20, 30, 90, 5, 10, 2, 15)
	payload = appendU16Test(payload, 1350)
	return payload
}
