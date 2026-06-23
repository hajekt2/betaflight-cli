package commands

import "testing"

func TestDecodeReceiverConfig(t *testing.T) {
	payload := receiverConfigTestPayload()
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

func TestEncodeReceiverConfig(t *testing.T) {
	config, err := DecodeReceiverConfig(receiverConfigTestPayload())
	if err != nil {
		t.Fatalf("DecodeReceiverConfig() error = %v", err)
	}
	payload := EncodeReceiverConfig(*config)
	want := receiverConfigTestPayload()
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
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

func receiverConfigTestPayload() []byte {
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
	return payload
}

func TestRCMapNames(t *testing.T) {
	names := rcMapNames([]uint8{0, 1, 3, 2, 4})
	if len(names) != 5 || names[0] != "ROLL" || names[2] != "THROTTLE" || names[4] != "AUX1" {
		t.Fatalf("names = %+v", names)
	}
}

func TestEncodeRSSIChannel(t *testing.T) {
	got := EncodeRSSIChannel(8)
	if len(got) != 1 || got[0] != 8 {
		t.Fatalf("EncodeRSSIChannel() = %v", got)
	}
}

func TestEncodeRCMap(t *testing.T) {
	got := EncodeRCMap([]uint8{0, 1, 3, 2})
	want := []byte{0, 1, 3, 2}
	if string(got) != string(want) {
		t.Fatalf("EncodeRCMap() = %v, want %v", got, want)
	}
}

func TestDecodeAndEncodeRCDeadband(t *testing.T) {
	config, err := DecodeRCDeadband([]byte{5, 7, 3, 50, 0})
	if err != nil {
		t.Fatalf("DecodeRCDeadband() error = %v", err)
	}
	if config.Deadband != 5 || config.YawDeadband != 7 || config.PosHoldDeadband != 3 || config.Deadband3DThrottle != 50 {
		t.Fatalf("config = %+v", config)
	}
	payload := EncodeRCDeadband(*config)
	want := []byte{5, 7, 3, 50, 0}
	if len(payload) != len(want) {
		t.Fatalf("payload = %v", payload)
	}
	for i := range want {
		if payload[i] != want[i] {
			t.Fatalf("payload = %v, want %v", payload, want)
		}
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

func TestEncodeRXFailChannel(t *testing.T) {
	payload := EncodeRXFailChannel(RXFailChannel{Index: 2, Mode: 2, Value: 1100})
	want := []byte{2, 2, 0x4c, 0x04}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestValidateRXFailTable(t *testing.T) {
	config := RXFailTableSetConfig{Channels: []RXFailChannel{
		{Index: 0, Mode: 0, Value: 1000},
		{Index: 4, Mode: 1, Value: 1500},
		{Index: 5, Mode: 2, Value: 1100},
	}}
	if err := ValidateRXFailTable(config); err != nil {
		t.Fatalf("ValidateRXFailTable() error = %v", err)
	}
}

func TestValidateRXFailTableRejectsDuplicateIndex(t *testing.T) {
	config := RXFailTableSetConfig{Channels: []RXFailChannel{
		{Index: 2, Mode: 2, Value: 1100},
		{Index: 2, Mode: 1, Value: 1500},
	}}
	if err := ValidateRXFailTable(config); err == nil {
		t.Fatal("ValidateRXFailTable() error = nil, want duplicate index error")
	}
}

func TestValidateRXFailChannelRejectsAutoOnAux(t *testing.T) {
	if err := ValidateRXFailChannel(RXFailChannel{Index: 4, Mode: 0, Value: 1000}); err == nil {
		t.Fatal("ValidateRXFailChannel() error = nil, want AUX auto-mode error")
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
