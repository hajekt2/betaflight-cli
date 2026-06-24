package commands

import "testing"

func TestDecodeAttitude(t *testing.T) {
	payload := []byte{0xf6, 0xff, 0x14, 0x00, 0xb4, 0x00}
	attitude, err := DecodeAttitude(payload)
	if err != nil {
		t.Fatalf("DecodeAttitude() error = %v", err)
	}
	if attitude.RollDegrees != -1 || attitude.PitchDegrees != 2 || attitude.YawDegrees != 180 {
		t.Fatalf("attitude = %+v", attitude)
	}
}

func TestDecodeAttitudeQuaternion(t *testing.T) {
	payload := []byte{0xff, 0x7f, 0x00, 0x00, 0x00, 0xc0, 0x00, 0x40}
	quaternion, err := DecodeAttitudeQuaternion(payload)
	if err != nil {
		t.Fatalf("DecodeAttitudeQuaternion() error = %v", err)
	}
	if quaternion.W != 1 || quaternion.X != 0 {
		t.Fatalf("quaternion = %+v", quaternion)
	}
	if quaternion.Y >= -0.5 || quaternion.Y <= -0.51 || quaternion.Z <= 0.5 || quaternion.Z >= 0.51 {
		t.Fatalf("quaternion = %+v", quaternion)
	}
}

func TestDecodeAttitudeQuaternionRejectsTrailingBytes(t *testing.T) {
	if _, err := DecodeAttitudeQuaternion([]byte{0xff, 0x7f, 0, 0, 0, 0, 0, 0, 1}); err == nil {
		t.Fatal("DecodeAttitudeQuaternion() error = nil")
	}
}

func TestDecodeRC(t *testing.T) {
	payload := []byte{0x00, 0x01, 0x00, 0x02}
	channels, err := DecodeRC(payload)
	if err != nil {
		t.Fatalf("DecodeRC() error = %v", err)
	}
	if len(channels) != 2 || channels[0] != 256 || channels[1] != 512 {
		t.Fatalf("channels = %+v", channels)
	}
}

func TestDecodeStatusWrapper(t *testing.T) {
	payload := appendU16Test(nil, 250)
	payload = appendU16Test(payload, 0)
	payload = appendU16Test(payload, 33)
	payload = appendU32Test(payload, 2)
	payload = append(payload, 0)
	out, err := DecodeStatus(payload)
	if err != nil {
		t.Fatalf("DecodeStatus() error = %v", err)
	}
	if out.Source != "MSP_STATUS" {
		t.Fatalf("source = %q", out.Source)
	}
	if out.CycleTimeUS != 250 || out.Profile != 0 {
		t.Fatalf("status = %+v", out)
	}
}
