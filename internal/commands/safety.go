package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const (
	armingConfigMinLength      = 3
	armingConfigModernLength   = 4
	boardAlignmentConfigLength = 6
	failsafeConfigLength       = 8
)

var (
	failsafeSwitchModeNames = []string{"STAGE1", "KILL", "STAGE2"}
	failsafeProcedureNames  = []string{"AUTO-LAND", "DROP", "GPS-RESCUE"}
	armingDisableFlagNames  = []string{
		"NOGYRO",
		"FAILSAFE",
		"RXLOSS",
		"NOT_DISARMED",
		"BOXFAILSAFE",
		"RUNAWAY",
		"CRASH",
		"THROTTLE",
		"ANGLE",
		"BOOTGRACE",
		"NOPREARM",
		"LOAD",
		"CALIB",
		"CLI",
		"CMS",
		"BST",
		"MSP",
		"PARALYZE",
		"GPS",
		"RESCUE_SW",
		"DSHOT_TELEM",
		"REBOOT_REQD",
		"DSHOT_BBANG",
		"NO_ACC_CAL",
		"MOTOR_PROTO",
		"FLIP_SWITCH",
		"ALT_HOLD_SW",
		"POS_HOLD_SW",
		"ARM_SWITCH",
	}
)

type SafetyStatus struct {
	ArmingConfig *ArmingConfig       `json:"arming_config,omitempty"`
	Failsafe     *FailsafeConfig     `json:"failsafe_config,omitempty"`
	Board        *BoardAlignment     `json:"board_alignment,omitempty"`
	Arming       *ArmingDisableState `json:"arming,omitempty"`
	Sources      map[string]string   `json:"sources,omitempty"`
}

type ArmingConfig struct {
	AutoDisarmDelayS     uint8 `json:"auto_disarm_delay_s"`
	SmallAngleDegrees    uint8 `json:"small_angle_degrees"`
	GyroCalOnFirstArm    *bool `json:"gyro_cal_on_first_arm,omitempty"`
	TrailingBytesIgnored int   `json:"trailing_bytes_ignored,omitempty"`
}

type ArmingConfigSetResult struct {
	Config       ArmingConfig `json:"config"`
	MSPCode      uint16       `json:"msp_code"`
	MSPName      string       `json:"msp_name"`
	Acknowledged bool         `json:"acknowledged"`
	SaveRequired bool         `json:"save_required"`
}

type FailsafeConfig struct {
	DelayTenthsS            uint8  `json:"delay_tenths_s"`
	LandingTimeS            uint8  `json:"landing_time_s"`
	Throttle                uint16 `json:"throttle"`
	SwitchMode              uint8  `json:"switch_mode"`
	SwitchModeName          string `json:"switch_mode_name,omitempty"`
	ThrottleLowDelayTenthsS uint16 `json:"throttle_low_delay_tenths_s"`
	Procedure               uint8  `json:"procedure"`
	ProcedureName           string `json:"procedure_name,omitempty"`
	TrailingBytesIgnored    int    `json:"trailing_bytes_ignored,omitempty"`
}

type FailsafeConfigSetResult struct {
	Config       FailsafeConfig `json:"config"`
	MSPCode      uint16         `json:"msp_code"`
	MSPName      string         `json:"msp_name"`
	Acknowledged bool           `json:"acknowledged"`
	SaveRequired bool           `json:"save_required"`
}

type BoardAlignment struct {
	RollDegrees          int16 `json:"roll_degrees"`
	PitchDegrees         int16 `json:"pitch_degrees"`
	YawDegrees           int16 `json:"yaw_degrees"`
	TrailingBytesIgnored int   `json:"trailing_bytes_ignored,omitempty"`
}

type BoardAlignmentSetResult struct {
	Alignment    BoardAlignment `json:"board_alignment"`
	MSPCode      uint16         `json:"msp_code"`
	MSPName      string         `json:"msp_name"`
	Acknowledged bool           `json:"acknowledged"`
	SaveRequired bool           `json:"save_required"`
}

type ArmingDisableState struct {
	Source          string              `json:"source"`
	Disabled        bool                `json:"disabled"`
	FlagCount       uint8               `json:"flag_count"`
	Flags           uint32              `json:"flags"`
	ActiveNames     []string            `json:"active_names"`
	ActiveFlags     []ArmingDisableFlag `json:"active_flags"`
	FlagCatalog     []ArmingDisableFlag `json:"flag_catalog"`
	UnknownMask     uint32              `json:"unknown_mask,omitempty"`
	ConfigStateFlag *uint8              `json:"config_state_flag,omitempty"`
}

type ArmingDisableFlag struct {
	Bit    uint8  `json:"bit"`
	Mask   uint32 `json:"mask"`
	Name   string `json:"name"`
	Active bool   `json:"active"`
	Known  bool   `json:"known"`
}

