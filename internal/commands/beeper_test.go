package commands

import "testing"

func TestDecodeBeeperConfig(t *testing.T) {
	payload := appendU32Test(nil, 0x00000012)
	payload = append(payload, 3)
	payload = appendU32Test(payload, 0x00000202)
	config, err := DecodeBeeperConfig(payload)
	if err != nil {
		t.Fatalf("DecodeBeeperConfig() error = %v", err)
	}
	if config.DisabledMask != 0x12 || config.DShotBeaconTone != 3 || config.DShotBeaconDisabledMask != 0x202 {
		t.Fatalf("config = %+v", config)
	}
	if len(config.Disabled) != 2 || config.Disabled[0] != "RX_LOST" || config.Disabled[1] != "ARMING" {
		t.Fatalf("config = %+v", config)
	}
	if len(config.DShotBeaconDisabled) != 2 || config.DShotBeaconDisabled[1] != "RX_SET" {
		t.Fatalf("config = %+v", config)
	}
}

func TestEncodeBeeperConfig(t *testing.T) {
	payload := EncodeBeeperConfig(BeeperConfig{
		DisabledMask:            0x00000012,
		DShotBeaconTone:         3,
		DShotBeaconDisabledMask: 0x00000202,
	})
	want := []byte{0x12, 0, 0, 0, 3, 0x02, 0x02, 0, 0}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}
