package commands

import (
	"bytes"
	"encoding/json"
	"testing"
)

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

func TestSetOSDCanvasPayload(t *testing.T) {
	payload := EncodeOSDCanvas(OSDCanvasSetConfig{Columns: 53, Rows: 20})
	if string(payload) != string([]byte{53, 20}) {
		t.Fatalf("payload = %v", payload)
	}
}

func TestEncodeOSDPosition(t *testing.T) {
	payload := EncodeOSDPosition(OSDPositionSetConfig{Index: 7, Raw: 0x084a, Screen: 1})
	want := []byte{7, 0x4a, 0x08, 1}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestEncodeOSDStat(t *testing.T) {
	payload := EncodeOSDStat(OSDStatSetConfig{Index: 3, Enabled: true})
	want := []byte{3, 1, 0, 0}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestEncodeOSDTimer(t *testing.T) {
	payload := EncodeOSDTimer(OSDTimerSetConfig{Index: 1, Value: 0x0456})
	want := []byte{254, 1, 0x56, 0x04}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestEncodeOSDVideoSystem(t *testing.T) {
	config := &OSDConfig{
		Units:             1,
		SelectedProfile:   2,
		StickOverlayMode:  1,
		CameraFrameWidth:  24,
		CameraFrameHeight: 18,
		EnabledWarnings:   0x12345678,
		Alarms: OSDAlarms{
			RSSI:        20,
			CapacityMAh: 1500,
			AltitudeM:   120,
			LinkQuality: 70,
			RSSIDBm:     -95,
		},
	}
	payload := EncodeOSDVideoSystem(config, 3)
	want := []byte{
		255, 3, 1, 20,
		0xdc, 0x05,
		0, 0,
		120, 0,
		0x78, 0x56,
		0x78, 0x56, 0x34, 0x12,
		2, 1, 24, 18,
		70, 0,
		0xa1, 0xff,
	}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

func TestValidateOSDGeneralSetConfig(t *testing.T) {
	videoSystem := uint8(4)
	if err := ValidateOSDGeneralSetConfig(OSDGeneralSetConfig{VideoSystem: &videoSystem}); err == nil {
		t.Fatal("ValidateOSDGeneralSetConfig() error = nil")
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

func TestEncodeOSDCharRoundTrip(t *testing.T) {
	bitmap := make([]byte, OSDCharVisibleBytes)
	for i := range bitmap {
		bitmap[i] = byte(i)
	}
	payload, err := EncodeOSDChar(7, bitmap)
	if err != nil {
		t.Fatalf("EncodeOSDChar() error = %v", err)
	}
	if len(payload) != OSDCharVisibleBytes+1 || payload[0] != 7 {
		t.Fatalf("payload = %v", payload[:min(len(payload), 4)])
	}
	character, err := DecodeOSDChar(payload)
	if err != nil {
		t.Fatalf("DecodeOSDChar() error = %v", err)
	}
	if character.Index != 7 || string(character.Bitmap) != string(bitmap) {
		t.Fatalf("character index = %d", character.Index)
	}
}

func TestDecodeOSDCharAcceptsFullBitmap(t *testing.T) {
	payload := []byte{9}
	payload = append(payload, make([]byte, OSDCharFullBytes)...)
	payload[len(payload)-1] = 0xaa
	character, err := DecodeOSDChar(payload)
	if err != nil {
		t.Fatalf("DecodeOSDChar() error = %v", err)
	}
	if character.Index != 9 || len(character.Bitmap) != OSDCharVisibleBytes {
		t.Fatalf("character = %+v", character)
	}
}

func TestDecodeOSDCharRejectsShortBitmap(t *testing.T) {
	payload := append([]byte{3}, make([]byte, OSDCharVisibleBytes-1)...)
	if _, err := DecodeOSDChar(payload); err == nil {
		t.Fatal("DecodeOSDChar() error = nil for short bitmap")
	}
	if _, err := DecodeOSDChar(nil); err == nil {
		t.Fatal("DecodeOSDChar() error = nil for empty payload")
	}
}

func TestSetOSDCharPayloadLength(t *testing.T) {
	bitmap := make([]byte, OSDCharVisibleBytes+1)
	if _, err := EncodeOSDChar(0, bitmap); err == nil {
		t.Fatal("EncodeOSDChar() error = nil for oversized bitmap")
	}
}

func TestEncodeOSDCharPixels(t *testing.T) {
	rows := make([][]uint8, osdCharRows)
	for y := range rows {
		row := make([]uint8, osdCharColumns)
		for x := range row {
			row[x] = uint8((x + y) % 4)
		}
		rows[y] = row
	}
	bitmap, err := EncodeOSDCharPixels(rows)
	if err != nil {
		t.Fatalf("EncodeOSDCharPixels() error = %v", err)
	}
	if len(bitmap) != OSDCharVisibleBytes {
		t.Fatalf("bitmap length = %d", len(bitmap))
	}
	// First four pixels of row 0 pack into one byte, most significant pair first.
	want := rows[0][0]<<6 | rows[0][1]<<4 | rows[0][2]<<2 | rows[0][3]
	if bitmap[0] != want {
		t.Fatalf("bitmap[0] = %#02x, want %#02x", bitmap[0], want)
	}
	if _, err := EncodeOSDCharPixels(rows[:17]); err == nil {
		t.Fatal("EncodeOSDCharPixels() error = nil for short row count")
	}
	rows[5][7] = 4
	if _, err := EncodeOSDCharPixels(rows); err == nil {
		t.Fatal("EncodeOSDCharPixels() error = nil for pixel value above 3")
	}
}

func TestOSDCharBitmapJSONRoundTrip(t *testing.T) {
	bitmap := make(OSDBitmap, OSDCharVisibleBytes)
	for i := range bitmap {
		bitmap[i] = byte(i)
	}
	data, err := json.Marshal(&OSDChar{Index: 7, Bitmap: bitmap})
	if err != nil {
		t.Fatalf("json.Marshal() error = %v", err)
	}
	if !bytes.HasPrefix(data, []byte(`{"index":7,"bitmap":[0,1,2,3`)) || !bytes.HasSuffix(data, []byte(`52,53]}`)) {
		t.Fatalf("bitmap must marshal as a number array, got %s", data[:min(len(data), 40)])
	}
	var decoded OSDChar
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	if decoded.Index != 7 || !bytes.Equal(decoded.Bitmap, bitmap) {
		t.Fatalf("round trip mismatch, character = %+v", decoded)
	}
	var config OSDCharSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		t.Fatalf("json.Unmarshal(OSDCharSetConfig) error = %v", err)
	}
	if config.Index != 7 || !bytes.Equal(config.Bitmap, bitmap) {
		t.Fatalf("config round trip mismatch, config = %+v", config)
	}
	if err := json.Unmarshal([]byte(`{"index":0,"bitmap":[256]}`), &decoded); err == nil {
		t.Fatal("json.Unmarshal() error = nil for out-of-range byte")
	}
}

func TestDecodeOSDVideoConfig(t *testing.T) {
	config, err := DecodeOSDVideoConfig([]byte{3, 1})
	if err != nil {
		t.Fatalf("DecodeOSDVideoConfig() error = %v", err)
	}
	if config.VideoSystem != 3 || config.VideoSystemName != "HD" {
		t.Fatalf("video system = %+v", config)
	}
	if config.Units != 1 || config.UnitsName != "METRIC" {
		t.Fatalf("units = %+v", config)
	}
	if _, err := DecodeOSDVideoConfig([]byte{3}); err == nil {
		t.Fatal("DecodeOSDVideoConfig() error = nil for short payload")
	}
	if _, err := DecodeOSDVideoConfig([]byte{3, 1, 0}); err == nil {
		t.Fatal("DecodeOSDVideoConfig() error = nil for trailing bytes")
	}
}

func TestValidateAndEncodeOSDVideoConfig(t *testing.T) {
	if err := ValidateOSDVideoConfig(OSDVideoConfigSetConfig{VideoSystem: 4, Units: 1}); err == nil {
		t.Fatal("ValidateOSDVideoConfig() error = nil for video_system 4")
	}
	if err := ValidateOSDVideoConfig(OSDVideoConfigSetConfig{VideoSystem: 2, Units: 3}); err == nil {
		t.Fatal("ValidateOSDVideoConfig() error = nil for units 3")
	}
	payload := EncodeOSDVideoConfig(OSDVideoConfigSetConfig{VideoSystem: 2, Units: 1})
	if string(payload) != string([]byte{2, 1}) {
		t.Fatalf("payload = %v", payload)
	}
}
