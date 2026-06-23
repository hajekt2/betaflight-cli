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
