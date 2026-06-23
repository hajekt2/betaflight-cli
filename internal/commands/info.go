package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type Info struct {
	Variant         string     `json:"variant"`
	FirmwareVersion string     `json:"firmware_version"`
	MSPAPIVersion   string     `json:"msp_api_version"`
	MSPProtocol     uint8      `json:"msp_protocol_version"`
	Board           *BoardInfo `json:"board,omitempty"`
	MCU             *MCUInfo   `json:"mcu,omitempty"`
	Build           *BuildInfo `json:"build,omitempty"`
	LegacyName      string     `json:"legacy_name,omitempty"`
}

type BoardInfo struct {
	Identifier         string `json:"identifier,omitempty"`
	HardwareRevision   uint16 `json:"hardware_revision,omitempty"`
	BoardType          uint8  `json:"board_type,omitempty"`
	TargetCapabilities uint8  `json:"target_capabilities,omitempty"`
	TargetName         string `json:"target_name,omitempty"`
	BoardName          string `json:"board_name,omitempty"`
	ManufacturerID     string `json:"manufacturer_id,omitempty"`
}

type MCUInfo struct {
	Source string `json:"source"`
	ID     uint8  `json:"id"`
	Name   string `json:"name"`
}

type BuildInfo struct {
	Source           string        `json:"source"`
	Date             string        `json:"date"`
	Time             string        `json:"time"`
	DateTime         string        `json:"date_time"`
	GitRevision      string        `json:"git_revision"`
	BuildOptions     []BuildOption `json:"build_options,omitempty"`
	BuildOptionCodes []uint16      `json:"build_option_codes,omitempty"`
	BuildOptionNames []string      `json:"build_option_names,omitempty"`
	UnknownOptions   []BuildOption `json:"unknown_options,omitempty"`
}

type BuildOption struct {
	Code  uint16 `json:"code"`
	Name  string `json:"name,omitempty"`
	Group string `json:"group,omitempty"`
	Known bool   `json:"known"`
}

