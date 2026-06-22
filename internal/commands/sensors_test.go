package commands

import "testing"

func TestDecodeSensorHardware(t *testing.T) {
	hardware, err := DecodeSensorHardware([]byte{1, 0xff}, []string{"gyro", "barometer"})
	if err != nil {
		t.Fatalf("DecodeSensorHardware() error = %v", err)
	}
	if len(hardware) != 2 || hardware[0].Name != "gyro" || !hardware[0].Available || hardware[1].Available {
		t.Fatalf("hardware = %+v", hardware)
	}
}

func TestDecodeRawIMU(t *testing.T) {
	payload := appendU16Test(nil, 2048)
	payload = appendS16Test(payload, -1024)
	payload = appendU16Test(payload, 512)
	payload = appendU16Test(payload, 100)
	payload = appendS16Test(payload, -50)
	payload = appendU16Test(payload, 25)
	payload = appendU16Test(payload, 300)
	payload = appendS16Test(payload, -200)
	payload = appendU16Test(payload, 100)
	imu, err := DecodeRawIMU(payload)
	if err != nil {
		t.Fatalf("DecodeRawIMU() error = %v", err)
	}
	if imu.AccelerometerRaw[0] != 2048 || imu.AccelerometerRaw[1] != -1024 || imu.GyroscopeRaw[1] != -50 {
		t.Fatalf("imu = %+v", imu)
	}
	if imu.AccelerometerG[0] != 1 || imu.AccelerometerG[1] != -0.5 {
		t.Fatalf("imu = %+v", imu)
	}
}

func TestDecodeSensorAlignmentAndCompass(t *testing.T) {
	payload := []byte{1, 1, 2, 3, 0x03}
	payload = appendS16Test(payload, -10)
	payload = appendU16Test(payload, 20)
	payload = appendU16Test(payload, 900)
	alignment, err := DecodeSensorAlignment(payload)
	if err != nil {
		t.Fatalf("DecodeSensorAlignment() error = %v", err)
	}
	if alignment.GyroAlignment != 1 || alignment.MagnetometerAlign != 2 || alignment.GyroEnabledMask == nil || *alignment.GyroEnabledMask != 3 {
		t.Fatalf("alignment = %+v", alignment)
	}
	if alignment.MagCustomAlignment == nil || alignment.MagCustomAlignment.Roll != -10 || alignment.MagCustomAlignment.Yaw != 900 {
		t.Fatalf("alignment = %+v", alignment)
	}
	compass, err := DecodeCompassConfig([]byte{123, 0})
	if err != nil {
		t.Fatalf("DecodeCompassConfig() error = %v", err)
	}
	if compass.DeclinationDeciDegrees != 123 || compass.DeclinationDegrees != 12.3 {
		t.Fatalf("compass = %+v", compass)
	}
}

func appendS16Test(dst []byte, v int16) []byte {
	return appendU16Test(dst, uint16(v))
}

func TestActiveSensorNames(t *testing.T) {
	names := activeSensorNames(0b0100001)
	if len(names) != 2 || names[0] != "accelerometer" || names[1] != "gyro" {
		t.Fatalf("names = %+v", names)
	}
}
