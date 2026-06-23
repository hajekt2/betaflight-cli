package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const (
	rcTuningModernLength    = 24
	pidAdvancedMinLength    = 61
	pidTripletLength        = 3
	pidControllerBetaflight = 0
)

var (
	defaultPIDNames        = []string{"ROLL", "PITCH", "YAW", "LEVEL", "MAG"}
	ratesTypeNames         = []string{"BETAFLIGHT", "RACEFLIGHT", "KISS", "ACTUAL", "QUICK"}
	throttleLimitTypeNames = []string{"OFF", "SCALE", "CLIP"}
)

func DefaultPIDNamesForCLI() []string {
	return append([]string(nil), defaultPIDNames...)
}

type PIDStatus struct {
	Controller  *PIDController    `json:"controller,omitempty"`
	Names       []string          `json:"names,omitempty"`
	Gains       []PIDGain         `json:"gains,omitempty"`
	RateProfile *RateProfile      `json:"rate_profile,omitempty"`
	Advanced    *PIDAdvanced      `json:"advanced,omitempty"`
	Sources     map[string]string `json:"sources,omitempty"`
}

type RateStatus struct {
	RateProfile *RateProfile      `json:"rate_profile,omitempty"`
	TPA         *TPAConfig        `json:"tpa,omitempty"`
	Sources     map[string]string `json:"sources,omitempty"`
}

type PIDController struct {
	ID   uint8  `json:"id"`
	Name string `json:"name,omitempty"`
}

type PIDGain struct {
	Index int    `json:"index"`
	Name  string `json:"name,omitempty"`
	P     uint8  `json:"p"`
	I     uint8  `json:"i"`
	D     uint8  `json:"d"`
}

type PIDGainsSetResult struct {
	Gains        []PIDGain `json:"gains"`
	MSPCode      uint16    `json:"msp_code"`
	MSPName      string    `json:"msp_name"`
	Acknowledged bool      `json:"acknowledged"`
	SaveRequired bool      `json:"save_required"`
}

type RateProfile struct {
	Axes                 []RateAxis `json:"axes"`
	Throttle             Throttle   `json:"throttle"`
	RatesType            uint8      `json:"rates_type"`
	RatesTypeName        string     `json:"rates_type_name,omitempty"`
	TrailingBytesIgnored int        `json:"trailing_bytes_ignored,omitempty"`
}

type RateProfileSetResult struct {
	RateProfile  RateProfile `json:"rate_profile"`
	MSPCode      uint16      `json:"msp_code"`
	MSPName      string      `json:"msp_name"`
	Acknowledged bool        `json:"acknowledged"`
	SaveRequired bool        `json:"save_required"`
}

type RateAxis struct {
	Axis         string  `json:"axis"`
	RCRate       uint8   `json:"rc_rate"`
	RCRateValue  float64 `json:"rc_rate_value"`
	Expo         uint8   `json:"expo"`
	ExpoValue    float64 `json:"expo_value"`
	Rate         uint8   `json:"rate"`
	RateValue    float64 `json:"rate_value"`
	RateLimitDPS uint16  `json:"rate_limit_dps"`
}

type Throttle struct {
	MidPercent    uint8   `json:"mid_percent"`
	MidValue      float64 `json:"mid_value"`
	ExpoPercent   uint8   `json:"expo_percent"`
	ExpoValue     float64 `json:"expo_value"`
	HoverPercent  uint8   `json:"hover_percent"`
	HoverValue    float64 `json:"hover_value"`
	LimitType     uint8   `json:"limit_type"`
	LimitTypeName string  `json:"limit_type_name,omitempty"`
	LimitPercent  uint8   `json:"limit_percent"`
}

type TPAConfig struct {
	Mode       uint8   `json:"mode"`
	Rate       uint8   `json:"rate"`
	RateValue  float64 `json:"rate_value"`
	Breakpoint uint16  `json:"breakpoint"`
}