func ReadInfo(ctx context.Context, client *connection.Client) (Info, []string) {
	target := client.Target()
	info := Info{
		Variant:         target.Variant,
		FirmwareVersion: target.FirmwareVersion,
		MSPAPIVersion:   target.MSPAPIVersion,
		MSPProtocol:     target.MSPProtocol,
	}
	var warnings []string
	frame, err := client.Request(ctx, msp.MSPBoardInfo, nil)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("MSP_BOARD_INFO unavailable: %v", err))
	} else {
		board, err := DecodeBoardInfo(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_BOARD_INFO decode failed: %v", err))
		} else {
			info.Board = board
		}
	}
	if frame, err := client.Request(ctx, msp.MSPBuildInfo, nil); err == nil {
		build, err := DecodeBuildInfo(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_BUILD_INFO decode failed: %v", err))
		} else {
			info.Build = build
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_BUILD_INFO unavailable: %v", err))
	}
	if frame, err := client.Request(ctx, msp.MSP2McuInfo, nil); err == nil {
		mcu, err := DecodeMCUInfo(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP2_MCU_INFO decode failed: %v", err))
		} else {
			info.MCU = mcu
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP2_MCU_INFO unavailable: %v", err))
	}
	if frame, err := client.Request(ctx, msp.MSPName, nil); err == nil {
		info.LegacyName = DecodeName(frame.Payload)
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_NAME unavailable: %v", err))
	}
	return info, warnings
}

func DecodeBoardInfo(payload []byte) (*BoardInfo, error) {
	r := msp.NewPayloadReader(payload)
	id, err := r.Bytes(4)
	if err != nil {
		return nil, err
	}
	rev, err := r.U16()
	if err != nil {
		return nil, err
	}
	boardType, err := r.U8()
	if err != nil {
		return nil, err
	}
	caps, err := r.U8()
	if err != nil {
		return nil, err
	}
	info := &BoardInfo{
		Identifier:         string(id),
		HardwareRevision:   rev,
		BoardType:          boardType,
		TargetCapabilities: caps,
	}
	if r.Remaining() > 0 {
		if s, err := r.PString(); err == nil {
			info.TargetName = s
		}
	}
	if r.Remaining() > 0 {
		if s, err := r.PString(); err == nil {
			info.BoardName = s
		}
	}
	if r.Remaining() > 0 {
		if s, err := r.PString(); err == nil {
			info.ManufacturerID = s
		}
	}
	return info, nil
}

func DecodeMCUInfo(payload []byte) (*MCUInfo, error) {
	r := msp.NewPayloadReader(payload)
	id, err := r.U8()
	if err != nil {
		return nil, msp.RequireNoShort(err, "MCU type ID")
	}
	name, err := r.PString()
	if err != nil {
		return nil, msp.RequireNoShort(err, "MCU name")
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP2_MCU_INFO returned %d trailing byte(s)", r.Remaining())
	}
	return &MCUInfo{
		Source: "MSP2_MCU_INFO",
		ID:     id,
		Name:   name,
	}, nil
}

func DecodeBuildInfo(payload []byte) (*BuildInfo, error) {
	const (
		buildDateLength   = 11
		buildTimeLength   = 8
		gitRevisionLength = 7
		minLength         = buildDateLength + buildTimeLength + gitRevisionLength
	)
	if len(payload) < minLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_BUILD_INFO size %d", len(payload), minLength)
	}
	r := msp.NewPayloadReader(payload)
	dateBytes, err := r.Bytes(buildDateLength)
	if err != nil {
		return nil, err
	}
	timeBytes, err := r.Bytes(buildTimeLength)
	if err != nil {
		return nil, err
	}
	revBytes, err := r.Bytes(gitRevisionLength)
	if err != nil {
		return nil, err
	}
	date := strings.TrimSpace(string(dateBytes))
	buildTime := strings.TrimSpace(string(timeBytes))
	info := &BuildInfo{
		Source:      "MSP_BUILD_INFO",
		Date:        date,
		Time:        buildTime,
		DateTime:    strings.TrimSpace(date + " " + buildTime),
		GitRevision: strings.TrimSpace(string(revBytes)),
	}
	for r.Remaining() > 0 {
		code, err := r.U16()
		if err != nil {
			return nil, msp.RequireNoShort(err, "build option")
		}
		option := decodeBuildOption(code)
		info.BuildOptions = append(info.BuildOptions, option)
		info.BuildOptionCodes = append(info.BuildOptionCodes, code)
		if option.Known {
			info.BuildOptionNames = append(info.BuildOptionNames, option.Name)
		} else {
			info.UnknownOptions = append(info.UnknownOptions, option)
		}
	}
	return info, nil
}

func DecodeName(payload []byte) string {
	return strings.TrimRight(string(payload), "\x00")
}

func decodeBuildOption(code uint16) BuildOption {
	if option, ok := buildOptionDefinitions[code]; ok {
		return option
	}
	return BuildOption{Code: code, Known: false}
}

