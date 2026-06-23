package commands

import "testing"

func TestDecodeAltitude(t *testing.T) {
	payload := appendU32Test(nil, 12345)
	payload = appendS16Test(payload, -67)
	altitude, err := DecodeAltitude(payload)
	if err != nil {
		t.Fatalf("DecodeAltitude() error = %v", err)
	}
	if altitude.AltitudeCm != 12345 || altitude.AltitudeM != 123.45 || altitude.VarioCmS != -67 {
		t.Fatalf("altitude = %+v", altitude)
	}
}

func TestDecodeAltitudeRejectsShortPayload(t *testing.T) {
	if _, err := DecodeAltitude([]byte{1, 2}); err == nil {
		t.Fatal("DecodeAltitude() error = nil, want short payload error")
	}
}

func TestDecodeRangefinderAltitude(t *testing.T) {
	rangefinder, err := DecodeRangefinderAltitude(appendU32Test(nil, 4321))
	if err != nil {
		t.Fatalf("DecodeRangefinderAltitude() error = %v", err)
	}
	if rangefinder.AltitudeCm != 4321 || rangefinder.AltitudeM != 43.21 {
		t.Fatalf("rangefinder = %+v", rangefinder)
	}
}

func TestDecodeAnalog(t *testing.T) {
	payload := []byte{160}
	payload = appendU16Test(payload, 321)
	payload = appendU16Test(payload, 900)
	payload = appendS16Test(payload, -123)
	payload = appendU16Test(payload, 1599)
	analog, err := DecodeAnalog(payload)
	if err != nil {
		t.Fatalf("DecodeAnalog() error = %v", err)
	}
	if analog.VoltageLegacyV != 16 || analog.DrawnMAh != 321 || analog.RSSI != 900 || analog.AmperageA != -1.23 || analog.VoltageV != 15.99 {
		t.Fatalf("analog = %+v", analog)
	}
}

func TestDecodeAnalogRejectsShortPayload(t *testing.T) {
	if _, err := DecodeAnalog([]byte{1, 2}); err == nil {
		t.Fatal("DecodeAnalog() error = nil, want short payload error")
	}
}
