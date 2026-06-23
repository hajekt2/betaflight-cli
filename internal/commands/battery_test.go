package commands

import "testing"

func TestDecodeBatteryConfigAndProfile(t *testing.T) {
	config, err := DecodeBatteryConfig(batteryConfigTestPayload())
	if err != nil {
		t.Fatalf("DecodeBatteryConfig() error = %v", err)
	}
	if config.CapacityMAh != 1300 || config.VoltageMeterSourceName != "ADC" || config.CurrentMeterSourceName != "ADC" {
		t.Fatalf("config = %+v", config)
	}
	if config.MinCellVoltageV != 3.3 || config.MaxCellVoltageV != 4.35 || config.WarningCellVoltageV != 3.5 {
		t.Fatalf("config = %+v", config)
	}

	profile, err := DecodeBatteryProfile(batteryProfileTestPayload())
	if err != nil {
		t.Fatalf("DecodeBatteryProfile() error = %v", err)
	}
	if profile.Index != 0 || profile.FullCellVoltageV != 4.2 || profile.ForceCellCount != 4 || profile.ConsumptionWarningPercent != 20 {
		t.Fatalf("profile = %+v", profile)
	}
}

func TestDecodeBatteryRuntimeState(t *testing.T) {
	payload := []byte{4}
	payload = appendU16Test(payload, 1300)
	payload = append(payload, 160)
	payload = appendU16Test(payload, 123)
	payload = appendS16Test(payload, 456)
	payload = append(payload, 0)
	payload = appendU16Test(payload, 1599)
	state, err := DecodeBatteryRuntimeState(payload)
	if err != nil {
		t.Fatalf("DecodeBatteryRuntimeState() error = %v", err)
	}
	if state.CellCount != 4 || state.VoltageV != 15.99 || state.AmperageA != 4.56 || state.StateName != "OK" {
		t.Fatalf("state = %+v", state)
	}
}

func TestDecodeVoltageAndCurrentMeters(t *testing.T) {
	voltageMeters, err := DecodeVoltageMeters([]byte{10, 160, 40, 120, 80, 41})
	if err != nil {
		t.Fatalf("DecodeVoltageMeters() error = %v", err)
	}
	if len(voltageMeters) != 3 || voltageMeters[0].IDName != "BATTERY_1" || voltageMeters[1].IDName != "12V_1" || voltageMeters[2].IDName != "CELL_1" || voltageMeters[0].VoltageV != 16 {
		t.Fatalf("voltage meters = %+v", voltageMeters)
	}

	currentPayload := []byte{10}
	currentPayload = appendU16Test(currentPayload, 123)
	currentPayload = appendU16Test(currentPayload, 4560)
	currentPayload = append(currentPayload, 80)
	currentPayload = appendU16Test(currentPayload, 10)
	currentPayload = appendU16Test(currentPayload, 1000)
	currentMeters, err := DecodeCurrentMeters(currentPayload)
	if err != nil {
		t.Fatalf("DecodeCurrentMeters() error = %v", err)
	}
	if len(currentMeters) != 2 || currentMeters[0].IDName != "BATTERY_1" || currentMeters[1].IDName != "VIRTUAL_1" || currentMeters[0].AmperageA != 4.56 {
		t.Fatalf("current meters = %+v", currentMeters)
	}
}

func TestDecodeMeterConfigs(t *testing.T) {
	voltageConfigs, err := DecodeVoltageMeterConfigs([]byte{1, 5, 10, 0, 110, 10, 1})
	if err != nil {
		t.Fatalf("DecodeVoltageMeterConfigs() error = %v", err)
	}
	if len(voltageConfigs) != 1 || voltageConfigs[0].SensorTypeName != "ADC_RESISTOR_DIVIDER" || voltageConfigs[0].VBATScale != 110 {
		t.Fatalf("voltage configs = %+v", voltageConfigs)
	}

	currentPayload := []byte{1, 6, 10, 1}
	currentPayload = appendS16Test(currentPayload, 400)
	currentPayload = appendS16Test(currentPayload, -10)
	currentConfigs, err := DecodeCurrentMeterConfigs(currentPayload)
	if err != nil {
		t.Fatalf("DecodeCurrentMeterConfigs() error = %v", err)
	}
	if len(currentConfigs) != 1 || currentConfigs[0].SensorTypeName != "ADC" || currentConfigs[0].Scale != 400 || currentConfigs[0].Offset != -10 {
		t.Fatalf("current configs = %+v", currentConfigs)
	}
}

func TestEncodeVoltageMeterConfig(t *testing.T) {
	got := EncodeVoltageMeterConfig(VoltageMeterConfig{ID: 10, VBATScale: 110, VBATResDivVal: 10, VBATResDivMultiplier: 1})
	want := []byte{10, 110, 10, 1}
	if string(got) != string(want) {
		t.Fatalf("EncodeVoltageMeterConfig() = %v, want %v", got, want)
	}
}

func TestDecodeBatteryRejectsShortPayload(t *testing.T) {
	if _, err := DecodeBatteryConfig([]byte{1, 2}); err == nil {
		t.Fatal("DecodeBatteryConfig() error = nil, want short payload error")
	}
	if _, err := DecodeBatteryRuntimeState([]byte{1, 2}); err == nil {
		t.Fatal("DecodeBatteryRuntimeState() error = nil, want short payload error")
	}
}

func batteryConfigTestPayload() []byte {
	payload := []byte{33, 43, 35}
	payload = appendU16Test(payload, 1300)
	payload = append(payload, 1, 1)
	payload = appendU16Test(payload, 330)
	payload = appendU16Test(payload, 435)
	payload = appendU16Test(payload, 350)
	return payload
}

func batteryProfileTestPayload() []byte {
	payload := []byte{0}
	payload = appendU16Test(payload, 330)
	payload = appendU16Test(payload, 435)
	payload = appendU16Test(payload, 350)
	payload = appendU16Test(payload, 420)
	payload = appendU16Test(payload, 1300)
	payload = append(payload, 4, 20)
	return payload
}
