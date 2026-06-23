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

type OSDPositionSetConfig struct {
	Index  uint8  `json:"index"`
	Raw    uint16 `json:"raw"`
	Screen uint8  `json:"screen"`
}

type OSDPositionSetResult struct {
	Config       OSDPositionSetConfig `json:"config"`
	MSPCode      uint16               `json:"msp_code"`
	MSPName      string               `json:"msp_name"`
	Acknowledged bool                 `json:"acknowledged"`
	SaveRequired bool                 `json:"save_required"`
}

type OSDStatSetConfig struct {
	Index   uint8 `json:"index"`
	Enabled bool  `json:"enabled"`
}

type OSDStatSetResult struct {
	Config       OSDStatSetConfig `json:"config"`
	MSPCode      uint16           `json:"msp_code"`
	MSPName      string           `json:"msp_name"`
	Acknowledged bool             `json:"acknowledged"`
	SaveRequired bool             `json:"save_required"`
}

type OSDTimerSetConfig struct {
	Index uint8  `json:"index"`
	Value uint16 `json:"value"`
}

type OSDTimerSetResult struct {
	Config       OSDTimerSetConfig `json:"config"`
	MSPCode      uint16            `json:"msp_code"`
	MSPName      string            `json:"msp_name"`
	Acknowledged bool              `json:"acknowledged"`
	SaveRequired bool              `json:"save_required"`
}

type OSDGeneralSetConfig struct {
	VideoSystem       *uint8             `json:"video_system,omitempty"`
	Units             *uint8             `json:"units,omitempty"`
	Alarms            *OSDAlarmSetConfig `json:"alarms,omitempty"`
	EnabledWarnings   *uint32            `json:"enabled_warnings,omitempty"`
	SelectedProfile   *uint8             `json:"selected_profile,omitempty"`
	StickOverlayMode  *uint8             `json:"stick_overlay_mode,omitempty"`
	CameraFrameWidth  *uint8             `json:"camera_frame_width,omitempty"`
	CameraFrameHeight *uint8             `json:"camera_frame_height,omitempty"`
}

type OSDAlarmSetConfig struct {
	RSSI        *uint8  `json:"rssi,omitempty"`
	CapacityMAh *uint16 `json:"capacity_mah,omitempty"`
	AltitudeM   *uint16 `json:"altitude_m,omitempty"`
	LinkQuality *uint16 `json:"link_quality,omitempty"`
	RSSIDBm     *int16  `json:"rssi_dbm,omitempty"`
}

type OSDGeneralSetResult struct {
	Requested      OSDGeneralSetConfig `json:"requested"`
	Config         OSDConfig           `json:"config"`
	MSPCode        uint16              `json:"msp_code"`
	MSPName        string              `json:"msp_name"`
	Acknowledged   bool                `json:"acknowledged"`
	SaveRequired   bool                `json:"save_required"`
	RebootPossible bool                `json:"reboot_possible"`
}

type OSDVideoSystemSetConfig struct {
	VideoSystem     uint8  `json:"video_system"`
	VideoSystemName string `json:"video_system_name,omitempty"`
}

type OSDVideoSystemSetResult struct {
	Config         OSDVideoSystemSetConfig `json:"config"`
	MSPCode        uint16                  `json:"msp_code"`
	MSPName        string                  `json:"msp_name"`
	Acknowledged   bool                    `json:"acknowledged"`
	SaveRequired   bool                    `json:"save_required"`
	RebootPossible bool                    `json:"reboot_possible"`
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

func SetOSDPosition(ctx context.Context, client *connection.Client, config OSDPositionSetConfig) (*OSDPositionSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetOSDConfig, EncodeOSDPosition(config)); err != nil {
		return nil, fmt.Errorf("osd position request failed: %w", err)
	}
	return &OSDPositionSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetOSDConfig,
		MSPName:      "MSP_SET_OSD_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeOSDPosition(config OSDPositionSetConfig) []byte {
	payload := appendU16Payload([]byte{config.Index}, config.Raw)
	return append(payload, config.Screen)
}

