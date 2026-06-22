package fakefc

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type FC struct {
	mu          sync.Mutex
	readTimeout time.Duration
	closed      bool
	out         bytes.Buffer
	CLI         map[string][]string
	Unsupported map[uint16]bool
	SaveCloses  bool
}

func New() *FC {
	return &FC{
		CLI: map[string][]string{
			"version": {
				"# Betaflight / STM32F405 (F405) 2025.12.1 Jan 01 2026 / 00:00:00 (fake) MSP API: 1.48",
				"# board: manufacturer_id: FAKE, board_name: FAKEF405",
			},
			"diff all": {
				"# version",
				"batch start",
				"defaults nosave",
				"board_name FAKEF405",
				"manufacturer_id FAKE",
				"profile 1",
				"rateprofile 2",
				"feature GPS",
				"serial UART1 64 115200 57600 0 115200",
				"aux 0 0 0 1700 2100 0 0",
				"set gyro_lpf1_static_hz = 0",
				"save",
			},
			"dump all": {
				"# version",
				"batch start",
				"defaults nosave",
				"board_name FAKEF405",
				"manufacturer_id FAKE",
				"profile 1",
				"rateprofile 2",
				"feature GPS",
				"serial UART1 64 115200 57600 0 115200",
				"aux 0 0 0 1700 2100 0 0",
				"resource MOTOR 1 A00",
				"vtxtable bands 1",
				"vtxtable channels 8",
				"vtxtable band 1 RACEBAND R FACTORY 5658 5695 5732 5769 5806 5843 5880 5917",
				"vtxtable powerlevels 2",
				"vtxtable powervalues 25 200",
				"vtxtable powerlabels 25 200",
				"led 0 0,0::C:0",
				"servo 0 1000 2000 1500 100 -1",
				"smix reverse 0 2 r",
				"adjrange 0 0 0 900 1300 12 0 0 0",
				"rxrange 0 1000 2000",
				"set gyro_lpf1_static_hz = 0",
				"set dterm_lpf1_static_hz = 90",
				"set p_roll = 45",
				"set roll_rc_rate = 7",
				"set serialrx_provider = CRSF",
				"set vtx_band = 5",
				"set osd_units = METRIC",
				"set gps_rescue_min_sats = 8",
				"set failsafe_procedure = DROP",
				"set bat_capacity = 1300",
				"set vbat_warning_cell_voltage = 350",
				"save",
			},
			"defaults nosave":                       {"defaults loaded without save"},
			"set gyro_lpf1_static_hz = 0":           {"gyro_lpf1_static_hz set to 0"},
			"set p_roll = 46":                       {"p_roll set to 46"},
			"set roll_rc_rate = 8":                  {"roll_rc_rate set to 8"},
			"set dterm_lpf1_static_hz = 100":        {"dterm_lpf1_static_hz set to 100"},
			"set serialrx_provider = CRSF":          {"serialrx_provider set to CRSF"},
			"set vtx_band = 5":                      {"vtx_band set to 5"},
			"set osd_units = METRIC":                {"osd_units set to METRIC"},
			"set gps_rescue_min_sats = 10":          {"gps_rescue_min_sats set to 10"},
			"set failsafe_procedure = DROP":         {"failsafe_procedure set to DROP"},
			"feature GPS":                           {"Enabled GPS"},
			"feature -GPS":                          {"Disabled GPS"},
			"serial UART1 64 115200 57600 0 115200": {"serial updated"},
			"aux 0 0 0 1700 2100 0 0":               {"aux updated"},
			"resource MOTOR 1 A00":                  {"resource updated"},
			"vtxtable bands 1":                      {"vtxtable updated"},
			"led 0 0,0::C:0":                        {"led updated"},
			"servo 0 1000 2000 1500 100 -1":         {"servo updated"},
			"smix reverse 0 2 r":                    {"smix updated"},
			"adjrange 0 0 0 900 1300 12 0 0 0":      {"adjrange updated"},
			"rxrange 0 1000 2000":                   {"rxrange updated"},
			"rxfail 2 s 1100":                       {"rxfail updated"},
			"profile 1":                             {"profile 1"},
			"rateprofile 2":                         {"rateprofile 2"},
			"battery_profile 1":                     {"battery_profile 1"},
			"save":                                  {"Saving..."},
		},
		Unsupported: map[uint16]bool{},
	}
}

