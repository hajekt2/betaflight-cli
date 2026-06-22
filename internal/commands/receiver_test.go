package commands

import "testing"

func TestDecodeReceiverConfig(t *testing.T) {
	payload := []byte{2}
	payload = appendU16Test(payload, 2000)
	payload = appendU16Test(payload, 1500)
	payload = appendU16Test(payload, 1000)
	payload = append(payload, 0)
	payload = appendU16Test(payload, 885)
	payload = appendU16Test(payload, 2115)
	payload = append(payload, 0, 0)
	payload = appendU16Test(payload, 1350)
	payload = append(payload, 0)
	payload = appendU32Test(payload, 0)
	payload = append(payload, 0, 10, 0, 0, 50, 60, 70, 0, 0, 80, 1)
	payload = append(payload, 1, 2, 3, 4, 5, 6, 7)
	config, err := DecodeReceiverConfig(payload)
	if err != nil {
		t.Fatalf("DecodeReceiverConfig() error = %v", err)
	}
	if config.SerialProvider != 2 || config.StickMax != 2000 || config.StickCenter != 1500 || config.StickMin != 1000 {
		t.Fatalf("config = %+v", config)
	}
	if config.RXMinUsec != 885 || config.RXMaxUsec != 2115 || config.AirModeActivateThreshold != 1350 {
		t.Fatalf("config = %+v", config)
	}
	if config.FPVCamAngleDegrees != 10 {
		t.Fatalf("config = %+v", config)
	}
	if config.RCSmoothingSetpointCutoff == nil || *config.RCSmoothingSetpointCutoff != 50 {
		t.Fatalf("config = %+v", config)
	}
	if config.RCSmoothingThrottleCutoff == nil || *config.RCSmoothingThrottleCutoff != 60 {
		t.Fatalf("config = %+v", config)
	}
	if config.RCSmoothingAutoFactorThrottle == nil || *config.RCSmoothingAutoFactorThrottle != 70 {
		t.Fatalf("config = %+v", config)
	}
	if config.RCSmoothingAutoFactor == nil || *config.RCSmoothingAutoFactor != 80 {
		t.Fatalf("config = %+v", config)
	}
	if config.RCSmoothing == nil || *config.RCSmoothing != 1 {
		t.Fatalf("config = %+v", config)
	}
	if len(config.ELRSUID) != 6 || config.ELRSUID[5] != 6 {
		t.Fatalf("config = %+v", config)
	}
	if config.ELRSModelID == nil || *config.ELRSModelID != 7 {
		t.Fatalf("config = %+v", config)
	}
}

func TestDecodeReceiverConfigLegacyPayload(t *testing.T) {
	payload := []byte{2}
	payload = appendU16Test(payload, 2000)
	payload = appendU16Test(payload, 1500)
	payload = appendU16Test(payload, 1000)
	payload = append(payload, 0)
	payload = appendU16Test(payload, 885)
	payload = appendU16Test(payload, 2115)
	payload = append(payload, 0, 0)
	payload = appendU16Test(payload, 1350)
	payload = append(payload, 0)
	payload = appendU32Test(payload, 0)
	payload = append(payload, 0, 10, 0)
	config, err := DecodeReceiverConfig(payload)
	if err != nil {
		t.Fatalf("DecodeReceiverConfig() error = %v", err)
	}
	if config.RCSmoothingSetpointCutoff != nil || config.RCSmoothingAutoFactor != nil || config.ELRSModelID != nil || len(config.ELRSUID) != 0 {
		t.Fatalf("config = %+v", config)
	}
}

func TestRCMapNames(t *testing.T) {
	names := rcMapNames([]uint8{0, 1, 3, 2, 4})
	if len(names) != 5 || names[0] != "ROLL" || names[2] != "THROTTLE" || names[4] != "AUX1" {
		t.Fatalf("names = %+v", names)
	}
}

func TestDecodeRXFailConfig(t *testing.T) {
	payload := []byte{0}
	payload = appendU16Test(payload, 1000)
	payload = append(payload, 1)
	payload = appendU16Test(payload, 1500)
	payload = append(payload, 2)
	payload = appendU16Test(payload, 1100)
	rows, err := DecodeRXFailConfig(payload)
	if err != nil {
		t.Fatalf("DecodeRXFailConfig() error = %v", err)
	}
	if len(rows) != 3 {
		t.Fatalf("rows = %+v", rows)
	}
	if rows[0].Name != "ROLL" || rows[0].ModeName != "AUTO" || rows[0].CLICommand != "rxfail 0 a" {
		t.Fatalf("row 0 = %+v", rows[0])
	}
	if rows[1].ModeName != "HOLD" || rows[1].CLICommand != "rxfail 1 h" {
		t.Fatalf("row 1 = %+v", rows[1])
	}
	if rows[2].ModeName != "SET" || !rows[2].RequiresValue || rows[2].CLICommand != "rxfail 2 s 1100" {
		t.Fatalf("row 2 = %+v", rows[2])
	}
}

func TestDecodeRXFailConfigRejectsPartialRow(t *testing.T) {
	if _, err := DecodeRXFailConfig([]byte{0, 1}); err == nil {
		t.Fatal("DecodeRXFailConfig() error = nil, want partial row error")
	}
}

func appendU16Test(dst []byte, v uint16) []byte {
	return append(dst, byte(v), byte(v>>8))
}

func appendU32Test(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
