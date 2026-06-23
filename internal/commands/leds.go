package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const (
	ledPositionMask       = 0x0f
	ledFunctionMask       = 0x0f
	ledOverlayMask        = 0x03ff
	ledColorMask          = 0x0f
	ledDirectionMask      = 0x3f
	ledXOffset            = 4
	ledFunctionOffset     = 8
	ledOverlayOffset      = 12
	ledColorOffset        = 22
	ledDirectionOffset    = 26
	ledStripTrailerLength = 2
)

var (
	ledDirectionLetters = []string{"n", "e", "s", "w", "u", "d"}
	ledFunctionLetters  = []string{"c", "f", "a", "l", "s", "g", "r", "p", "e", "u"}
	ledOverlayLetters   = []string{"t", "y", "o", "b", "v", "i", "w"}
)

type LEDStatus struct {
	Strip      *LEDStripConfig   `json:"strip,omitempty"`
	Colors     []LEDColor        `json:"colors,omitempty"`
	ModeColors []LEDModeColor    `json:"mode_colors,omitempty"`
	Values     *LEDConfigValues  `json:"values,omitempty"`
	Sources    map[string]string `json:"sources,omitempty"`
}

type LEDStripConfig struct {
	LEDs              []LEDConfig `json:"leds"`
	AdvancedSupported bool        `json:"advanced_supported"`
	Profile           uint8       `json:"profile"`
}

type LEDConfig struct {
	Index          int      `json:"index"`
	Raw            uint32   `json:"raw"`
	Configured     bool     `json:"configured"`
	X              uint8    `json:"x"`
	Y              uint8    `json:"y"`
	FunctionID     uint8    `json:"function_id"`
	Function       string   `json:"function,omitempty"`
	OverlaysMask   uint16   `json:"overlays_mask"`
	Overlays       []string `json:"overlays,omitempty"`
	Color          uint8    `json:"color"`
	DirectionsMask uint8    `json:"directions_mask"`
	Directions     []string `json:"directions,omitempty"`
	CLISyntax      string   `json:"cli_syntax"`
}

type LEDColor struct {
	Index int    `json:"index"`
	Hue   uint16 `json:"hue"`
	Sat   uint8  `json:"sat"`
	Val   uint8  `json:"val"`
}

type LEDModeColor struct {
	Mode      uint8 `json:"mode"`
	Direction uint8 `json:"direction"`
	Color     uint8 `json:"color"`
}

type LEDModeColorSetResult struct {
	ModeColor    LEDModeColor `json:"mode_color"`
	MSPCode      uint16       `json:"msp_code"`
	MSPName      string       `json:"msp_name"`
	Acknowledged bool         `json:"acknowledged"`
	SaveRequired bool         `json:"save_required"`
}

type LEDConfigValues struct {
	Brightness   uint8  `json:"brightness"`
	RainbowDelta uint16 `json:"rainbow_delta"`
	RainbowFreq  uint16 `json:"rainbow_freq"`
}

type LEDConfigValuesSetResult struct {
	Values       LEDConfigValues `json:"values"`
	MSPCode      uint16          `json:"msp_code"`
	MSPName      string          `json:"msp_name"`
	Acknowledged bool            `json:"acknowledged"`
	SaveRequired bool            `json:"save_required"`
}

type LEDColorsSetResult struct {
	Colors       []LEDColor `json:"colors"`
	MSPCode      uint16     `json:"msp_code"`
	MSPName      string     `json:"msp_name"`
	Acknowledged bool       `json:"acknowledged"`
	SaveRequired bool       `json:"save_required"`
}