func (f *FC) Read(p []byte) (int, error) {
	deadline := time.Now().Add(f.readTimeout)
	for {
		f.mu.Lock()
		if f.closed {
			f.mu.Unlock()
			return 0, io.EOF
		}
		if f.out.Len() > 0 {
			n, _ := f.out.Read(p)
			f.mu.Unlock()
			return n, nil
		}
		timeout := f.readTimeout
		f.mu.Unlock()
		if timeout <= 0 {
			return 0, nil
		}
		if time.Now().After(deadline) {
			return 0, nil
		}
		time.Sleep(time.Millisecond)
	}
}

func (f *FC) Write(p []byte) (int, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.closed {
		return 0, io.ErrClosedPipe
	}
	if len(p) == 0 {
		return 0, nil
	}
	if p[0] == 0x02 {
		f.handleCLI(p)
		return len(p), nil
	}
	frame, err := msp.ReadFrame(bytes.NewReader(p))
	if err != nil {
		return 0, err
	}
	f.handleMSP(frame)
	return len(p), nil
}

func (f *FC) ResetInputBuffer() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.out.Reset()
	return nil
}

func (f *FC) ResetOutputBuffer() error {
	return nil
}

func (f *FC) SetReadTimeout(timeout time.Duration) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.readTimeout = timeout
	return nil
}

func (f *FC) Close() error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.closed = true
	return nil
}

