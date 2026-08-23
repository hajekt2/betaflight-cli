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

func TestDecodeOpticalFlow(t *testing.T) {
	payload := []byte{200}
	payload = appendS32Test(payload, -400)
	payload = appendS32Test(payload, 300)
	flow, err := DecodeOpticalFlow(payload)
	if err != nil {
		t.Fatalf("DecodeOpticalFlow() error = %v", err)
	}
	if flow.Quality != 200 {
		t.Fatalf("quality = %d, want 200", flow.Quality)
	}
	if want := 200.0 * 100 / 255; flow.QualityPct != want {
		t.Fatalf("quality percent = %v, want %v", flow.QualityPct, want)
	}
	if flow.MotionX != -400 || flow.MotionY != 300 {
		t.Fatalf("motion = (%d, %d), want (-400, 300)", flow.MotionX, flow.MotionY)
	}
	if flow.FlowRateXRaw != -2 || flow.FlowRateYRaw != 1.5 {
		t.Fatalf("flow rates = (%v, %v), want (-2, 1.5)", flow.FlowRateXRaw, flow.FlowRateYRaw)
	}
}

func TestDecodeOpticalFlowRejectsShortPayload(t *testing.T) {
	if _, err := DecodeOpticalFlow([]byte{1, 2, 3}); err == nil {
		t.Fatal("DecodeOpticalFlow() error = nil, want short payload error")
	}
}

func TestDecodeRangefinderMt(t *testing.T) {
	payload := []byte{180}
	payload = appendS32Test(payload, 123456)
	reading, err := DecodeRangefinderMt(payload)
	if err != nil {
		t.Fatalf("DecodeRangefinderMt() error = %v", err)
	}
	if reading.Quality != 180 || reading.DistanceMm != 123456 {
		t.Fatalf("rangefinder mt = %+v", reading)
	}

	outOfRange := []byte{0}
	outOfRange = appendS32Test(outOfRange, -1)
	reading, err = DecodeRangefinderMt(outOfRange)
	if err != nil {
		t.Fatalf("DecodeRangefinderMt() out-of-range error = %v", err)
	}
	if reading.DistanceMm != -1 {
		t.Fatalf("out-of-range distance = %d, want -1", reading.DistanceMm)
	}
}

func TestDecodeRangefinderMtRejectsShortPayload(t *testing.T) {
	if _, err := DecodeRangefinderMt([]byte{1, 2}); err == nil {
		t.Fatal("DecodeRangefinderMt() error = nil, want short payload error")
	}
}
