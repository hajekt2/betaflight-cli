package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const (
	osdFlagFeature            = 1 << 0
	osdFlagHardwareFrSkyOSD   = 1 << 3
	osdFlagHardwareMax7456    = 1 << 4
	osdFlagDeviceDetected     = 1 << 5
	osdFlagMSPDevice          = 1 << 6
	osdFlagHardwareAirbotOSD  = 1 << 7
	osdPositionCoordinateMask = 0x07ff
	osdPositionXYMask         = 0x1f
	osdPositionXHDMask        = 1 << 10
	osdProfileMask            = 0x3800
	osdTypeMask               = 0xc000
)

type OSDStatus struct {
	Config   *OSDConfig   `json:"config,omitempty"`
	Canvas   *OSDCanvas   `json:"canvas,omitempty"`
	Warnings *OSDWarnings `json:"warnings,omitempty"`
}

type OSDConfig struct {
	Flags              OSDFlags      `json:"flags"`
	VideoSystem        uint8         `json:"video_system"`
	VideoSystemName    string        `json:"video_system_name,omitempty"`
	Units              uint8         `json:"units"`
	UnitsName          string        `json:"units_name,omitempty"`
	Alarms             OSDAlarms     `json:"alarms"`
	ItemPositions      []OSDPosition `json:"item_positions,omitempty"`
	StatisticsEnabled  []bool        `json:"statistics_enabled,omitempty"`
	Timers             []uint16      `json:"timers,omitempty"`
	WarningsCount      uint8         `json:"warnings_count"`
	EnabledWarnings    uint32        `json:"enabled_warnings"`
	ProfilesAvailable  uint8         `json:"profiles_available"`
	SelectedProfile    uint8         `json:"selected_profile"`
	StickOverlayMode   uint8         `json:"stick_overlay_mode"`
	CameraFrameWidth   uint8         `json:"camera_frame_width"`
	CameraFrameHeight  uint8         `json:"camera_frame_height"`
	RawItemCount       uint8         `json:"raw_item_count"`
	RawStatCount       uint8         `json:"raw_stat_count"`
	RawTimerCount      uint8         `json:"raw_timer_count"`
	LegacyWarningFlags uint16        `json:"legacy_warning_flags"`
}

type OSDFlags struct {
	Raw                 uint8 `json:"raw"`
	FeatureEnabled      bool  `json:"feature_enabled"`
	HardwareFrSkyOSD    bool  `json:"hardware_frsky_osd"`
	HardwareMax7456     bool  `json:"hardware_max7456"`
	DeviceDetected      bool  `json:"device_detected"`
	MSPDevice           bool  `json:"msp_device"`
	HardwareAirbotTheia bool  `json:"hardware_airbot_theia"`
}

type OSDAlarms struct {
	RSSI        uint8  `json:"rssi"`
	CapacityMAh uint16 `json:"capacity_mah"`
	AltitudeM   uint16 `json:"altitude_m"`
	LinkQuality uint16 `json:"link_quality"`
	RSSIDBm     int16  `json:"rssi_dbm"`
}

type OSDPosition struct {
	Index       int    `json:"index"`
	Raw         uint16 `json:"raw"`
	Visible     bool   `json:"visible"`
	X           uint8  `json:"x"`
	Y           uint8  `json:"y"`
	ProfileMask uint16 `json:"profile_mask"`
	ElementType uint8  `json:"element_type"`
}

type OSDCanvas struct {
	Columns uint8 `json:"columns"`
	Rows    uint8 `json:"rows"`
}

type OSDWarnings struct {
	DisplayAttributes uint8  `json:"display_attributes"`
	Text              string `json:"text"`
}

type OSDCanvasSetConfig struct {
	Columns uint8 `json:"columns"`
	Rows    uint8 `json:"rows"`
}

type OSDCanvasSetResult struct {
	Config         OSDCanvasSetConfig `json:"config"`
	MSPCode        uint16             `json:"msp_code"`
	MSPName        string             `json:"msp_name"`
	Acknowledged   bool               `json:"acknowledged"`
	SaveRequired   bool               `json:"save_required"`
	RebootPossible bool               `json:"reboot_possible"`
}