type PIDAdvanced struct {
	RollPitchItermIgnoreRate uint16    `json:"roll_pitch_iterm_ignore_rate"`
	YawItermIgnoreRate       uint16    `json:"yaw_iterm_ignore_rate"`
	YawPLimit                uint16    `json:"yaw_p_limit"`
	DeltaMethod              uint8     `json:"delta_method"`
	VBATPIDCompensation      uint8     `json:"vbat_pid_compensation"`
	FeedforwardTransition    uint8     `json:"feedforward_transition"`
	DtermSetpointWeightLow   uint8     `json:"dterm_setpoint_weight_low"`
	ToleranceBand            uint8     `json:"tolerance_band"`
	ToleranceBandReduction   uint8     `json:"tolerance_band_reduction"`
	ItermThrottleGain        uint8     `json:"iterm_throttle_gain"`
	RateAccelLimit           uint16    `json:"rate_accel_limit"`
	YawRateAccelLimit        uint16    `json:"yaw_rate_accel_limit"`
	LevelAngleLimit          uint8     `json:"level_angle_limit"`
	LevelSensitivity         uint8     `json:"level_sensitivity"`
	ItermThrottleThreshold   uint16    `json:"iterm_throttle_threshold"`
	AntiGravityGain          uint16    `json:"anti_gravity_gain"`
	DtermSetpointWeight      uint16    `json:"dterm_setpoint_weight"`
	ItermRotation            uint8     `json:"iterm_rotation"`
	SmartFeedforward         uint8     `json:"smart_feedforward"`
	ItermRelax               uint8     `json:"iterm_relax"`
	ItermRelaxType           uint8     `json:"iterm_relax_type"`
	AbsoluteControlGain      uint8     `json:"absolute_control_gain"`
	ThrottleBoost            uint8     `json:"throttle_boost"`
	AcroTrainerAngleLimit    uint8     `json:"acro_trainer_angle_limit"`
	FeedforwardRoll          uint16    `json:"feedforward_roll"`
	FeedforwardPitch         uint16    `json:"feedforward_pitch"`
	FeedforwardYaw           uint16    `json:"feedforward_yaw"`
	AntiGravityMode          uint8     `json:"anti_gravity_mode"`
	DMaxRoll                 uint8     `json:"d_max_roll"`
	DMaxPitch                uint8     `json:"d_max_pitch"`
	DMaxYaw                  uint8     `json:"d_max_yaw"`
	DMaxGain                 uint8     `json:"d_max_gain"`
	DMaxAdvance              uint8     `json:"d_max_advance"`
	UseIntegratedYaw         uint8     `json:"use_integrated_yaw"`
	IntegratedYawRelax       uint8     `json:"integrated_yaw_relax"`
	ItermRelaxCutoff         uint8     `json:"iterm_relax_cutoff"`
	MotorOutputLimit         uint8     `json:"motor_output_limit"`
	AutoProfileCellCount     int8      `json:"auto_profile_cell_count"`
	IdleMinRPM               uint8     `json:"idle_min_rpm"`
	FeedforwardAveraging     uint8     `json:"feedforward_averaging"`
	FeedforwardSmoothFactor  uint8     `json:"feedforward_smooth_factor"`
	FeedforwardBoost         uint8     `json:"feedforward_boost"`
	FeedforwardMaxRateLimit  uint8     `json:"feedforward_max_rate_limit"`
	FeedforwardJitterFactor  uint8     `json:"feedforward_jitter_factor"`
	VBATSagCompensation      uint8     `json:"vbat_sag_compensation"`
	ThrustLinearization      uint8     `json:"thrust_linearization"`
	TPA                      TPAConfig `json:"tpa"`
	TrailingBytesIgnored     int       `json:"trailing_bytes_ignored,omitempty"`
}

