package commands

import "testing"

func TestDecodeOSDConfig(t *testing.T) {
	payload := []byte{0x31, 3, 1, 20}
	payload = appendU16Test(payload, 1500)
	payload = append(payload, 0, 3)
	payload = appendU16Test(payload, 120)
	payload = appendU16Test(payload, 0x0800|10|(2<<5))
	payload = appendU16Test(payload, 5|(3<<5))
	payload = appendU16Test(payload, 0x0800|20|(4<<5))
	payload = append(payload, 2, 1, 0, 2)
	payload = appendU16Test(payload, 0x0123)
	payload = appendU16Test(payload, 0x0456)
	payload = appendU16Test(payload, 0x0005)
	payload = append(payload, 4)
	payload = appendU32Test(payload, 0x00000005)
	payload = append(payload, 3, 2, 1, 24, 18)
	payload = appendU16Test(payload, 70)
	payload = appendS16Test(payload, -95)
	config, err := DecodeOSDConfig(payload)
	if err != nil {
		t.Fatalf("DecodeOSDConfig() error = %v", err)
	}
	if !config.Flags.FeatureEnabled || !config.Flags.HardwareMax7456 || !config.Flags.DeviceDetected || config.Flags.MSPDevice {
		t.Fatalf("flags = %+v", config.Flags)
	}
	if config.VideoSystemName != "HD" || config.UnitsName != "METRIC" {
		t.Fatalf("config = %+v", config)
	}
	if len(config.ItemPositions) != 3 || config.ItemPositions[0].X != 10 || config.ItemPositions[0].Y != 2 {
		t.Fatalf("item positions = %+v", config.ItemPositions)
	}
	if !config.ItemPositions[0].Visible || config.ItemPositions[1].Visible {
		t.Fatalf("hidden item decoded as visible: %+v", config.ItemPositions[1])
	}
	if len(config.StatisticsEnabled) != 2 || !config.StatisticsEnabled[0] || config.StatisticsEnabled[1] {
		t.Fatalf("statistics = %+v", config.StatisticsEnabled)
	}
	if config.Alarms.LinkQuality != 70 || config.Alarms.RSSIDBm != -95 {
		t.Fatalf("alarms = %+v", config.Alarms)
	}
}

func TestDecodeOSDCanvas(t *testing.T) {
	canvas, err := DecodeOSDCanvas([]byte{53, 20})
	if err != nil {
		t.Fatalf("DecodeOSDCanvas() error = %v", err)
	}
	if canvas.Columns != 53 || canvas.Rows != 20 {
		t.Fatalf("canvas = %+v", canvas)
	}
}

func TestDecodeOSDWarnings(t *testing.T) {
	warnings, err := DecodeOSDWarnings([]byte{2, 11, 'L', 'O', 'W', ' ', 'B', 'A', 'T', 'T', 'E', 'R', 'Y'})
	if err != nil {
		t.Fatalf("DecodeOSDWarnings() error = %v", err)
	}
	if warnings.DisplayAttributes != 2 || warnings.Text != "LOW BATTERY" {
		t.Fatalf("warnings = %+v", warnings)
	}
}