var buildOptionDefinitions = map[uint16]BuildOption{
	4097:  {Code: 4097, Name: "USE_SERIALRX_CRSF", Group: "receiver", Known: true},
	4098:  {Code: 4098, Name: "USE_SERIALRX_FPORT", Group: "receiver", Known: true},
	4099:  {Code: 4099, Name: "USE_SERIALRX_GHST", Group: "receiver", Known: true},
	4100:  {Code: 4100, Name: "USE_SERIALRX_IBUS", Group: "receiver", Known: true},
	4101:  {Code: 4101, Name: "USE_SERIALRX_JETIEXBUS", Group: "receiver", Known: true},
	4102:  {Code: 4102, Name: "USE_RX_PPM", Group: "receiver", Known: true},
	4103:  {Code: 4103, Name: "USE_SERIALRX_SBUS", Group: "receiver", Known: true},
	4104:  {Code: 4104, Name: "USE_SERIALRX_SPEKTRUM", Group: "receiver", Known: true},
	4105:  {Code: 4105, Name: "USE_SERIALRX_SRXL2", Group: "receiver", Known: true},
	4106:  {Code: 4106, Name: "USE_SERIALRX_SUMD", Group: "receiver", Known: true},
	4107:  {Code: 4107, Name: "USE_SERIALRX_SUMH", Group: "receiver", Known: true},
	4108:  {Code: 4108, Name: "USE_SERIALRX_XBUS", Group: "receiver", Known: true},
	4109:  {Code: 4109, Name: "USE_SERIALRX_MAVLINK", Group: "receiver", Known: true},
	8230:  {Code: 8230, Name: "USE_BRUSHED", Group: "motor", Known: true},
	8231:  {Code: 8231, Name: "USE_DSHOT", Group: "motor", Known: true},
	8232:  {Code: 8232, Name: "USE_MULTISHOT", Group: "motor", Known: true},
	8233:  {Code: 8233, Name: "USE_ONESHOT", Group: "motor", Known: true},
	8234:  {Code: 8234, Name: "USE_PROSHOT", Group: "motor", Known: true},
	8235:  {Code: 8235, Name: "USE_PWM_OUTPUT", Group: "motor", Known: true},
	12301: {Code: 12301, Name: "USE_TELEMETRY_FRSKY_HUB", Group: "telemetry", Known: true},
	12302: {Code: 12302, Name: "USE_TELEMETRY_HOTT", Group: "telemetry", Known: true},
	12303: {Code: 12303, Name: "USE_TELEMETRY_IBUS_EXTENDED", Group: "telemetry", Known: true},
	12304: {Code: 12304, Name: "USE_TELEMETRY_LTM", Group: "telemetry", Known: true},
	12305: {Code: 12305, Name: "USE_TELEMETRY_MAVLINK", Group: "telemetry", Known: true},
	12306: {Code: 12306, Name: "USE_TELEMETRY_SMARTPORT", Group: "telemetry", Known: true},
	12307: {Code: 12307, Name: "USE_TELEMETRY_SRXL", Group: "telemetry", Known: true},
	16404: {Code: 16404, Name: "USE_ACRO_TRAINER", Group: "general", Known: true},
	16405: {Code: 16405, Name: "USE_AKK_SMARTAUDIO", Group: "general", Known: true},
	16406: {Code: 16406, Name: "USE_BATTERY_CONTINUE", Group: "general", Known: true},
	16407: {Code: 16407, Name: "USE_CAMERA_CONTROL", Group: "general", Known: true},
	16408: {Code: 16408, Name: "USE_DASHBOARD", Group: "general", Known: true},
	16409: {Code: 16409, Name: "USE_EMFAT_TOOLS", Group: "general", Known: true},
	16410: {Code: 16410, Name: "USE_ESCSERIAL_SIMONK", Group: "general", Known: true},
	16411: {Code: 16411, Name: "USE_FRSKYOSD", Group: "general", Known: true},
	16412: {Code: 16412, Name: "USE_GPS", Group: "general", Known: true},
	16413: {Code: 16413, Name: "USE_LED_STRIP", Group: "general", Known: true},
	16414: {Code: 16414, Name: "USE_LED_STRIP_64", Group: "general", Known: true},
	16415: {Code: 16415, Name: "USE_MAG", Group: "general", Known: true},
	16416: {Code: 16416, Name: "USE_OSD_SD", Group: "general", Known: true},
	16417: {Code: 16417, Name: "USE_OSD_HD", Group: "general", Known: true},
	16418: {Code: 16418, Name: "USE_PINIO", Group: "general", Known: true},
	16419: {Code: 16419, Name: "USE_RACE_PRO", Group: "general", Known: true},
	16420: {Code: 16420, Name: "USE_SERVOS", Group: "general", Known: true},
	16421: {Code: 16421, Name: "USE_VTX", Group: "general", Known: true},
	16422: {Code: 16422, Name: "USE_ALTITUDE_HOLD", Group: "general", Known: true},
	16423: {Code: 16423, Name: "USE_SOFTSERIAL", Group: "general", Known: true},
	16424: {Code: 16424, Name: "USE_WING", Group: "general", Known: true},
	16425: {Code: 16425, Name: "USE_POSITION_HOLD", Group: "general", Known: true},
	16426: {Code: 16426, Name: "USE_CHIRP", Group: "general", Known: true},
	16427: {Code: 16427, Name: "USE_FLIGHT_PLAN", Group: "general", Known: true},
	16428: {Code: 16428, Name: "USE_OPTICALFLOW", Group: "general", Known: true},
	16429: {Code: 16429, Name: "USE_RANGEFINDER", Group: "general", Known: true},
}
