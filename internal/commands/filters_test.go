package commands

import "testing"

func TestDecodeAdvancedConfig(t *testing.T) {
	config, err := DecodeAdvancedConfig(advancedConfigTestPayload())
	if err != nil {
		t.Fatalf("DecodeAdvancedConfig() error = %v", err)
	}
	if config.PIDProcessDenom != 4 || config.MotorProtocolName != "DSHOT600" || config.MotorIdlePercent != 5.5 {
		t.Fatalf("config = %+v", config)
	}
	if config.GyroCheckOverflowName != "ALL" || config.DebugMode != 5 || config.DebugModeCount != 80 {
		t.Fatalf("config = %+v", config)
	}
}

func TestDecodeFilterConfig(t *testing.T) {
	config, err := DecodeFilterConfig(filterConfigTestPayload())
	if err != nil {
		t.Fatalf("DecodeFilterConfig() error = %v", err)
	}
	if config.GyroLPF1StaticHz != 150 || config.GyroLPF2TypeName != "PT2" || config.DtermLPF2TypeName != "PT2" {
		t.Fatalf("config = %+v", config)
	}
	if config.DynamicLowpass.GyroMinHz != 100 || config.DynamicLowpass.DtermMaxHz != 200 || config.DynamicLowpass.DtermExpo != 5 {
		t.Fatalf("dynamic lowpass = %+v", config.DynamicLowpass)
	}
	if config.DynamicNotch.Q != 300 || config.DynamicNotch.MaxHz != 600 || config.DynamicNotch.Count != 3 {
		t.Fatalf("dynamic notch = %+v", config.DynamicNotch)
	}
	if config.RPMFilter.Harmonics != 3 || config.RPMFilter.Q != 500 || len(config.RPMFilter.Weights) != 3 || config.RPMFilter.Weights[2] != 60 {
		t.Fatalf("rpm filter = %+v", config.RPMFilter)
	}
}

func TestDecodeFilterConfigRejectsShortPayload(t *testing.T) {
	if _, err := DecodeFilterConfig([]byte{1, 2, 3}); err == nil {
		t.Fatal("DecodeFilterConfig() error = nil, want short payload error")
	}
}

func advancedConfigTestPayload() []byte {
	payload := []byte{1, 4, 0, 7}
	payload = appendU16Test(payload, 480)
	payload = appendU16Test(payload, 550)
	payload = append(payload, 0, 1, 0, 1, 32)
	payload = appendU16Test(payload, 125)
	payload = appendU16Test(payload, 10)
	payload = append(payload, 2, 5, 80)
	return payload
}

func filterConfigTestPayload() []byte {
	payload := []byte{90}
	payload = appendU16Test(payload, 100)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 0)
	payload = append(payload, 0, 1, 0)
	payload = appendU16Test(payload, 150)
	payload = appendU16Test(payload, 500)
	payload = append(payload, 0, 2)
	payload = appendU16Test(payload, 150)
	payload = append(payload, 2)
	payload = appendU16Test(payload, 100)
	payload = appendU16Test(payload, 400)
	payload = appendU16Test(payload, 80)
	payload = appendU16Test(payload, 200)
	payload = append(payload, 0, 0)
	payload = appendU16Test(payload, 300)
	payload = appendU16Test(payload, 100)
	payload = append(payload, 3, 100)
	payload = appendU16Test(payload, 600)
	payload = append(payload, 5, 3)
	payload = appendU16Test(payload, 50)
	payload = appendU16Test(payload, 500)
	payload = append(payload, 100, 80, 60)
	return payload
}