func SetOSDStat(ctx context.Context, client *connection.Client, config OSDStatSetConfig) (*OSDStatSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetOSDConfig, EncodeOSDStat(config)); err != nil {
		return nil, fmt.Errorf("osd statistic request failed: %w", err)
	}
	return &OSDStatSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetOSDConfig,
		MSPName:      "MSP_SET_OSD_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeOSDStat(config OSDStatSetConfig) []byte {
	payload := appendU16Payload([]byte{config.Index}, uint16(boolByte(config.Enabled)))
	return append(payload, 0)
}

func SetOSDTimer(ctx context.Context, client *connection.Client, config OSDTimerSetConfig) (*OSDTimerSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetOSDConfig, EncodeOSDTimer(config)); err != nil {
		return nil, fmt.Errorf("osd timer request failed: %w", err)
	}
	return &OSDTimerSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetOSDConfig,
		MSPName:      "MSP_SET_OSD_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeOSDTimer(config OSDTimerSetConfig) []byte {
	payload := []byte{254, config.Index}
	return appendU16Payload(payload, config.Value)
}

func SetOSDGeneralConfig(ctx context.Context, client *connection.Client, patch OSDGeneralSetConfig) (*OSDGeneralSetResult, error) {
	if err := ValidateOSDGeneralSetConfig(patch); err != nil {
		return nil, err
	}
	frame, err := client.Request(ctx, msp.MSPOSDConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("osd config unavailable before general config update: %w", err)
	}
	config, err := DecodeOSDConfig(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("osd config decode failed before general config update: %w", err)
	}
	previousVideoSystem := config.VideoSystem
	if err := applyOSDGeneralPatch(config, patch); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetOSDConfig, EncodeOSDGeneralConfig(config)); err != nil {
		return nil, fmt.Errorf("osd general config request failed: %w", err)
	}
	return &OSDGeneralSetResult{
		Requested:      patch,
		Config:         *config,
		MSPCode:        msp.MSPSetOSDConfig,
		MSPName:        "MSP_SET_OSD_CONFIG",
		Acknowledged:   true,
		SaveRequired:   true,
		RebootPossible: config.VideoSystem == 3 || previousVideoSystem == 3,
	}, nil
}

func EncodeOSDGeneralConfig(config *OSDConfig) []byte {
	return encodeOSDGeneralConfig(config, config.VideoSystem)
}

func SetOSDVideoSystem(ctx context.Context, client *connection.Client, videoSystem uint8) (*OSDVideoSystemSetResult, error) {
	if videoSystem > 3 {
		return nil, fmt.Errorf("video_system must be 0 (AUTO), 1 (PAL), 2 (NTSC), or 3 (HD)")
	}
	frame, err := client.Request(ctx, msp.MSPOSDConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("osd config unavailable before video system update: %w", err)
	}
	config, err := DecodeOSDConfig(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("osd config decode failed before video system update: %w", err)
	}
	if _, err := client.Request(ctx, msp.MSPSetOSDConfig, EncodeOSDVideoSystem(config, videoSystem)); err != nil {
		return nil, fmt.Errorf("osd video system request failed: %w", err)
	}
	resultConfig := OSDVideoSystemSetConfig{VideoSystem: videoSystem, VideoSystemName: lookupVideoSystem(videoSystem)}
	return &OSDVideoSystemSetResult{
		Config:         resultConfig,
		MSPCode:        msp.MSPSetOSDConfig,
		MSPName:        "MSP_SET_OSD_CONFIG",
		Acknowledged:   true,
		SaveRequired:   true,
		RebootPossible: videoSystem == 3 || config.VideoSystem == 3,
	}, nil
}

func EncodeOSDVideoSystem(config *OSDConfig, videoSystem uint8) []byte {
	return encodeOSDGeneralConfig(config, videoSystem)
}