func ReadSafetyStatus(ctx context.Context, client *connection.Client) (*SafetyStatus, []string, error) {
	status := &SafetyStatus{Sources: map[string]string{}}
	warnings := []string{}

	if armingConfig, err := readArmingConfig(ctx, client); err == nil {
		status.ArmingConfig = armingConfig
		status.Sources["arming_config"] = "MSP_ARMING_CONFIG"
	} else {
		warnings = append(warnings, err.Error())
	}
	if failsafe, err := readFailsafeConfig(ctx, client); err == nil {
		status.Failsafe = failsafe
		status.Sources["failsafe"] = "MSP_FAILSAFE_CONFIG"
	} else {
		warnings = append(warnings, err.Error())
	}
	if board, err := readBoardAlignment(ctx, client); err == nil {
		status.Board = board
		status.Sources["board_alignment"] = "MSP_BOARD_ALIGNMENT_CONFIG"
	} else {
		warnings = append(warnings, err.Error())
	}
	if flightStatus, err := readStatus(ctx, client); err == nil {
		arming := DecodeArmingDisableState(flightStatus)
		status.Arming = arming
		status.Sources["arming"] = flightStatus.Source
	} else {
		warnings = append(warnings, err.Error())
	}
	if status.ArmingConfig == nil && status.Failsafe == nil && status.Board == nil && status.Arming == nil {
		return nil, warnings, fmt.Errorf("safety status unavailable")
	}
	return status, warnings, nil
}

func DecodeArmingConfig(payload []byte) (*ArmingConfig, error) {
	if len(payload) < armingConfigMinLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_ARMING_CONFIG minimum size %d", len(payload), armingConfigMinLength)
	}
	r := msp.NewPayloadReader(payload)
	autoDisarmDelay, err := r.U8()
	if err != nil {
		return nil, err
	}
	if _, err := r.U8(); err != nil {
		return nil, err
	}
	smallAngle, err := r.U8()
	if err != nil {
		return nil, err
	}
	var gyroCal *bool
	if len(payload) >= armingConfigModernLength {
		raw, err := r.U8()
		if err != nil {
			return nil, err
		}
		value := raw != 0
		gyroCal = &value
	}
	return &ArmingConfig{
		AutoDisarmDelayS:     autoDisarmDelay,
		SmallAngleDegrees:    smallAngle,
		GyroCalOnFirstArm:    gyroCal,
		TrailingBytesIgnored: r.Remaining(),
	}, nil
}

func DecodeFailsafeConfig(payload []byte) (*FailsafeConfig, error) {
	if len(payload) < failsafeConfigLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_FAILSAFE_CONFIG size %d", len(payload), failsafeConfigLength)
	}
	r := msp.NewPayloadReader(payload)
	delay, err := r.U8()
	if err != nil {
		return nil, err
	}
	landing, err := r.U8()
	if err != nil {
		return nil, err
	}
	throttle, err := r.U16()
	if err != nil {
		return nil, err
	}
	switchMode, err := r.U8()
	if err != nil {
		return nil, err
	}
	throttleLowDelay, err := r.U16()
	if err != nil {
		return nil, err
	}
	procedure, err := r.U8()
	if err != nil {
		return nil, err
	}
	return &FailsafeConfig{
		DelayTenthsS:            delay,
		LandingTimeS:            landing,
		Throttle:                throttle,
		SwitchMode:              switchMode,
		SwitchModeName:          indexedName(failsafeSwitchModeNames, switchMode),
		ThrottleLowDelayTenthsS: throttleLowDelay,
		Procedure:               procedure,
		ProcedureName:           indexedName(failsafeProcedureNames, procedure),
		TrailingBytesIgnored:    r.Remaining(),
	}, nil
}

func DecodeBoardAlignment(payload []byte) (*BoardAlignment, error) {
	if len(payload) < boardAlignmentConfigLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_BOARD_ALIGNMENT_CONFIG size %d", len(payload), boardAlignmentConfigLength)
	}
	r := msp.NewPayloadReader(payload)
	roll, err := r.S16()
	if err != nil {
		return nil, err
	}
	pitch, err := r.S16()
	if err != nil {
		return nil, err
	}
	yaw, err := r.S16()
	if err != nil {
		return nil, err
	}
	return &BoardAlignment{
		RollDegrees:          roll,
		PitchDegrees:         pitch,
		YawDegrees:           yaw,
		TrailingBytesIgnored: r.Remaining(),
	}, nil
}

func SetArmingConfig(ctx context.Context, client *connection.Client, config ArmingConfig) (*ArmingConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetArmingConfig, EncodeArmingConfig(config)); err != nil {
		return nil, fmt.Errorf("arming config request failed: %w", err)
	}
	return &ArmingConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetArmingConfig,
		MSPName:      "MSP_SET_ARMING_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeArmingConfig(config ArmingConfig) []byte {
	gyroCal := uint8(0)
	if config.GyroCalOnFirstArm != nil && *config.GyroCalOnFirstArm {
		gyroCal = 1
	}
	return []byte{config.AutoDisarmDelayS, 0, config.SmallAngleDegrees, gyroCal}
}