func ReadOSDStatus(ctx context.Context, client *connection.Client) (*OSDStatus, []string, error) {
	status := &OSDStatus{}
	warnings := []string{}
	frame, err := client.Request(ctx, msp.MSPOSDConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("osd config unavailable: %w", err)
	}
	config, err := DecodeOSDConfig(frame.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("osd config decode failed: %w", err)
	}
	status.Config = config
	if canvas, err := readOSDCanvas(ctx, client); err == nil {
		status.Canvas = canvas
	} else {
		warnings = append(warnings, err.Error())
	}
	if renderedWarnings, err := readOSDWarnings(ctx, client); err == nil {
		status.Warnings = renderedWarnings
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func SetOSDCanvas(ctx context.Context, client *connection.Client, config OSDCanvasSetConfig) (*OSDCanvasSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetOSDCanvas, EncodeOSDCanvas(config)); err != nil {
		return nil, fmt.Errorf("osd canvas request failed: %w", err)
	}
	return &OSDCanvasSetResult{
		Config:         config,
		MSPCode:        msp.MSPSetOSDCanvas,
		MSPName:        "MSP_SET_OSD_CANVAS",
		Acknowledged:   true,
		SaveRequired:   true,
		RebootPossible: true,
	}, nil
}

func EncodeOSDCanvas(config OSDCanvasSetConfig) []byte {
	return []byte{config.Columns, config.Rows}
}

func DecodeOSDConfig(payload []byte) (*OSDConfig, error) {
	r := msp.NewPayloadReader(payload)
	flags, err := r.U8()
	if err != nil {
		return nil, err
	}
	videoSystem, err := r.U8()
	if err != nil {
		return nil, err
	}
	units, err := r.U8()
	if err != nil {
		return nil, err
	}
	rssiAlarm, err := r.U8()
	if err != nil {
		return nil, err
	}
	capAlarm, err := r.U16()
	if err != nil {
		return nil, err
	}
	if _, err := r.U8(); err != nil {
		return nil, err
	}
	itemCount, err := r.U8()
	if err != nil {
		return nil, err
	}
	altAlarm, err := r.U16()
	if err != nil {
		return nil, err
	}
	config := &OSDConfig{
		Flags:           decodeOSDFlags(flags),
		VideoSystem:     videoSystem,
		VideoSystemName: lookupVideoSystem(videoSystem),
		Units:           units,
		UnitsName:       lookupOSDUnits(units),
		Alarms: OSDAlarms{
			RSSI:        rssiAlarm,
			CapacityMAh: capAlarm,
			AltitudeM:   altAlarm,
		},
		RawItemCount: itemCount,
	}
	config.ItemPositions, err = readOSDPositions(r, itemCount)
	if err != nil {
		return nil, err
	}
	statCount, err := r.U8()
	if err != nil {
		return nil, err
	}
	config.RawStatCount = statCount
	config.StatisticsEnabled, err = readBoolBytes(r, statCount)
	if err != nil {
		return nil, err
	}
	timerCount, err := r.U8()
	if err != nil {
		return nil, err
	}
	config.RawTimerCount = timerCount
	config.Timers, err = readU16Values(r, timerCount)
	if err != nil {
		return nil, err
	}
	legacyWarnings, err := r.U16()
	if err != nil {
		return nil, err
	}
	config.LegacyWarningFlags = legacyWarnings
	if r.Remaining() >= 1 {
		config.WarningsCount, err = r.U8()
		if err != nil {
			return nil, err
		}
	}
	if r.Remaining() >= 4 {
		config.EnabledWarnings, err = r.U32()
		if err != nil {
			return nil, err
		}
	} else {
		config.EnabledWarnings = uint32(legacyWarnings)
	}
	if r.Remaining() >= 1 {
		config.ProfilesAvailable, err = r.U8()
		if err != nil {
			return nil, err
		}
	}
	if r.Remaining() >= 1 {
		config.SelectedProfile, err = r.U8()
		if err != nil {
			return nil, err
		}
	}
	if r.Remaining() >= 1 {
		config.StickOverlayMode, err = r.U8()
		if err != nil {
			return nil, err
		}
	}
	if r.Remaining() >= 1 {
		config.CameraFrameWidth, err = r.U8()
		if err != nil {
			return nil, err
		}
	}
	if r.Remaining() >= 1 {
		config.CameraFrameHeight, err = r.U8()
		if err != nil {
			return nil, err
		}
	}
	if r.Remaining() >= 2 {
		config.Alarms.LinkQuality, err = r.U16()
		if err != nil {
			return nil, err
		}
	}
	if r.Remaining() >= 2 {
		rssiDBm, err := r.U16()
		if err != nil {
			return nil, err
		}
		config.Alarms.RSSIDBm = int16(rssiDBm)
	}
	return config, nil
}

func DecodeOSDCanvas(payload []byte) (*OSDCanvas, error) {
	r := msp.NewPayloadReader(payload)
	cols, err := r.U8()
	if err != nil {
		return nil, err
	}
	rows, err := r.U8()
	if err != nil {
		return nil, err
	}
	return &OSDCanvas{Columns: cols, Rows: rows}, nil
}

func DecodeOSDWarnings(payload []byte) (*OSDWarnings, error) {
	r := msp.NewPayloadReader(payload)
	attrs, err := r.U8()
	if err != nil {
		return nil, err
	}
	text, err := r.PString()
	if err != nil {
		return nil, err
	}
	return &OSDWarnings{DisplayAttributes: attrs, Text: text}, nil
}

func readOSDCanvas(ctx context.Context, client *connection.Client) (*OSDCanvas, error) {
	frame, err := client.Request(ctx, msp.MSPOSDCanvas, nil)
	if err != nil {
		return nil, fmt.Errorf("osd canvas unavailable: %w", err)
	}
	return DecodeOSDCanvas(frame.Payload)
}

func readOSDWarnings(ctx context.Context, client *connection.Client) (*OSDWarnings, error) {
	frame, err := client.Request(ctx, msp.MSP2GetOSDWarnings, nil)
	if err != nil {
		return nil, fmt.Errorf("osd warnings unavailable: %w", err)
	}
	return DecodeOSDWarnings(frame.Payload)
}

func readOSDPositions(r *msp.PayloadReader, count uint8) ([]OSDPosition, error) {
	positions := make([]OSDPosition, 0, count)
	for i := 0; i < int(count); i++ {
		raw, err := r.U16()
		if err != nil {
			return nil, err
		}
		coord := raw & osdPositionCoordinateMask
		positions = append(positions, OSDPosition{
			Index:       i,
			Raw:         raw,
			Visible:     raw&osdProfileMask != 0,
			X:           uint8((coord & osdPositionXYMask) | ((coord & osdPositionXHDMask) >> 5)),
			Y:           uint8((coord >> 5) & osdPositionXYMask),
			ProfileMask: raw & osdProfileMask,
			ElementType: uint8((raw & osdTypeMask) >> 14),
		})
	}
	return positions, nil
}

func readBoolBytes(r *msp.PayloadReader, count uint8) ([]bool, error) {
	values := make([]bool, 0, count)
	for i := 0; i < int(count); i++ {
		raw, err := r.U8()
		if err != nil {
			return nil, err
		}
		values = append(values, raw != 0)
	}
	return values, nil
}

func readU16Values(r *msp.PayloadReader, count uint8) ([]uint16, error) {
	values := make([]uint16, 0, count)
	for i := 0; i < int(count); i++ {
		value, err := r.U16()
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return values, nil
}

func decodeOSDFlags(raw uint8) OSDFlags {
	return OSDFlags{
		Raw:                 raw,
		FeatureEnabled:      raw&osdFlagFeature != 0,
		HardwareFrSkyOSD:    raw&osdFlagHardwareFrSkyOSD != 0,
		HardwareMax7456:     raw&osdFlagHardwareMax7456 != 0,
		DeviceDetected:      raw&osdFlagDeviceDetected != 0,
		MSPDevice:           raw&osdFlagMSPDevice != 0,
		HardwareAirbotTheia: raw&osdFlagHardwareAirbotOSD != 0,
	}
}

func lookupVideoSystem(v uint8) string {
	switch v {
	case 0:
		return "AUTO"
	case 1:
		return "PAL"
	case 2:
		return "NTSC"
	case 3:
		return "HD"
	default:
		return ""
	}
}

func lookupOSDUnits(v uint8) string {
	switch v {
	case 0:
		return "IMPERIAL"
	case 1:
		return "METRIC"
	case 2:
		return "BRITISH"
	default:
		return ""
	}
}