func (f *FC) handleMSP(frame msp.Frame) {
	if f.Unsupported[frame.Code] {
		f.out.Write(response(frame.Code, nil, true))
		return
	}
	switch frame.Code {
	case msp.MSPAPIVersion:
		f.out.Write(response(frame.Code, []byte{0, 1, 48}, false))
	case msp.MSPFCVariant:
		f.out.Write(response(frame.Code, []byte("BTFL"), false))
	case msp.MSPFCVersion:
		payload := []byte{25, 12, 1}
		payload = append(payload, byte(len("2025.12.1")))
		payload = append(payload, []byte("2025.12.1")...)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBoardInfo:
		payload := []byte("F405")
		payload = appendU16(payload, 0)
		payload = append(payload, 0, 65)
		payload = appendPString(payload, "STM32F405")
		payload = appendPString(payload, "FAKEF405")
		payload = appendPString(payload, "FAKE")
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPFeatureConfig:
		f.out.Write(response(frame.Code, appendU32(nil, 0x00040488), false))
	case msp.MSPStatusEx:
		payload := make([]byte, 0, 32)
		payload = appendU16(payload, 250)
		payload = appendU16(payload, 0)
		payload = appendU16(payload, 33)
		payload = appendU32(payload, 2)
		payload = append(payload, 0)
		payload = appendU16(payload, 42)
		payload = append(payload, 4, 0, 4, 0, 0, 0, 0, 29)
		payload = appendU32(payload, 0x1234)
		payload = append(payload, 0)
		payload = appendU16(payload, 425)
		payload = append(payload, 6, 3, 1)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPAttitude:
		payload := appendS16(nil, -10)
		payload = appendS16(payload, 20)
		payload = appendS16(payload, 180)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBatteryState:
		payload := []byte{4}
		payload = appendU16(payload, 1300)
		payload = append(payload, 160)
		payload = appendU16(payload, 123)
		payload = appendS16(payload, 456)
		payload = append(payload, 0)
		payload = appendU16(payload, 1599)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBatteryConfig:
		f.out.Write(response(frame.Code, fakeBatteryConfigPayload(), false))
	case msp.MSP2BatteryProfile:
		f.out.Write(response(frame.Code, fakeBatteryProfilePayload(), false))
	case msp.MSPVoltageMeters:
		f.out.Write(response(frame.Code, []byte{10, 160, 40, 120}, false))
	case msp.MSPCurrentMeters:
		payload := []byte{10}
		payload = appendU16(payload, 123)
		payload = appendU16(payload, 4560)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPVoltageMeterConfig:
		f.out.Write(response(frame.Code, []byte{1, 5, 10, 0, 110, 10, 1}, false))
	case msp.MSPCurrentMeterConfig:
		payload := []byte{1, 6, 10, 1}
		payload = appendS16(payload, 400)
		payload = appendS16(payload, -10)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPArmingConfig:
		f.out.Write(response(frame.Code, []byte{5, 0, 25, 1}, false))
	case msp.MSPFailsafeConfig:
		payload := []byte{15, 60}
		payload = appendU16(payload, 1000)
		payload = append(payload, 2)
		payload = appendU16(payload, 100)
		payload = append(payload, 1)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBoardAlignmentConfig:
		payload := appendS16(nil, -2)
		payload = appendS16(payload, 3)
		payload = appendS16(payload, 90)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBlackboxConfig:
		payload := []byte{1, 2, 1, 4}
		payload = appendU16(payload, 16)
		payload = append(payload, 2)
		payload = appendU32(payload, 0x1001)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBoxnames:
		f.out.Write(response(frame.Code, modeNamePage(frame.Payload), false))
	case msp.MSPBoxids:
		f.out.Write(response(frame.Code, modeIDPage(frame.Payload), false))
	case msp.MSPModeRanges:
		payload := []byte{0, 0, 32, 48, 52, 2, 16, 32, 0, 0, 0, 0}
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPModeRangesExtra:
		payload := []byte{3, 0, 0, 0, 52, 1, 53, 0, 0, 0}
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPRXConfig:
		payload := []byte{2}
		payload = appendU16(payload, 2000)
		payload = appendU16(payload, 1500)
		payload = appendU16(payload, 1000)
		payload = append(payload, 0)
		payload = appendU16(payload, 885)
		payload = appendU16(payload, 2115)
		payload = append(payload, 0, 0)
		payload = appendU16(payload, 1350)
		payload = append(payload, 0)
		payload = appendU32(payload, 0)
		payload = append(payload, 0, 10, 0, 0, 50, 60, 70, 0, 0, 80, 1)
		payload = append(payload, 1, 2, 3, 4, 5, 6, 7)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPRXMap:
		f.out.Write(response(frame.Code, []byte{0, 1, 3, 2}, false))
	case msp.MSPRSSIConfig:
		f.out.Write(response(frame.Code, []byte{8}, false))
	case msp.MSPRxfailConfig:
		payload := []byte{0}
		payload = appendU16(payload, 1000)
		payload = append(payload, 1)
		payload = appendU16(payload, 1500)
		payload = append(payload, 2)
		payload = appendU16(payload, 1100)
		payload = append(payload, 1)
		payload = appendU16(payload, 1500)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPGPSConfig:
		f.out.Write(response(frame.Code, []byte{1, 0, 1, 1, 1, 1}, false))
	case msp.MSPRawGPS:
		payload := []byte{1, 12}
		payload = appendS32(payload, 599123456)
		payload = appendS32(payload, 105123456)
		payload = appendU16(payload, 124)
		payload = appendU16(payload, 1450)
		payload = appendU16(payload, 2715)
		payload = appendU16(payload, 95)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPCompGPS:
		payload := appendU16(nil, 342)
		payload = appendU16(payload, 184)
		payload = append(payload, 1)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPGPSRescue:
		payload := appendU16(nil, 3200)
		payload = appendU16(payload, 100)
		payload = appendU16(payload, 50)
		payload = appendU16(payload, 1500)
		payload = appendU16(payload, 1200)
		payload = appendU16(payload, 1800)
		payload = appendU16(payload, 1450)
		payload = append(payload, 1, 8)
		payload = appendU16(payload, 500)
		payload = appendU16(payload, 150)
		payload = append(payload, 1, 2)
		payload = appendU16(payload, 30)
		payload = appendU16(payload, 20)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPGPSRescuePids:
		payload := appendU16(nil, 80)
		payload = appendU16(payload, 10)
		payload = appendU16(payload, 5)
		payload = appendU16(payload, 120)
		payload = appendU16(payload, 20)
		payload = appendU16(payload, 10)
		payload = appendU16(payload, 45)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPGpssvinfo:
		f.out.Write(response(frame.Code, []byte{2, 0, 12, 4, 45, 1, 24, 3, 39}, false))
	case msp.MSPSensorConfig:
		f.out.Write(response(frame.Code, []byte{1, 2, 3, 4, 5}, false))
	case msp.MSP2SensorConfigActive:
		f.out.Write(response(frame.Code, []byte{10, 11, 12, 13, 14, 15}, false))
	case msp.MSPRawImu:
		payload := appendS16(nil, 2048)
		payload = appendS16(payload, -1024)
		payload = appendS16(payload, 512)
		payload = appendS16(payload, 100)
		payload = appendS16(payload, -50)
		payload = appendS16(payload, 25)
		payload = appendS16(payload, 300)
		payload = appendS16(payload, -200)
		payload = appendS16(payload, 100)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSensorAlignment:
		payload := []byte{1, 1, 2, 3, 0x03}
		payload = appendS16(payload, -10)
		payload = appendS16(payload, 20)
		payload = appendS16(payload, 900)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPCompassConfig:
		payload := appendS16(nil, 123)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBeeperConfig:
		payload := appendU32(nil, 0x00000012)
		payload = append(payload, 3)
		payload = appendU32(payload, 0x00000202)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPPID:
		f.out.Write(response(frame.Code, []byte{
			45, 80, 30,
			47, 84, 34,
			45, 80, 0,
			50, 50, 75,
			40, 0, 0,
		}, false))
	case msp.MSPPidnames:
		f.out.Write(response(frame.Code, []byte("ROLL;PITCH;YAW;LEVEL;MAG;"), false))
	case msp.MSPPIDController:
		f.out.Write(response(frame.Code, []byte{0}, false))
	case msp.MSPRCTuning:
		payload := []byte{7, 10, 70, 72, 65, 0, 50, 20}
		payload = appendU16(payload, 0)
		payload = append(payload, 5, 8, 7, 9, 1, 80)
		payload = appendU16(payload, 900)
		payload = appendU16(payload, 850)
		payload = appendU16(payload, 800)
		payload = append(payload, 3, 45)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPPIDAdvanced:
		f.out.Write(response(frame.Code, fakePIDAdvancedPayload(), false))
	case msp.MSPAdvancedConfig:
		f.out.Write(response(frame.Code, fakeAdvancedConfigPayload(), false))
	case msp.MSPFilterConfig:
		f.out.Write(response(frame.Code, fakeFilterConfigPayload(), false))
	case msp.MSPMotorConfig:
		payload := appendU16(nil, 0)
		payload = appendU16(payload, 2000)
		payload = appendU16(payload, 1000)
		payload = append(payload, 4, 14, 1, 1)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPMotor:
		payload := appendU16(nil, 1000)
		payload = appendU16(payload, 1001)
		payload = appendU16(payload, 1002)
		payload = appendU16(payload, 1003)
		payload = appendU16(payload, 0)
		payload = appendU16(payload, 0)
		payload = appendU16(payload, 0)
		payload = appendU16(payload, 0)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPMotorTelemetry:
		payload := []byte{2}
		payload = appendU32(payload, 12500)
		payload = appendU16(payload, 25)
		payload = append(payload, 42)
		payload = appendU16(payload, 1680)
		payload = appendU16(payload, 230)
		payload = appendU16(payload, 120)
		payload = appendU32(payload, 12600)
		payload = appendU16(payload, 30)
		payload = append(payload, 43)
		payload = appendU16(payload, 1675)
		payload = appendU16(payload, 240)
		payload = appendU16(payload, 121)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPMotor3dConfig:
		payload := appendU16(nil, 1406)
		payload = appendU16(payload, 1514)
		payload = appendU16(payload, 1460)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSP2MotorOutputReordering:
		f.out.Write(response(frame.Code, []byte{4, 0, 1, 2, 3}, false))
	case msp.MSPServo:
		payload := appendU16(nil, 1500)
		payload = appendU16(payload, 1501)
		payload = appendU16(payload, 0)
		payload = appendU16(payload, 0)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPServoConfigurations:
		payload := appendU16(nil, 1000)
		payload = appendU16(payload, 2000)
		payload = appendU16(payload, 1500)
		payload = append(payload, 100, 255)
		payload = appendU32(payload, 0x05)
		payload = appendU16(payload, 1100)
		payload = appendU16(payload, 1900)
		payload = appendU16(payload, 1501)
		payload = append(payload, 206, 2)
		payload = appendU32(payload, 0)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPServoMixRules:
		f.out.Write(response(frame.Code, []byte{0, 1, 100, 10, 0, 100, 2, 0, 0, 0, 0, 0, 0, 0}, false))
	case msp.MSPVTXConfig:
		payload := []byte{3, 5, 8, 2, 1}
		payload = appendU16(payload, 5861)
		payload = append(payload, 1, 2)
		payload = appendU16(payload, 5662)
		payload = append(payload, 1, 5, 8, 3)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPLedStripConfig:
		payload := appendU32(nil, fakeLEDConfigRaw(1, 2, 0, 1, 3, 0x03))
		payload = appendU32(payload, fakeLEDConfigRaw(3, 4, 1, 1<<3, 5, 0x04))
		payload = append(payload, 1, 0)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPLedColors:
		payload := appendU16(nil, 0)
		payload = append(payload, 0, 255)
		payload = appendU16(payload, 120)
		payload = append(payload, 255, 255)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPLedStripModecolor:
		f.out.Write(response(frame.Code, []byte{0, 0, 3, 1, 2, 5}, false))
	case msp.MSP2GetLedStripConfigValues:
		payload := []byte{50}
		payload = appendU16(payload, 20)
		payload = appendU16(payload, 120)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSP2CommonSerialConfig:
		payload := []byte{2}
		payload = append(payload, 20)
		payload = appendU32(payload, 1)
		payload = append(payload, 5, 0, 0, 5)
		payload = append(payload, 51)
		payload = appendU32(payload, 64|2)
		payload = append(payload, 5, 4, 0, 0)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPOSDConfig:
		payload := []byte{0x31, 3, 1, 20}
		payload = appendU16(payload, 1500)
		payload = append(payload, 0, 3)
		payload = appendU16(payload, 120)
		payload = appendU16(payload, 0x0800|10|(2<<5))
		payload = appendU16(payload, 5|(3<<5))
		payload = appendU16(payload, 0x0800|20|(4<<5))
		payload = append(payload, 2, 1, 0, 2)
		payload = appendU16(payload, 0x0123)
		payload = appendU16(payload, 0x0456)
		payload = appendU16(payload, 0x0005)
		payload = append(payload, 4)
		payload = appendU32(payload, 0x00000005)
		payload = append(payload, 3, 2, 1, 24, 18)
		payload = appendU16(payload, 70)
		payload = appendS16(payload, -95)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPOSDCanvas:
		f.out.Write(response(frame.Code, []byte{53, 20}, false))
	case msp.MSP2GetOSDWarnings:
		payload := []byte{2}
		payload = appendPString(payload, "LOW BATTERY")
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPRC:
		var payload []byte
		for _, v := range []uint16{1500, 1500, 1500, 1000, 2000, 1500} {
			payload = appendU16(payload, v)
		}
		f.out.Write(response(frame.Code, payload, false))
	default:
		f.out.Write(response(frame.Code, nil, true))
	}
}

