package commands

import "testing"

func TestDecodeLEDStripConfig(t *testing.T) {
	payload := appendTestU32(nil, testLEDConfigRaw(1, 2, 0, 1, 3, 0x03))
	payload = appendTestU32(payload, testLEDConfigRaw(3, 4, 1, 1<<3, 5, 0x04))
	payload = append(payload, 1, 0)

	config, err := DecodeLEDStripConfig(payload)
	if err != nil {
		t.Fatalf("DecodeLEDStripConfig() error = %v", err)
	}
	if !config.AdvancedSupported || config.Profile != 0 || len(config.LEDs) != 2 {
		t.Fatalf("config = %+v", config)
	}
	first := config.LEDs[0]
	if first.X != 1 || first.Y != 2 || first.Function != "c" || first.Color != 3 || first.CLISyntax != "1,2:ne:Ct:3" {
		t.Fatalf("first = %+v", first)
	}
	if len(first.Directions) != 2 || first.Directions[0] != "n" || first.Directions[1] != "e" {
		t.Fatalf("first directions = %+v", first.Directions)
	}
	second := config.LEDs[1]
	if second.X != 3 || second.Y != 4 || second.Function != "f" || second.Overlays[0] != "b" || second.Directions[0] != "s" {
		t.Fatalf("second = %+v", second)
	}
}

func TestDecodeLEDStripConfigRejectsBadPayload(t *testing.T) {
	if _, err := DecodeLEDStripConfig([]byte{1}); err == nil {
		t.Fatal("DecodeLEDStripConfig() error = nil, want short payload error")
	}
	if _, err := DecodeLEDStripConfig([]byte{1, 2, 3}); err == nil {
		t.Fatal("DecodeLEDStripConfig() error = nil, want misaligned payload error")
	}
}

func TestDecodeLEDColorsModeColorsAndValues(t *testing.T) {
	colors, err := DecodeLEDColors([]byte{0, 0, 0, 255, 120, 0, 255, 255})
	if err != nil {
		t.Fatalf("DecodeLEDColors() error = %v", err)
	}
	if len(colors) != 2 || colors[1].Hue != 120 || colors[1].Sat != 255 || colors[1].Val != 255 {
		t.Fatalf("colors = %+v", colors)
	}

	modeColors, err := DecodeLEDModeColors([]byte{0, 0, 3, 1, 2, 5})
	if err != nil {
		t.Fatalf("DecodeLEDModeColors() error = %v", err)
	}
	if len(modeColors) != 2 || modeColors[1].Mode != 1 || modeColors[1].Direction != 2 || modeColors[1].Color != 5 {
		t.Fatalf("modeColors = %+v", modeColors)
	}

	values, err := DecodeLEDConfigValues([]byte{50, 20, 0, 120, 0})
	if err != nil {
		t.Fatalf("DecodeLEDConfigValues() error = %v", err)
	}
	if values.Brightness != 50 || values.RainbowDelta != 20 || values.RainbowFreq != 120 {
		t.Fatalf("values = %+v", values)
	}
}

func TestEncodeLEDConfigValues(t *testing.T) {
	got := EncodeLEDConfigValues(LEDConfigValues{Brightness: 50, RainbowDelta: 20, RainbowFreq: 120})
	want := []byte{50, 20, 0, 120, 0}
	if string(got) != string(want) {
		t.Fatalf("EncodeLEDConfigValues() = %v, want %v", got, want)
	}
}

func TestEncodeLEDColors(t *testing.T) {
	got := EncodeLEDColors([]LEDColor{
		{Index: 0, Hue: 0, Sat: 0, Val: 255},
		{Index: 1, Hue: 120, Sat: 255, Val: 255},
	})
	want := []byte{0, 0, 0, 255, 120, 0, 255, 255}
	if string(got) != string(want) {
		t.Fatalf("EncodeLEDColors() = %v, want %v", got, want)
	}
}

func TestValidateLEDColors(t *testing.T) {
	if err := ValidateLEDColors([]LEDColor{{Hue: 360, Sat: 0, Val: 255}}); err == nil {
		t.Fatal("ValidateLEDColors() error = nil, want hue error")
	}
	if err := ValidateLEDColors(nil); err == nil {
		t.Fatal("ValidateLEDColors() error = nil, want empty table error")
	}
}

func testLEDConfigRaw(x, y, function uint8, overlays uint16, color, directions uint8) uint32 {
	return uint32(y&0x0f) |
		uint32(x&0x0f)<<4 |
		uint32(function&0x0f)<<8 |
		uint32(overlays&0x03ff)<<12 |
		uint32(color&0x0f)<<22 |
		uint32(directions&0x3f)<<26
}

func appendTestU32(dst []byte, v uint32) []byte {
	return append(dst, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}