func SetFailsafeConfig(ctx context.Context, client *connection.Client, config FailsafeConfig) (*FailsafeConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetFailsafeConfig, EncodeFailsafeConfig(config)); err != nil {
		return nil, fmt.Errorf("failsafe config request failed: %w", err)
	}
	return &FailsafeConfigSetResult{
		Config:       config,
		MSPCode:      msp.MSPSetFailsafeConfig,
		MSPName:      "MSP_SET_FAILSAFE_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeFailsafeConfig(config FailsafeConfig) []byte {
	payload := []byte{config.DelayTenthsS, config.LandingTimeS}
	payload = appendU16Payload(payload, config.Throttle)
	payload = append(payload, config.SwitchMode)
	payload = appendU16Payload(payload, config.ThrottleLowDelayTenthsS)
	payload = append(payload, config.Procedure)
	return payload
}

func SetBoardAlignment(ctx context.Context, client *connection.Client, alignment BoardAlignment) (*BoardAlignmentSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetBoardAlignmentConfig, EncodeBoardAlignment(alignment)); err != nil {
		return nil, fmt.Errorf("board alignment request failed: %w", err)
	}
	return &BoardAlignmentSetResult{
		Alignment:    alignment,
		MSPCode:      msp.MSPSetBoardAlignmentConfig,
		MSPName:      "MSP_SET_BOARD_ALIGNMENT_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeBoardAlignment(alignment BoardAlignment) []byte {
	payload := make([]byte, 0, boardAlignmentConfigLength)
	payload = append(payload, byte(alignment.RollDegrees), byte(uint16(alignment.RollDegrees)>>8))
	payload = append(payload, byte(alignment.PitchDegrees), byte(uint16(alignment.PitchDegrees)>>8))
	payload = append(payload, byte(alignment.YawDegrees), byte(uint16(alignment.YawDegrees)>>8))
	return payload
}

func DecodeArmingDisableState(status *Status) *ArmingDisableState {
	if status == nil || status.ArmingDisableCount == nil || status.ArmingDisableFlags == nil {
		return nil
	}
	count := *status.ArmingDisableCount
	flags := *status.ArmingDisableFlags
	out := &ArmingDisableState{
		Source:          status.Source,
		Disabled:        flags != 0,
		FlagCount:       count,
		Flags:           flags,
		ActiveNames:     []string{},
		ActiveFlags:     []ArmingDisableFlag{},
		FlagCatalog:     []ArmingDisableFlag{},
		ConfigStateFlag: status.ConfigStateFlag,
	}
	for bit := uint8(0); bit < count && bit < 32; bit++ {
		mask := uint32(1) << bit
		name := armingDisableFlagName(bit)
		known := int(bit) < len(armingDisableFlagNames)
		active := flags&mask != 0
		flag := ArmingDisableFlag{
			Bit:    bit,
			Mask:   mask,
			Name:   name,
			Active: active,
			Known:  known,
		}
		out.FlagCatalog = append(out.FlagCatalog, flag)
		if active {
			out.ActiveFlags = append(out.ActiveFlags, flag)
			out.ActiveNames = append(out.ActiveNames, name)
		}
	}
	out.UnknownMask = flags &^ namedArmingFlagMask(count)
	return out
}

func readArmingConfig(ctx context.Context, client *connection.Client) (*ArmingConfig, error) {
	frame, err := client.Request(ctx, msp.MSPArmingConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("arming config unavailable: %w", err)
	}
	return DecodeArmingConfig(frame.Payload)
}

func readFailsafeConfig(ctx context.Context, client *connection.Client) (*FailsafeConfig, error) {
	frame, err := client.Request(ctx, msp.MSPFailsafeConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("failsafe config unavailable: %w", err)
	}
	return DecodeFailsafeConfig(frame.Payload)
}

func readBoardAlignment(ctx context.Context, client *connection.Client) (*BoardAlignment, error) {
	frame, err := client.Request(ctx, msp.MSPBoardAlignmentConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("board alignment unavailable: %w", err)
	}
	return DecodeBoardAlignment(frame.Payload)
}

func armingDisableFlagName(bit uint8) string {
	if int(bit) < len(armingDisableFlagNames) {
		return armingDisableFlagNames[bit]
	}
	return fmt.Sprintf("UNKNOWN_%d", bit)
}

func namedArmingFlagMask(count uint8) uint32 {
	namedCount := count
	if int(namedCount) > len(armingDisableFlagNames) {
		namedCount = uint8(len(armingDisableFlagNames))
	}
	if namedCount >= 32 {
		return ^uint32(0)
	}
	if namedCount == 0 {
		return 0
	}
	return (uint32(1) << namedCount) - 1
}