func (f *FC) handleCLI(p []byte) {
	command := strings.TrimSpace(string(bytes.TrimSuffix(bytes.TrimPrefix(p, []byte{0x02}), []byte{0x03})))
	command = strings.TrimSuffix(command, "\n")
	lines, ok := f.CLI[command]
	if !ok {
		lines = []string{fmt.Sprintf("unknown command: %s", command)}
	}
	f.out.WriteByte(0x02)
	for _, line := range lines {
		f.out.WriteString(line)
		f.out.WriteByte('\n')
	}
	f.out.WriteByte(0x03)
	if command == "save" && f.SaveCloses {
		f.closed = true
	}
}

func response(code uint16, payload []byte, unsupported bool) []byte {
	frame := msp.EncodeRequest(code, payload)
	if unsupported {
		frame[2] = '!'
	} else {
		frame[2] = '>'
	}
	return frame
}

func appendPString(dst []byte, s string) []byte {
	dst = append(dst, byte(len(s)))
	return append(dst, []byte(s)...)
}

func appendU16(dst []byte, v uint16) []byte {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	return append(dst, buf[:]...)
}

func appendS16(dst []byte, v int16) []byte {
	return appendU16(dst, uint16(v))
}

func appendS32(dst []byte, v int32) []byte {
	return appendU32(dst, uint32(v))
}