func ReadLEDStatus(ctx context.Context, client *connection.Client) (*LEDStatus, []string, error) {
	status := &LEDStatus{Sources: map[string]string{}}
	warnings := []string{}

	frame, err := client.Request(ctx, msp.MSPLedStripConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("led strip config unavailable: %w", err)
	}
	strip, err := DecodeLEDStripConfig(frame.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("led strip config decode failed: %w", err)
	}
	status.Strip = strip
	status.Sources["strip"] = "MSP_LED_STRIP_CONFIG"

	if colors, err := readLEDColors(ctx, client); err == nil {
		status.Colors = colors
		status.Sources["colors"] = "MSP_LED_COLORS"
	} else {
		warnings = append(warnings, err.Error())
	}
	if modeColors, err := readLEDModeColors(ctx, client); err == nil {
		status.ModeColors = modeColors
		status.Sources["mode_colors"] = "MSP_LED_STRIP_MODECOLOR"
	} else {
		warnings = append(warnings, err.Error())
	}
	if values, err := readLEDConfigValues(ctx, client); err == nil {
		status.Values = values
		status.Sources["values"] = "MSP2_GET_LED_STRIP_CONFIG_VALUES"
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func DecodeLEDStripConfig(payload []byte) (*LEDStripConfig, error) {
	if len(payload) < ledStripTrailerLength {
		return nil, fmt.Errorf("payload length %d is shorter than LED strip trailer", len(payload))
	}
	ledBytes := len(payload) - ledStripTrailerLength
	if ledBytes%4 != 0 {
		return nil, fmt.Errorf("payload has %d LED bytes, want a multiple of 4", ledBytes)
	}
	r := msp.NewPayloadReader(payload[:ledBytes])
	leds := make([]LEDConfig, 0, ledBytes/4)
	for i := 0; r.Remaining() > 0; i++ {
		raw, err := r.U32()
		if err != nil {
			return nil, err
		}
		leds = append(leds, decodeLEDConfig(i, raw))
	}
	return &LEDStripConfig{
		LEDs:              leds,
		AdvancedSupported: payload[ledBytes] != 0,
		Profile:           payload[ledBytes+1],
	}, nil
}

func DecodeLEDColors(payload []byte) ([]LEDColor, error) {
	if len(payload)%4 != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of LED color size 4", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	colors := make([]LEDColor, 0, len(payload)/4)
	for i := 0; r.Remaining() > 0; i++ {
		hue, err := r.U16()
		if err != nil {
			return nil, err
		}
		sat, err := r.U8()
		if err != nil {
			return nil, err
		}
		val, err := r.U8()
		if err != nil {
			return nil, err
		}
		colors = append(colors, LEDColor{Index: i, Hue: hue, Sat: sat, Val: val})
	}
	return colors, nil
}

func DecodeLEDModeColors(payload []byte) ([]LEDModeColor, error) {
	if len(payload)%3 != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of LED mode color size 3", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	modeColors := make([]LEDModeColor, 0, len(payload)/3)
	for r.Remaining() > 0 {
		mode, err := r.U8()
		if err != nil {
			return nil, err
		}
		direction, err := r.U8()
		if err != nil {
			return nil, err
		}
		color, err := r.U8()
		if err != nil {
			return nil, err
		}
		modeColors = append(modeColors, LEDModeColor{Mode: mode, Direction: direction, Color: color})
	}
	return modeColors, nil
}

func DecodeLEDConfigValues(payload []byte) (*LEDConfigValues, error) {
	r := msp.NewPayloadReader(payload)
	brightness, err := r.U8()
	if err != nil {
		return nil, err
	}
	rainbowDelta, err := r.U16()
	if err != nil {
		return nil, err
	}
	rainbowFreq, err := r.U16()
	if err != nil {
		return nil, err
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("payload has %d trailing LED config value bytes", r.Remaining())
	}
	return &LEDConfigValues{
		Brightness:   brightness,
		RainbowDelta: rainbowDelta,
		RainbowFreq:  rainbowFreq,
	}, nil
}

func SetLEDConfigValues(ctx context.Context, client *connection.Client, values LEDConfigValues) (*LEDConfigValuesSetResult, error) {
	if _, err := client.Request(ctx, msp.MSP2SetLedStripConfigValues, EncodeLEDConfigValues(values)); err != nil {
		return nil, fmt.Errorf("led config values request failed: %w", err)
	}
	return &LEDConfigValuesSetResult{
		Values:       values,
		MSPCode:      msp.MSP2SetLedStripConfigValues,
		MSPName:      "MSP2_SET_LED_STRIP_CONFIG_VALUES",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetLEDColors(ctx context.Context, client *connection.Client, colors []LEDColor) (*LEDColorsSetResult, error) {
	if err := ValidateLEDColors(colors); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetLedColors, EncodeLEDColors(colors)); err != nil {
		return nil, fmt.Errorf("led colors request failed: %w", err)
	}
	return &LEDColorsSetResult{
		Colors:       colors,
		MSPCode:      msp.MSPSetLedColors,
		MSPName:      "MSP_SET_LED_COLORS",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetLEDModeColor(ctx context.Context, client *connection.Client, modeColor LEDModeColor) (*LEDModeColorSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetLedStripModecolor, EncodeLEDModeColor(modeColor)); err != nil {
		return nil, fmt.Errorf("led mode color request failed: %w", err)
	}
	return &LEDModeColorSetResult{
		ModeColor:    modeColor,
		MSPCode:      msp.MSPSetLedStripModecolor,
		MSPName:      "MSP_SET_LED_STRIP_MODECOLOR",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func ValidateLEDColors(colors []LEDColor) error {
	if len(colors) == 0 {
		return fmt.Errorf("led colors must contain at least one row")
	}
	for i, color := range colors {
		if color.Hue > 359 {
			return fmt.Errorf("colors[%d].hue must be <= 359", i)
		}
		if color.Index != 0 && color.Index != i {
			return fmt.Errorf("colors[%d].index must match row position %d", i, i)
		}
	}
	return nil
}

func EncodeLEDConfigValues(values LEDConfigValues) []byte {
	payload := []byte{values.Brightness}
	payload = append(payload, byte(values.RainbowDelta), byte(values.RainbowDelta>>8))
	payload = append(payload, byte(values.RainbowFreq), byte(values.RainbowFreq>>8))
	return payload
}

func EncodeLEDColors(colors []LEDColor) []byte {
	payload := make([]byte, 0, len(colors)*4)
	for _, color := range colors {
		payload = appendU16Payload(payload, color.Hue)
		payload = append(payload, color.Sat, color.Val)
	}
	return payload
}

func EncodeLEDModeColor(modeColor LEDModeColor) []byte {
	return []byte{modeColor.Mode, modeColor.Direction, modeColor.Color}
}

func readLEDColors(ctx context.Context, client *connection.Client) ([]LEDColor, error) {
	frame, err := client.Request(ctx, msp.MSPLedColors, nil)
	if err != nil {
		return nil, fmt.Errorf("led colors unavailable: %w", err)
	}
	colors, err := DecodeLEDColors(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("led colors decode failed: %w", err)
	}
	return colors, nil
}

func readLEDModeColors(ctx context.Context, client *connection.Client) ([]LEDModeColor, error) {
	frame, err := client.Request(ctx, msp.MSPLedStripModecolor, nil)
	if err != nil {
		return nil, fmt.Errorf("led mode colors unavailable: %w", err)
	}
	modeColors, err := DecodeLEDModeColors(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("led mode colors decode failed: %w", err)
	}
	return modeColors, nil
}

func readLEDConfigValues(ctx context.Context, client *connection.Client) (*LEDConfigValues, error) {
	frame, err := client.Request(ctx, msp.MSP2GetLedStripConfigValues, nil)
	if err != nil {
		return nil, fmt.Errorf("led config values unavailable: %w", err)
	}
	values, err := DecodeLEDConfigValues(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("led config values decode failed: %w", err)
	}
	return values, nil
}

func decodeLEDConfig(index int, raw uint32) LEDConfig {
	functionID := uint8((raw >> ledFunctionOffset) & ledFunctionMask)
	overlaysMask := uint16((raw >> ledOverlayOffset) & ledOverlayMask)
	directionsMask := uint8((raw >> ledDirectionOffset) & ledDirectionMask)
	function := indexedLetter(ledFunctionLetters, functionID)
	overlays := bitLetters(ledOverlayLetters, uint32(overlaysMask))
	directions := bitLetters(ledDirectionLetters, uint32(directionsMask))
	color := uint8((raw >> ledColorOffset) & ledColorMask)
	return LEDConfig{
		Index:          index,
		Raw:            raw,
		Configured:     raw != 0,
		X:              uint8((raw >> ledXOffset) & ledPositionMask),
		Y:              uint8(raw & ledPositionMask),
		FunctionID:     functionID,
		Function:       function,
		OverlaysMask:   overlaysMask,
		Overlays:       overlays,
		Color:          color,
		DirectionsMask: directionsMask,
		Directions:     directions,
		CLISyntax:      ledCLISyntax(raw, function, overlays, directions, color),
	}
}

func ledCLISyntax(raw uint32, function string, overlays []string, directions []string, color uint8) string {
	x := uint8((raw >> ledXOffset) & ledPositionMask)
	y := uint8(raw & ledPositionMask)
	return fmt.Sprintf("%d,%d:%s:%s%s:%d", x, y, joinLetters(directions), strings.ToUpper(function), joinLetters(overlays), color)
}

func indexedLetter(letters []string, index uint8) string {
	if int(index) >= len(letters) {
		return ""
	}
	return letters[index]
}

func bitLetters(letters []string, mask uint32) []string {
	values := []string{}
	for i, letter := range letters {
		if mask&(1<<i) != 0 {
			values = append(values, letter)
		}
	}
	return values
}

func joinLetters(values []string) string {
	result := ""
	for _, value := range values {
		result += value
	}
	return result
}