func ReadPIDStatus(ctx context.Context, client *connection.Client) (*PIDStatus, []string, error) {
	status := &PIDStatus{Sources: map[string]string{}}
	warnings := []string{}

	frame, err := client.Request(ctx, msp.MSPPID, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("pid gains unavailable: %w", err)
	}
	names := defaultPIDNames
	if pidNames, err := readPIDNames(ctx, client); err == nil {
		names = pidNames
		status.Names = pidNames
		status.Sources["names"] = "MSP_PIDNAMES"
	} else {
		status.Names = defaultPIDNames
		warnings = append(warnings, err.Error())
	}
	gains, err := DecodePIDGains(frame.Payload, names)
	if err != nil {
		return nil, warnings, fmt.Errorf("pid gains decode failed: %w", err)
	}
	status.Gains = gains
	status.Sources["gains"] = "MSP_PID"

	if controller, err := readPIDController(ctx, client); err == nil {
		status.Controller = controller
		status.Sources["controller"] = "MSP_PID_CONTROLLER"
	} else {
		warnings = append(warnings, err.Error())
	}
	if rateProfile, err := readRateProfile(ctx, client); err == nil {
		status.RateProfile = rateProfile
		status.Sources["rate_profile"] = "MSP_RC_TUNING"
	} else {
		warnings = append(warnings, err.Error())
	}
	if advanced, err := readPIDAdvanced(ctx, client); err == nil {
		status.Advanced = advanced
		status.Sources["advanced"] = "MSP_PID_ADVANCED"
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func ReadRateStatus(ctx context.Context, client *connection.Client) (*RateStatus, []string, error) {
	status := &RateStatus{Sources: map[string]string{}}
	warnings := []string{}

	rateProfile, err := readRateProfile(ctx, client)
	if err != nil {
		return nil, nil, err
	}
	status.RateProfile = rateProfile
	status.Sources["rate_profile"] = "MSP_RC_TUNING"

	if advanced, err := readPIDAdvanced(ctx, client); err == nil {
		status.TPA = &advanced.TPA
		status.Sources["tpa"] = "MSP_PID_ADVANCED"
	} else {
		warnings = append(warnings, err.Error())
	}
	return status, warnings, nil
}

func SetPIDGains(ctx context.Context, client *connection.Client, gains []PIDGain) (*PIDGainsSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetPID, EncodePIDGains(gains)); err != nil {
		return nil, fmt.Errorf("pid gains request failed: %w", err)
	}
	return &PIDGainsSetResult{
		Gains:        gains,
		MSPCode:      msp.MSPSetPID,
		MSPName:      "MSP_SET_PID",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func SetRateProfile(ctx context.Context, client *connection.Client, profile RateProfile) (*RateProfileSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetRCTuning, EncodeRateProfile(profile)); err != nil {
		return nil, fmt.Errorf("rate profile request failed: %w", err)
	}
	return &RateProfileSetResult{
		RateProfile:  profile,
		MSPCode:      msp.MSPSetRCTuning,
		MSPName:      "MSP_SET_RC_TUNING",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeRateProfile(profile RateProfile) []byte {
	roll := rateAxisByName(profile.Axes, "roll")
	pitch := rateAxisByName(profile.Axes, "pitch")
	yaw := rateAxisByName(profile.Axes, "yaw")
	payload := []byte{
		roll.RCRate,
		roll.Expo,
		roll.Rate,
		pitch.Rate,
		yaw.Rate,
		0,
		profile.Throttle.MidPercent,
		profile.Throttle.ExpoPercent,
	}
	payload = appendU16Payload(payload, 0)
	payload = append(payload,
		yaw.Expo,
		yaw.RCRate,
		pitch.RCRate,
		pitch.Expo,
		profile.Throttle.LimitType,
		profile.Throttle.LimitPercent,
	)
	payload = appendU16Payload(payload, roll.RateLimitDPS)
	payload = appendU16Payload(payload, pitch.RateLimitDPS)
	payload = appendU16Payload(payload, yaw.RateLimitDPS)
	payload = append(payload, profile.RatesType, profile.Throttle.HoverPercent)
	return payload
}

func rateAxisByName(axes []RateAxis, name string) RateAxis {
	for _, axis := range axes {
		if strings.EqualFold(axis.Axis, name) {
			return axis
		}
	}
	return RateAxis{Axis: name}
}

func EncodePIDGains(gains []PIDGain) []byte {
	payload := make([]byte, 0, len(gains)*pidTripletLength)
	for _, gain := range gains {
		payload = append(payload, gain.P, gain.I, gain.D)
	}
	return payload
}

func DecodePIDGains(payload []byte, names []string) ([]PIDGain, error) {
	if len(payload)%pidTripletLength != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of PID triplet size 3", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	gains := make([]PIDGain, 0, len(payload)/pidTripletLength)
	for i := 0; r.Remaining() > 0; i++ {
		p, err := r.U8()
		if err != nil {
			return nil, err
		}
		ii, err := r.U8()
		if err != nil {
			return nil, err
		}
		d, err := r.U8()
		if err != nil {
			return nil, err
		}
		gains = append(gains, PIDGain{Index: i, Name: pidNameAt(names, i), P: p, I: ii, D: d})
	}
	return gains, nil
}

func DecodePIDNames(payload []byte) []string {
	parts := strings.Split(string(payload), ";")
	names := []string{}
	for _, part := range parts {
		if part != "" {
			names = append(names, part)
		}
	}
	return names
}

func DecodePIDController(payload []byte) (*PIDController, error) {
	r := msp.NewPayloadReader(payload)
	id, err := r.U8()
	if err != nil {
		return nil, err
	}
	name := ""
	if id == pidControllerBetaflight {
		name = "BETAFLIGHT"
	}
	return &PIDController{ID: id, Name: name}, nil
}

func DecodeRateProfile(payload []byte) (*RateProfile, error) {
	if len(payload) < rcTuningModernLength {
		return nil, fmt.Errorf("payload length %d is shorter than modern MSP_RC_TUNING size %d", len(payload), rcTuningModernLength)
	}
	r := msp.NewPayloadReader(payload)
	rollRcRate, err := r.U8()
	if err != nil {
		return nil, err
	}
	rollExpo, err := r.U8()
	if err != nil {
		return nil, err
	}
	rollRate, err := r.U8()
	if err != nil {
		return nil, err
	}
	pitchRate, err := r.U8()
	if err != nil {
		return nil, err
	}
	yawRate, err := r.U8()
	if err != nil {
		return nil, err
	}
	if _, err := r.U8(); err != nil {
		return nil, err
	}
	thrMid, err := r.U8()
	if err != nil {
		return nil, err
	}
	thrExpo, err := r.U8()
	if err != nil {
		return nil, err
	}
	if _, err := r.U16(); err != nil {
		return nil, err
	}
	yawExpo, err := r.U8()
	if err != nil {
		return nil, err
	}
	yawRcRate, err := r.U8()
	if err != nil {
		return nil, err
	}
	pitchRcRate, err := r.U8()
	if err != nil {
		return nil, err
	}
	pitchExpo, err := r.U8()
	if err != nil {
		return nil, err
	}
	throttleLimitType, err := r.U8()
	if err != nil {
		return nil, err
	}
	throttleLimitPercent, err := r.U8()
	if err != nil {
		return nil, err
	}
	rollRateLimit, err := r.U16()
	if err != nil {
		return nil, err
	}
	pitchRateLimit, err := r.U16()
	if err != nil {
		return nil, err
	}
	yawRateLimit, err := r.U16()
	if err != nil {
		return nil, err
	}
	ratesType, err := r.U8()
	if err != nil {
		return nil, err
	}
	thrHover, err := r.U8()
	if err != nil {
		return nil, err
	}
	return &RateProfile{
		Axes: []RateAxis{
			rateAxis("roll", rollRcRate, rollExpo, rollRate, rollRateLimit),
			rateAxis("pitch", pitchRcRate, pitchExpo, pitchRate, pitchRateLimit),
			rateAxis("yaw", yawRcRate, yawExpo, yawRate, yawRateLimit),
		},
		Throttle: Throttle{
			MidPercent:    thrMid,
			MidValue:      percentValue(thrMid),
			ExpoPercent:   thrExpo,
			ExpoValue:     percentValue(thrExpo),
			HoverPercent:  thrHover,
			HoverValue:    percentValue(thrHover),
			LimitType:     throttleLimitType,
			LimitTypeName: indexedName(throttleLimitTypeNames, throttleLimitType),
			LimitPercent:  throttleLimitPercent,
		},
		RatesType:            ratesType,
		RatesTypeName:        indexedName(ratesTypeNames, ratesType),
		TrailingBytesIgnored: r.Remaining(),
	}, nil
}

func DecodePIDAdvanced(payload []byte) (*PIDAdvanced, error) {
	if len(payload) < pidAdvancedMinLength {
		return nil, fmt.Errorf("payload length %d is shorter than modern MSP_PID_ADVANCED size %d", len(payload), pidAdvancedMinLength)
	}
	r := msp.NewPayloadReader(payload)
	advanced := &PIDAdvanced{}
	var err error
	if advanced.RollPitchItermIgnoreRate, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.YawItermIgnoreRate, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.YawPLimit, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.DeltaMethod, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.VBATPIDCompensation, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardTransition, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.DtermSetpointWeightLow, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ToleranceBand, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ToleranceBandReduction, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ItermThrottleGain, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.RateAccelLimit, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.YawRateAccelLimit, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.LevelAngleLimit, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.LevelSensitivity, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ItermThrottleThreshold, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.AntiGravityGain, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.DtermSetpointWeight, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.ItermRotation, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.SmartFeedforward, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ItermRelax, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ItermRelaxType, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.AbsoluteControlGain, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ThrottleBoost, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.AcroTrainerAngleLimit, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardRoll, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardPitch, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardYaw, err = r.U16(); err != nil {
		return nil, err
	}
	if advanced.AntiGravityMode, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.DMaxRoll, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.DMaxPitch, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.DMaxYaw, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.DMaxGain, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.DMaxAdvance, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.UseIntegratedYaw, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.IntegratedYawRelax, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ItermRelaxCutoff, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.MotorOutputLimit, err = r.U8(); err != nil {
		return nil, err
	}
	autoProfileCellCount, err := r.U8()
	if err != nil {
		return nil, err
	}
	advanced.AutoProfileCellCount = int8(autoProfileCellCount)
	if advanced.IdleMinRPM, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardAveraging, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardSmoothFactor, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardBoost, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardMaxRateLimit, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.FeedforwardJitterFactor, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.VBATSagCompensation, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.ThrustLinearization, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.TPA.Mode, err = r.U8(); err != nil {
		return nil, err
	}
	if advanced.TPA.Rate, err = r.U8(); err != nil {
		return nil, err
	}
	advanced.TPA.RateValue = percentValue(advanced.TPA.Rate)
	if advanced.TPA.Breakpoint, err = r.U16(); err != nil {
		return nil, err
	}
	advanced.TrailingBytesIgnored = r.Remaining()
	return advanced, nil
}

func readPIDNames(ctx context.Context, client *connection.Client) ([]string, error) {
	frame, err := client.Request(ctx, msp.MSPPidnames, nil)
	if err != nil {
		return nil, fmt.Errorf("pid names unavailable: %w", err)
	}
	return DecodePIDNames(frame.Payload), nil
}

func readPIDController(ctx context.Context, client *connection.Client) (*PIDController, error) {
	frame, err := client.Request(ctx, msp.MSPPIDController, nil)
	if err != nil {
		return nil, fmt.Errorf("pid controller unavailable: %w", err)
	}
	controller, err := DecodePIDController(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("pid controller decode failed: %w", err)
	}
	return controller, nil
}

func readRateProfile(ctx context.Context, client *connection.Client) (*RateProfile, error) {
	frame, err := client.Request(ctx, msp.MSPRCTuning, nil)
	if err != nil {
		return nil, fmt.Errorf("rate profile unavailable: %w", err)
	}
	rateProfile, err := DecodeRateProfile(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("rate profile decode failed: %w", err)
	}
	return rateProfile, nil
}

func readPIDAdvanced(ctx context.Context, client *connection.Client) (*PIDAdvanced, error) {
	frame, err := client.Request(ctx, msp.MSPPIDAdvanced, nil)
	if err != nil {
		return nil, fmt.Errorf("advanced pid unavailable: %w", err)
	}
	advanced, err := DecodePIDAdvanced(frame.Payload)
	if err != nil {
		return nil, fmt.Errorf("advanced pid decode failed: %w", err)
	}
	return advanced, nil
}

func rateAxis(axis string, rcRate, expo, rate uint8, limit uint16) RateAxis {
	return RateAxis{
		Axis:         axis,
		RCRate:       rcRate,
		RCRateValue:  percentValue(rcRate),
		Expo:         expo,
		ExpoValue:    percentValue(expo),
		Rate:         rate,
		RateValue:    percentValue(rate),
		RateLimitDPS: limit,
	}
}

func percentValue(v uint8) float64 {
	return float64(v) / 100
}

func pidNameAt(names []string, index int) string {
	if index < len(names) {
		return names[index]
	}
	return ""
}

func indexedName(names []string, index uint8) string {
	if int(index) >= len(names) {
		return ""
	}
	return names[index]
}
