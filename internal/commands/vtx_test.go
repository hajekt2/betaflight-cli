package commands

import "testing"

func TestDecodeVTXConfig(t *testing.T) {
	payload := []byte{3, 5, 8, 2, 1, 0xE5, 0x16, 1, 2, 0x1E, 0x16, 1, 5, 8, 3}
	config, err := DecodeVTXConfig(payload)
	if err != nil {
		t.Fatalf("DecodeVTXConfig() error = %v", err)
	}
	if config.Type != 3 || config.TypeName != "SMARTAUDIO" {
		t.Fatalf("config = %+v", config)
	}
	if config.Band != 5 || config.Channel != 8 || config.Power != 2 {
		t.Fatalf("config = %+v", config)
	}
	if !config.PitMode || config.FrequencyMHz != 5861 || !config.DeviceReady || config.LowPowerDisarm != 2 {
		t.Fatalf("config = %+v", config)
	}
	if config.PitModeFrequency == nil || *config.PitModeFrequency != 5662 {
		t.Fatalf("config = %+v", config)
	}
	if config.Table == nil || !config.Table.Available || config.Table.Bands != 5 || config.Table.Channels != 8 || config.Table.PowerLevels != 3 {
		t.Fatalf("config = %+v", config)
	}
}

func TestDecodeVTXConfigLegacyPayload(t *testing.T) {
	payload := []byte{255, 0, 0, 0, 0, 0, 0, 0, 0}
	config, err := DecodeVTXConfig(payload)
	if err != nil {
		t.Fatalf("DecodeVTXConfig() error = %v", err)
	}
	if config.TypeName != "UNKNOWN" || config.PitModeFrequency != nil || config.Table != nil {
		t.Fatalf("config = %+v", config)
	}
}

func TestEncodeVTXConfig(t *testing.T) {
	payload := EncodeVTXConfig(VTXConfigSetConfig{
		Band:             5,
		Channel:          8,
		Power:            2,
		PitMode:          true,
		FrequencyMHz:     5861,
		LowPowerDisarm:   2,
		PitModeFrequency: 5662,
	})
	want := []byte{0xe5, 0x16, 2, 1, 2, 0x1e, 0x16, 5, 8, 0xe5, 0x16}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestDecodeVTXTableRows(t *testing.T) {
	bandPayload := []byte{1, 8, 'R', 'A', 'C', 'E', 'B', 'A', 'N', 'D', 'R', 1, 2, 0x1a, 0x16, 0x3f, 0x16}
	band, err := DecodeVTXTableBand(bandPayload)
	if err != nil {
		t.Fatalf("DecodeVTXTableBand() error = %v", err)
	}
	if band.Band != 1 || band.Name != "RACEBAND" || band.Letter != "R" || !band.Factory || len(band.FrequenciesMHz) != 2 || band.FrequenciesMHz[0] != 5658 || band.FrequenciesMHz[1] != 5695 {
		t.Fatalf("band = %+v", band)
	}

	power, err := DecodeVTXTablePowerLevel([]byte{2, 0xc8, 0x00, 3, '2', '0', '0'})
	if err != nil {
		t.Fatalf("DecodeVTXTablePowerLevel() error = %v", err)
	}
	if power.Level != 2 || power.Value != 200 || power.Label != "200" {
		t.Fatalf("power = %+v", power)
	}
}

func TestDecodeVTXTableRowsRejectTrailingBytes(t *testing.T) {
	if _, err := DecodeVTXTableBand([]byte{1, 1, 'A', 'A', 1, 0, 9}); err == nil {
		t.Fatal("DecodeVTXTableBand() error = nil")
	}
	if _, err := DecodeVTXTablePowerLevel([]byte{1, 25, 0, 2, '2', '5', 9}); err == nil {
		t.Fatal("DecodeVTXTablePowerLevel() error = nil")
	}
}

func TestDecodeVTXDeviceStatus(t *testing.T) {
	payload := []byte{3, 1, 1, 5, 8, 1, 2, 1}
	payload = appendU16Payload(payload, 5861)
	payload = append(payload, 1)
	payload = appendU32Payload(payload, 0x03)
	payload = append(payload, 2)
	payload = appendU16Payload(payload, 1)
	payload = appendU16Payload(payload, 25)
	payload = appendU16Payload(payload, 2)
	payload = appendU16Payload(payload, 200)
	payload = append(payload, 2, 0xaa, 0x55)
	status, err := DecodeVTXDeviceStatus(payload)
	if err != nil {
		t.Fatalf("DecodeVTXDeviceStatus() error = %v", err)
	}
	if !status.Supported || !status.DevicePresent || status.TypeName != "SMARTAUDIO" || !status.Ready {
		t.Fatalf("status = %+v", status)
	}
	if !status.BandChannelAvailable || status.Band != 5 || status.Channel != 8 {
		t.Fatalf("status = %+v", status)
	}
	if !status.PowerIndexAvailable || status.PowerIndex != 2 || !status.FrequencyAvailable || status.FrequencyMHz != 5861 {
		t.Fatalf("status = %+v", status)
	}
	if !status.StatusAvailable || status.StatusRaw != 3 || !status.PitMode || !status.Locked {
		t.Fatalf("status = %+v", status)
	}
	if len(status.PowerLevels) != 2 || status.PowerLevels[1].Power != 200 {
		t.Fatalf("power levels = %+v", status.PowerLevels)
	}
	if status.CustomStatusByteCount != 2 || len(status.CustomStatusBytes) != 2 || status.CustomStatusBytes[0] != 0xaa {
		t.Fatalf("custom status = %+v", status.CustomStatusBytes)
	}
}

func TestDecodeVTXDeviceStatusNoDevice(t *testing.T) {
	status, err := DecodeVTXDeviceStatus(nil)
	if err != nil {
		t.Fatalf("DecodeVTXDeviceStatus() error = %v", err)
	}
	if !status.Supported || status.DevicePresent {
		t.Fatalf("status = %+v", status)
	}
}

func TestEncodeVTXTableBand(t *testing.T) {
	payload := EncodeVTXTableBand(VTXTableBandSetConfig{
		Band:           1,
		Name:           "RACEBAND",
		Letter:         "R",
		Factory:        true,
		FrequenciesMHz: []uint16{5658, 5695},
	})
	want := []byte{1, 8, 'R', 'A', 'C', 'E', 'B', 'A', 'N', 'D', 'R', 1, 2, 0x1a, 0x16, 0x3f, 0x16}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestEncodeVTXTablePower(t *testing.T) {
	payload := EncodeVTXTablePower(VTXTablePowerSetConfig{Level: 2, Value: 200, Label: "200"})
	want := []byte{2, 0xc8, 0x00, 3, '2', '0', '0'}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestValidateVTXTable(t *testing.T) {
	err := ValidateVTXTable(VTXTableSetConfig{
		Bands: []VTXTableBandSetConfig{{
			Band:           1,
			Name:           "RACEBAND",
			Letter:         "R",
			Factory:        true,
			FrequenciesMHz: []uint16{5658},
		}},
		Powers: []VTXTablePowerSetConfig{{Level: 1, Value: 25, Label: "25"}},
	})
	if err != nil {
		t.Fatalf("ValidateVTXTable() error = %v", err)
	}
	if err := ValidateVTXTable(VTXTableSetConfig{}); err == nil {
		t.Fatal("ValidateVTXTable() error = nil")
	}
}
