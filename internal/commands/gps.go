package commands

import (
	"context"
	"encoding/hex"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type GPSStatus struct {
	Config     *GPSConfig     `json:"config,omitempty"`
	Position   *GPSPosition   `json:"position,omitempty"`
	Home       *GPSHome       `json:"home,omitempty"`
	Rescue     *GPSRescue     `json:"rescue,omitempty"`
	RescuePID  *GPSRescuePID  `json:"rescue_pid,omitempty"`
	Satellites []GPSSatellite `json:"satellites,omitempty"`
	Statistics *GPSStatistics `json:"statistics,omitempty"`
}

type GPSConfig struct {
	Provider        uint8 `json:"provider"`
	SBASMode        uint8 `json:"sbas_mode"`
	AutoConfig      bool  `json:"auto_config"`
	AutoBaud        bool  `json:"auto_baud"`
	HomePointOnce   *bool `json:"home_point_once,omitempty"`
	UBloxUseGalileo *bool `json:"ublox_use_galileo,omitempty"`
}

type GPSConfigSetResult struct {
	Config       GPSConfig `json:"config"`
	MSPCode      uint16    `json:"msp_code"`
	MSPName      string    `json:"msp_name"`
	Acknowledged bool      `json:"acknowledged"`
	SaveRequired bool      `json:"save_required"`
}

type GPSPosition struct {
	Fix              bool    `json:"fix"`
	Satellites       uint8   `json:"satellites"`
	LatitudeE7       int32   `json:"latitude_e7"`
	LongitudeE7      int32   `json:"longitude_e7"`
	LatitudeDegrees  float64 `json:"latitude_degrees"`
	LongitudeDegrees float64 `json:"longitude_degrees"`
	AltitudeM        uint16  `json:"altitude_m"`
	GroundSpeedCMS   uint16  `json:"ground_speed_cm_s"`
	GroundCourseDeg  float64 `json:"ground_course_degrees"`
	PDOP             *uint16 `json:"pdop,omitempty"`
}

type GPSHome struct {
	DistanceM    uint16 `json:"distance_m"`
	DirectionDeg uint16 `json:"direction_degrees"`
	Update       bool   `json:"update"`
}

type GPSRescue struct {
	MaxRescueAngle        uint16  `json:"max_rescue_angle"`
	ReturnAltitudeM       uint16  `json:"return_altitude_m"`
	DescentDistanceM      uint16  `json:"descent_distance_m"`
	GroundSpeedCMS        uint16  `json:"ground_speed_cm_s"`
	ThrottleMin           uint16  `json:"throttle_min"`
	ThrottleMax           uint16  `json:"throttle_max"`
	ThrottleHover         uint16  `json:"throttle_hover"`
	SanityChecks          uint8   `json:"sanity_checks"`
	MinSats               uint8   `json:"min_sats"`
	AscendRate            *uint16 `json:"ascend_rate,omitempty"`
	DescendRate           *uint16 `json:"descend_rate,omitempty"`
	AllowArmingWithoutFix *bool   `json:"allow_arming_without_fix,omitempty"`
	AltitudeMode          *uint8  `json:"altitude_mode,omitempty"`
	MinStartDistanceM     *uint16 `json:"min_start_distance_m,omitempty"`
	InitialClimbM         *uint16 `json:"initial_climb_m,omitempty"`
}

type GPSRescuePID struct {
	AltitudeP uint16 `json:"altitude_p"`
	AltitudeI uint16 `json:"altitude_i"`
	AltitudeD uint16 `json:"altitude_d"`
	VelocityP uint16 `json:"velocity_p"`
	VelocityI uint16 `json:"velocity_i"`
	VelocityD uint16 `json:"velocity_d"`
	YawP      uint16 `json:"yaw_p"`
}

type GPSRescueSetResult struct {
	Config       GPSRescue `json:"config"`
	MSPCode      uint16    `json:"msp_code"`
	MSPName      string    `json:"msp_name"`
	Acknowledged bool      `json:"acknowledged"`
	SaveRequired bool      `json:"save_required"`
}

type GPSRescuePIDSetResult struct {
	Config       GPSRescuePID `json:"config"`
	MSPCode      uint16       `json:"msp_code"`
	MSPName      string       `json:"msp_name"`
	Acknowledged bool         `json:"acknowledged"`
	SaveRequired bool         `json:"save_required"`
}

type GPSSatellite struct {
	Channel uint8 `json:"channel"`
	SVID    uint8 `json:"svid"`
	Quality uint8 `json:"quality"`
	CNO     uint8 `json:"cno"`
}

// GPSStatistics surfaces an uninterpreted MSP_GPSSTATISTICS (0xA6) payload.
//
// Upstream note (2026.6.1): msp_protocol.h defines the code ("Get GPS
// debugging data", line 220) but src/main/msp/msp.c contains no case handler
// for it, so no field layout is derivable from firmware; the payload is
// surfaced raw rather than guessed.
type GPSStatistics struct {
	PayloadLen int    `json:"payload_len"`
	PayloadHex string `json:"payload_hex"`
}

func ReadGPSStatus(ctx context.Context, client *connection.Client) (*GPSStatus, []string, error) {
	status := &GPSStatus{}
	warnings := []string{}
	frame, err := client.Request(ctx, msp.MSPGPSConfig, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("gps config unavailable: %w", err)
	}
	config, err := DecodeGPSConfig(frame.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("gps config decode failed: %w", err)
	}
	status.Config = config
	if position, err := readGPSPosition(ctx, client); err == nil {
		status.Position = position
	} else {
		warnings = append(warnings, err.Error())
	}
	if home, err := readGPSHome(ctx, client); err == nil {
		status.Home = home
	} else {
		warnings = append(warnings, err.Error())
	}
	if rescue, err := readGPSRescue(ctx, client); err == nil {
		status.Rescue = rescue
	} else {
		warnings = append(warnings, err.Error())
	}
	if rescuePID, err := readGPSRescuePID(ctx, client); err == nil {
		status.RescuePID = rescuePID
	} else {
		warnings = append(warnings, err.Error())
	}
	if satellites, err := readGPSSatellites(ctx, client); err == nil {
		status.Satellites = satellites
	} else {
		warnings = append(warnings, err.Error())
	}
	if statistics, err := readGPSStatistics(ctx, client); err == nil {
		status.Statistics = statistics
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func SetGPSConfig(ctx context.Context, client *connection.Client, config GPSConfig) (*GPSConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetGPSConfig, EncodeGPSConfig(config)); err != nil {
		return nil, fmt.Errorf("gps config request failed: %w", err)
	}
	return &GPSConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetGPSConfig,
		MSPName:      "MSP_SET_GPS_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetGPSRescue(ctx context.Context, client *connection.Client, config GPSRescue) (*GPSRescueSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetGPSRescue, EncodeGPSRescue(config)); err != nil {
		return nil, fmt.Errorf("gps rescue request failed: %w", err)
	}
	return &GPSRescueSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetGPSRescue,
		MSPName:      "MSP_SET_GPS_RESCUE",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetGPSRescuePID(ctx context.Context, client *connection.Client, config GPSRescuePID) (*GPSRescuePIDSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetGPSRescuePids, EncodeGPSRescuePID(config)); err != nil {
		return nil, fmt.Errorf("gps rescue pids request failed: %w", err)
	}
	return &GPSRescuePIDSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetGPSRescuePids,
		MSPName:      "MSP_SET_GPS_RESCUE_PIDS",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeGPSConfig(config GPSConfig) []byte {
	payload := []byte{config.Provider, config.SBASMode, boolByte(config.AutoConfig), boolByte(config.AutoBaud)}
	if config.HomePointOnce != nil {
		payload = append(payload, boolByte(*config.HomePointOnce))
	}
	if config.UBloxUseGalileo != nil {
		payload = append(payload, boolByte(*config.UBloxUseGalileo))
	}
	return payload
}

func EncodeGPSRescue(config GPSRescue) []byte {
	payload := appendU16GPS(nil, config.MaxRescueAngle)
	payload = appendU16GPS(payload, config.ReturnAltitudeM)
	payload = appendU16GPS(payload, config.DescentDistanceM)
	payload = appendU16GPS(payload, config.GroundSpeedCMS)
	payload = appendU16GPS(payload, config.ThrottleMin)
	payload = appendU16GPS(payload, config.ThrottleMax)
	payload = appendU16GPS(payload, config.ThrottleHover)
	payload = append(payload, config.SanityChecks, config.MinSats)
	if config.AscendRate != nil && config.DescendRate != nil && config.AllowArmingWithoutFix != nil && config.AltitudeMode != nil {
		payload = appendU16GPS(payload, *config.AscendRate)
		payload = appendU16GPS(payload, *config.DescendRate)
		payload = append(payload, boolByte(*config.AllowArmingWithoutFix), *config.AltitudeMode)
	}
	if config.MinStartDistanceM != nil {
		payload = appendU16GPS(payload, *config.MinStartDistanceM)
	}
	if config.InitialClimbM != nil {
		payload = appendU16GPS(payload, *config.InitialClimbM)
	}
	return payload
}

func EncodeGPSRescuePID(config GPSRescuePID) []byte {
	payload := appendU16GPS(nil, config.AltitudeP)
	payload = appendU16GPS(payload, config.AltitudeI)
	payload = appendU16GPS(payload, config.AltitudeD)
	payload = appendU16GPS(payload, config.VelocityP)
	payload = appendU16GPS(payload, config.VelocityI)
	payload = appendU16GPS(payload, config.VelocityD)
	payload = appendU16GPS(payload, config.YawP)
	return payload
}

func appendU16GPS(dst []byte, value uint16) []byte {
	return append(dst, byte(value), byte(value>>8))
}

func boolByte(v bool) byte {
	if v {
		return 1
	}
	return 0
}

func DecodeGPSConfig(payload []byte) (*GPSConfig, error) {
	r := msp.NewPayloadReader(payload)
	provider, err := r.U8()
	if err != nil {
		return nil, err
	}
	sbas, err := r.U8()
	if err != nil {
		return nil, err
	}
	autoConfig, err := r.U8()
	if err != nil {
		return nil, err
	}
	autoBaud, err := r.U8()
	if err != nil {
		return nil, err
	}
	config := &GPSConfig{
		Provider:   provider,
		SBASMode:   sbas,
		AutoConfig: autoConfig != 0,
		AutoBaud:   autoBaud != 0,
	}
	if r.Remaining() >= 1 {
		homePointOnce, err := r.U8()
		if err != nil {
			return nil, err
		}
		v := homePointOnce != 0
		config.HomePointOnce = &v
	}
	if r.Remaining() >= 1 {
		useGalileo, err := r.U8()
		if err != nil {
			return nil, err
		}
		v := useGalileo != 0
		config.UBloxUseGalileo = &v
	}
	return config, nil
}

func DecodeGPSPosition(payload []byte) (*GPSPosition, error) {
	r := msp.NewPayloadReader(payload)
	fix, err := r.U8()
	if err != nil {
		return nil, err
	}
	sats, err := r.U8()
	if err != nil {
		return nil, err
	}
	latRaw, err := r.U32()
	if err != nil {
		return nil, err
	}
	lonRaw, err := r.U32()
	if err != nil {
		return nil, err
	}
	altitude, err := r.U16()
	if err != nil {
		return nil, err
	}
	speed, err := r.U16()
	if err != nil {
		return nil, err
	}
	course, err := r.U16()
	if err != nil {
		return nil, err
	}
	lat := int32(latRaw)
	lon := int32(lonRaw)
	position := &GPSPosition{
		Fix:              fix != 0,
		Satellites:       sats,
		LatitudeE7:       lat,
		LongitudeE7:      lon,
		LatitudeDegrees:  float64(lat) / 1e7,
		LongitudeDegrees: float64(lon) / 1e7,
		AltitudeM:        altitude,
		GroundSpeedCMS:   speed,
		GroundCourseDeg:  float64(course) / 10,
	}
	if r.Remaining() >= 2 {
		pdop, err := r.U16()
		if err != nil {
			return nil, err
		}
		position.PDOP = &pdop
	}
	return position, nil
}

func DecodeGPSHome(payload []byte) (*GPSHome, error) {
	r := msp.NewPayloadReader(payload)
	distance, err := r.U16()
	if err != nil {
		return nil, err
	}
	direction, err := r.U16()
	if err != nil {
		return nil, err
	}
	update, err := r.U8()
	if err != nil {
		return nil, err
	}
	return &GPSHome{DistanceM: distance, DirectionDeg: direction, Update: update != 0}, nil
}

func DecodeGPSRescue(payload []byte) (*GPSRescue, error) {
	r := msp.NewPayloadReader(payload)
	rescue := &GPSRescue{}
	var err error
	if rescue.MaxRescueAngle, err = r.U16(); err != nil {
		return nil, err
	}
	if rescue.ReturnAltitudeM, err = r.U16(); err != nil {
		return nil, err
	}
	if rescue.DescentDistanceM, err = r.U16(); err != nil {
		return nil, err
	}
	if rescue.GroundSpeedCMS, err = r.U16(); err != nil {
		return nil, err
	}
	if rescue.ThrottleMin, err = r.U16(); err != nil {
		return nil, err
	}
	if rescue.ThrottleMax, err = r.U16(); err != nil {
		return nil, err
	}
	if rescue.ThrottleHover, err = r.U16(); err != nil {
		return nil, err
	}
	if rescue.SanityChecks, err = r.U8(); err != nil {
		return nil, err
	}
	if rescue.MinSats, err = r.U8(); err != nil {
		return nil, err
	}
	if r.Remaining() >= 2 {
		v, err := r.U16()
		if err != nil {
			return nil, err
		}
		rescue.AscendRate = &v
	}
	if r.Remaining() >= 2 {
		v, err := r.U16()
		if err != nil {
			return nil, err
		}
		rescue.DescendRate = &v
	}
	if r.Remaining() >= 1 {
		raw, err := r.U8()
		if err != nil {
			return nil, err
		}
		v := raw != 0
		rescue.AllowArmingWithoutFix = &v
	}
	if r.Remaining() >= 1 {
		v, err := r.U8()
		if err != nil {
			return nil, err
		}
		rescue.AltitudeMode = &v
	}
	if r.Remaining() >= 2 {
		v, err := r.U16()
		if err != nil {
			return nil, err
		}
		rescue.MinStartDistanceM = &v
	}
	if r.Remaining() >= 2 {
		v, err := r.U16()
		if err != nil {
			return nil, err
		}
		rescue.InitialClimbM = &v
	}
	return rescue, nil
}

func DecodeGPSRescuePID(payload []byte) (*GPSRescuePID, error) {
	r := msp.NewPayloadReader(payload)
	pid := &GPSRescuePID{}
	var err error
	if pid.AltitudeP, err = r.U16(); err != nil {
		return nil, err
	}
	if pid.AltitudeI, err = r.U16(); err != nil {
		return nil, err
	}
	if pid.AltitudeD, err = r.U16(); err != nil {
		return nil, err
	}
	if pid.VelocityP, err = r.U16(); err != nil {
		return nil, err
	}
	if pid.VelocityI, err = r.U16(); err != nil {
		return nil, err
	}
	if pid.VelocityD, err = r.U16(); err != nil {
		return nil, err
	}
	if pid.YawP, err = r.U16(); err != nil {
		return nil, err
	}
	return pid, nil
}

func DecodeGPSSatellites(payload []byte) ([]GPSSatellite, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, err
	}
	satellites := make([]GPSSatellite, 0, count)
	for i := 0; i < int(count); i++ {
		channel, err := r.U8()
		if err != nil {
			return nil, err
		}
		svid, err := r.U8()
		if err != nil {
			return nil, err
		}
		quality, err := r.U8()
		if err != nil {
			return nil, err
		}
		cno, err := r.U8()
		if err != nil {
			return nil, err
		}
		satellites = append(satellites, GPSSatellite{Channel: channel, SVID: svid, Quality: quality, CNO: cno})
	}
	return satellites, nil
}

func readGPSPosition(ctx context.Context, client *connection.Client) (*GPSPosition, error) {
	frame, err := client.Request(ctx, msp.MSPRawGPS, nil)
	if err != nil {
		return nil, fmt.Errorf("gps position unavailable: %w", err)
	}
	return DecodeGPSPosition(frame.Payload)
}

func readGPSHome(ctx context.Context, client *connection.Client) (*GPSHome, error) {
	frame, err := client.Request(ctx, msp.MSPCompGPS, nil)
	if err != nil {
		return nil, fmt.Errorf("gps home unavailable: %w", err)
	}
	return DecodeGPSHome(frame.Payload)
}

func readGPSRescue(ctx context.Context, client *connection.Client) (*GPSRescue, error) {
	frame, err := client.Request(ctx, msp.MSPGPSRescue, nil)
	if err != nil {
		return nil, fmt.Errorf("gps rescue unavailable: %w", err)
	}
	return DecodeGPSRescue(frame.Payload)
}

func readGPSRescuePID(ctx context.Context, client *connection.Client) (*GPSRescuePID, error) {
	frame, err := client.Request(ctx, msp.MSPGPSRescuePids, nil)
	if err != nil {
		return nil, fmt.Errorf("gps rescue pids unavailable: %w", err)
	}
	return DecodeGPSRescuePID(frame.Payload)
}

func readGPSSatellites(ctx context.Context, client *connection.Client) ([]GPSSatellite, error) {
	frame, err := client.Request(ctx, msp.MSPGpssvinfo, nil)
	if err != nil {
		return nil, fmt.Errorf("gps satellite info unavailable: %w", err)
	}
	return DecodeGPSSatellites(frame.Payload)
}

func readGPSStatistics(ctx context.Context, client *connection.Client) (*GPSStatistics, error) {
	frame, err := client.Request(ctx, msp.MSPGpsstatistics, nil)
	if err != nil {
		return nil, fmt.Errorf("gps statistics unavailable: %w", err)
	}
	stats, err := DecodeGPSStatistics(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("gps statistics unavailable: %w", err)
	}
	return stats, nil
}

// DecodeGPSStatistics wraps a raw MSP_GPSSTATISTICS payload without
// interpreting fields; see GPSStatistics for why no layout is applied.
func DecodeGPSStatistics(payload []byte) (*GPSStatistics, error) {
	if len(payload) == 0 {
		return nil, fmt.Errorf("MSP_GPSSTATISTICS returned empty payload")
	}
	return &GPSStatistics{
		PayloadLen: len(payload),
		PayloadHex: hex.EncodeToString(payload),
	}, nil
}