func appendU32(dst []byte, v uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	return append(dst, buf[:]...)
}

func fakeLEDConfigRaw(x, y, function uint8, overlays uint16, color, directions uint8) uint32 {
	return uint32(y&0x0f) |
		uint32(x&0x0f)<<4 |
		uint32(function&0x0f)<<8 |
		uint32(overlays&0x03ff)<<12 |
		uint32(color&0x0f)<<22 |
		uint32(directions&0x3f)<<26
}

func fakePIDAdvancedPayload() []byte {
	payload := appendU16(nil, 0)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 0)
	payload = append(payload, 0, 0, 20, 0, 0, 0, 0)
	payload = appendU16(payload, 100)
	payload = appendU16(payload, 120)
	payload = append(payload, 55, 0)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 3500)
	payload = appendU16(payload, 0)
	payload = append(payload, 1, 0, 2, 1, 0, 5, 20)
	payload = appendU16(payload, 120)
	payload = appendU16(payload, 125)
	payload = appendU16(payload, 120)
	payload = append(payload, 0, 40, 42, 0, 35, 50, 0, 20, 15, 95, 0, 30, 2, 10, 20, 30, 90, 5, 10, 2, 15)
	payload = appendU16(payload, 1350)
	return payload
}

func fakeAdvancedConfigPayload() []byte {
	payload := []byte{1, 4, 0, 7}
	payload = appendU16(payload, 480)
	payload = appendU16(payload, 550)
	payload = append(payload, 0, 1, 0, 1, 32)
	payload = appendU16(payload, 125)
	payload = appendU16(payload, 10)
	payload = append(payload, 2, 5, 80)
	return payload
}

