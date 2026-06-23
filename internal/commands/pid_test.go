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

func TestEncodeRateProfile(t *testing.T) {
	payload := EncodeRateProfile(RateProfile{
		Axes: []RateAxis{
			{Axis: "roll", RCRate: 7, Expo: 10, Rate: 70, RateLimitDPS: 900},
			{Axis: "pitch", RCRate: 7, Expo: 9, Rate: 72, RateLimitDPS: 850},
			{Axis: "yaw", RCRate: 8, Expo: 5, Rate: 65, RateLimitDPS: 800},
		},
		Throttle: Throttle{
			MidPercent:   50,
			ExpoPercent:  20,
			HoverPercent: 45,
			LimitType:    1,
			LimitPercent: 80,
		},
		RatesType: 3,
	})
	want := []byte{7, 10, 70, 72, 65, 0, 50, 20, 0, 0, 5, 8, 7, 9, 1, 80, 0x84, 0x03, 0x52, 0x03, 0x20, 0x03, 3, 45}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
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

func TestEncodePIDAdvanced(t *testing.T) {
	advanced, err := DecodePIDAdvanced(pidAdvancedTestPayload())
	if err != nil {
		t.Fatalf("DecodePIDAdvanced() error = %v", err)
	}
	payload := EncodePIDAdvanced(*advanced)
	want := pidAdvancedTestPayload()
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestValidatePIDAdvancedRejectsInvalidFeedforwardAveraging(t *testing.T) {
	advanced := PIDAdvanced{FeedforwardAveraging: 4}
	if err := ValidatePIDAdvanced(advanced); err == nil {
		t.Fatal("ValidatePIDAdvanced() error = nil, want feedforward averaging error")
	}
}

func TestDecodeAndEncodeSimplifiedTuning(t *testing.T) {
	payload := simplifiedTuningTestPayload()
	tuning, err := DecodeSimplifiedTuning(payload)
	if err != nil {
		t.Fatalf("DecodeSimplifiedTuning() error = %v", err)
	}
	if tuning.PIDs.Mode != 2 || tuning.PIDs.MasterMultiplier != 100 || !tuning.Dterm.Enabled || tuning.Dterm.LPF1DynamicMaxHz != 170 || tuning.Gyro.LPF2StaticHz != 250 {
		t.Fatalf("tuning = %+v", tuning)
	}
	got := EncodeSimplifiedTuning(*tuning)
	if string(got) != string(payload) {
		t.Fatalf("EncodeSimplifiedTuning() = %v, want %v", got, payload)
	}
}

func TestValidateSimplifiedTuning(t *testing.T) {
	tuning, err := DecodeSimplifiedTuning(simplifiedTuningTestPayload())
	if err != nil {
		t.Fatalf("DecodeSimplifiedTuning() error = %v", err)
	}
	tuning.PIDs.Mode = 3
	if err := ValidateSimplifiedTuning(*tuning); err == nil {
		t.Fatal("ValidateSimplifiedTuning() error = nil, want mode error")
	}
	tuning.PIDs.Mode = 2
	tuning.Gyro.Multiplier = 9
	if err := ValidateSimplifiedTuning(*tuning); err == nil {
		t.Fatal("ValidateSimplifiedTuning() error = nil, want gyro multiplier error")
	}
}

func TestDecodeSimplifiedCalculatedPIDs(t *testing.T) {
	payload := []byte{50, 85, 35, 45}
	payload = appendU16Test(payload, 120)
	payload = append(payload, 52, 87, 37, 47)
	payload = appendU16Test(payload, 125)
	gains, err := DecodeSimplifiedCalculatedPIDs(payload)
	if err != nil {
		t.Fatalf("DecodeSimplifiedCalculatedPIDs() error = %v", err)
	}
	if len(gains) != 2 || gains[0].Axis != "roll" || gains[0].DMax != 45 || gains[1].F != 125 {
		t.Fatalf("gains = %+v", gains)
	}
}

func TestDecodeSimplifiedTuningValidation(t *testing.T) {
	validation, err := DecodeSimplifiedTuningValidation([]byte{1, 0, 1, 99})
	if err != nil {
		t.Fatalf("DecodeSimplifiedTuningValidation() error = %v", err)
	}
	if !validation.PIDsMatch || validation.GyroMatch || !validation.DtermMatch || validation.TrailingBytesIgnored != 1 {
		t.Fatalf("validation = %+v", validation)
	}
}

func simplifiedTuningTestPayload() []byte {
	payload := []byte{2, 100, 100, 100, 100, 100, 100, 100, 100}
	payload = appendU32Test(payload, 0)
	payload = appendU32Test(payload, 0)
	payload = append(payload, 1, 100)
	payload = appendU16Test(payload, 100)
	payload = appendU16Test(payload, 150)
	payload = appendU16Test(payload, 70)
	payload = appendU16Test(payload, 170)
	payload = appendU32Test(payload, 0)
	payload = appendU32Test(payload, 0)
	payload = append(payload, 1, 100)
	payload = appendU16Test(payload, 150)
	payload = appendU16Test(payload, 250)
	payload = appendU16Test(payload, 75)
	payload = appendU16Test(payload, 300)
	payload = appendU32Test(payload, 0)
	payload = appendU32Test(payload, 0)
	return payload
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