func ValidateOSDGeneralSetConfig(patch OSDGeneralSetConfig) error {
	if patch.VideoSystem != nil && *patch.VideoSystem > 3 {
		return fmt.Errorf("video_system must be 0 (AUTO), 1 (PAL), 2 (NTSC), or 3 (HD)")
	}
	if patch.Units != nil && *patch.Units > 2 {
		return fmt.Errorf("units must be 0 (IMPERIAL), 1 (METRIC), or 2 (BRITISH)")
	}
	if patch.Alarms != nil {
		alarms := patch.Alarms
		if alarms.RSSI != nil && *alarms.RSSI > 100 {
			return fmt.Errorf("alarms.rssi must be 0..100")
		}
		if alarms.CapacityMAh != nil && *alarms.CapacityMAh > 20000 {
			return fmt.Errorf("alarms.capacity_mah must be 0..20000")
		}
		if alarms.AltitudeM != nil && *alarms.AltitudeM > 10000 {
			return fmt.Errorf("alarms.altitude_m must be 0..10000")
		}
		if alarms.LinkQuality != nil && *alarms.LinkQuality > 100 {
			return fmt.Errorf("alarms.link_quality must be 0..100")
		}
	}
	if patch.StickOverlayMode != nil && (*patch.StickOverlayMode < 1 || *patch.StickOverlayMode > 4) {
		return fmt.Errorf("stick_overlay_mode must be 1..4")
	}
	return nil
}

func encodeOSDGeneralConfig(config *OSDConfig, videoSystem uint8) []byte {
	payload := []byte{255, videoSystem, config.Units, config.Alarms.RSSI}
	payload = appendU16Payload(payload, config.Alarms.CapacityMAh)
	payload = appendU16Payload(payload, 0)
	payload = appendU16Payload(payload, config.Alarms.AltitudeM)
	payload = appendU16Payload(payload, uint16(config.EnabledWarnings))
	payload = appendU32Payload(payload, config.EnabledWarnings)
	payload = append(payload, config.SelectedProfile, config.StickOverlayMode, config.CameraFrameWidth, config.CameraFrameHeight)
	payload = appendU16Payload(payload, config.Alarms.LinkQuality)
	return appendU16Payload(payload, uint16(config.Alarms.RSSIDBm))
}

func applyOSDGeneralPatch(config *OSDConfig, patch OSDGeneralSetConfig) error {
	if err := ValidateOSDGeneralSetConfig(patch); err != nil {
		return err
	}
	if patch.VideoSystem != nil {
		config.VideoSystem = *patch.VideoSystem
		config.VideoSystemName = lookupVideoSystem(*patch.VideoSystem)
	}
	if patch.Units != nil {
		config.Units = *patch.Units
		config.UnitsName = lookupOSDUnits(*patch.Units)
	}
	if patch.Alarms != nil {
		alarms := patch.Alarms
		if alarms.RSSI != nil {
			config.Alarms.RSSI = *alarms.RSSI
		}
		if alarms.CapacityMAh != nil {
			config.Alarms.CapacityMAh = *alarms.CapacityMAh
		}
		if alarms.AltitudeM != nil {
			config.Alarms.AltitudeM = *alarms.AltitudeM
		}
		if alarms.LinkQuality != nil {
			config.Alarms.LinkQuality = *alarms.LinkQuality
		}
		if alarms.RSSIDBm != nil {
			config.Alarms.RSSIDBm = *alarms.RSSIDBm
		}
	}
	if patch.EnabledWarnings != nil {
		config.EnabledWarnings = *patch.EnabledWarnings
	}
	if patch.SelectedProfile != nil {
		config.SelectedProfile = *patch.SelectedProfile
	}
	if patch.StickOverlayMode != nil {
		config.StickOverlayMode = *patch.StickOverlayMode
	}
	if patch.CameraFrameWidth != nil {
		config.CameraFrameWidth = *patch.CameraFrameWidth
	}
	if patch.CameraFrameHeight != nil {
		config.CameraFrameHeight = *patch.CameraFrameHeight
	}
	return nil
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