func fakeFilterConfigPayload() []byte {
	payload := []byte{90}
	payload = appendU16(payload, 100)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 0)
	payload = appendU16(payload, 0)
	payload = append(payload, 0, 1, 0)
	payload = appendU16(payload, 150)
	payload = appendU16(payload, 500)
	payload = append(payload, 0, 2)
	payload = appendU16(payload, 150)
	payload = append(payload, 2)
	payload = appendU16(payload, 100)
	payload = appendU16(payload, 400)
	payload = appendU16(payload, 80)
	payload = appendU16(payload, 200)
	payload = append(payload, 0, 0)
	payload = appendU16(payload, 300)
	payload = appendU16(payload, 100)
	payload = append(payload, 3, 100)
	payload = appendU16(payload, 600)
	payload = append(payload, 5, 3)
	payload = appendU16(payload, 50)
	payload = appendU16(payload, 500)
	payload = append(payload, 100, 80, 60)
	return payload
}

func fakeBatteryConfigPayload() []byte {
	payload := []byte{33, 43, 35}
	payload = appendU16(payload, 1300)
	payload = append(payload, 1, 1)
	payload = appendU16(payload, 330)
	payload = appendU16(payload, 435)
	payload = appendU16(payload, 350)
	return payload
}

func fakeBatteryProfilePayload() []byte {
	payload := []byte{0}
	payload = appendU16(payload, 330)
	payload = appendU16(payload, 435)
	payload = appendU16(payload, 350)
	payload = appendU16(payload, 420)
	payload = appendU16(payload, 1300)
	payload = append(payload, 4, 20)
	return payload
}

func modeNamePage(payload []byte) []byte {
	page := requestPage(payload)
	names := fakeModeNames()
	start := page * 32
	if start >= len(names) {
		return nil
	}
	end := start + 32
	if end > len(names) {
		end = len(names)
	}
	return []byte(strings.Join(names[start:end], ";") + ";")
}

func modeIDPage(payload []byte) []byte {
	page := requestPage(payload)
	ids := fakeModeIDs()
	start := page * 32
	if start >= len(ids) {
		return nil
	}
	end := start + 32
	if end > len(ids) {
		end = len(ids)
	}
	return ids[start:end]
}

func requestPage(payload []byte) int {
	if len(payload) == 0 {
		return 0
	}
	return int(payload[0])
}

func fakeModeNames() []string {
	return []string{
		"ARM",
		"ANGLE",
		"HORIZON",
		"ALTHOLD",
		"ANTI GRAVITY",
		"MAG",
		"HEADFREE",
		"HEADADJ",
		"CAMSTAB",
		"POS HOLD",
		"PASSTHRU",
		"BEEPER",
		"LEDLOW",
		"CALIB",
		"OSD DISABLE",
		"TELEMETRY",
		"SERVO1",
		"SERVO2",
		"SERVO3",
		"BLACKBOX",
		"FAILSAFE",
		"AIR MODE",
		"3D DISABLE",
		"FPV ANGLE MIX",
		"BLACKBOX ERASE",
		"CAMERA CONTROL 1",
		"CAMERA CONTROL 2",
		"CAMERA CONTROL 3",
		"FLIP OVER AFTER CRASH",
		"PREARM",
		"GPS BEEP SATELLITE COUNT",
		"VTX PIT MODE",
		"BEEPER MUTE",
		"READY",
	}
}

func fakeModeIDs() []byte {
	return []byte{
		0,
		1,
		2,
		3,
		4,
		5,
		6,
		7,
		8,
		11,
		12,
		13,
		15,
		17,
		19,
		20,
		23,
		24,
		25,
		26,
		27,
		28,
		29,
		30,
		31,
		32,
		33,
		34,
		35,
		36,
		37,
		39,
		52,
		53,
	}
}
