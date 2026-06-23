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
	mu                 sync.Mutex
	readTimeout        time.Duration
	closed             bool
	out                bytes.Buffer
	CLI                map[string][]string
	Unsupported        map[uint16]bool
	SaveCloses         bool
	DataflashUsedBytes uint32
	DataflashData      []byte
}

func New() *FC {
	return &FC{
		CLI: map[string][]string{
			"version": {
				"# Betaflight / STM32F405 (F405) 2025.12.1 Jan 01 2026 / 00:00:00 (fake) MSP API: 1.48",
				"# board: manufacturer_id: FAKE, board_name: FAKEF405",
			},
			"tasks": {
				"Task list             rate/hz  max/us  avg/us maxload avgload  total/ms   late    run reqd/us",
				"00 - (         SYSTEM)    1000      10       4  0.2%  0.1%       100      0   1000       5",
				"01 - (            PID)    8000      20       8  2.5%  1.5%       200      1   2000      10",
				"Check Functions (RX, ...)          6       2          0.3%        50",
				"Total (excluding SERIAL)                       1.6%",
			},
			"status": {
				"CONFIG: CONFIGURED (3820b / 16384b)",
				"",
				"DEVICES DETECTED: SPI=2, I2C=1 (0 errors)",
				"GYRO: (1) BMI270 enabled locked dma",
				"GPS: connected, UART1 115200 (set to AUTO), configured, version =  M10",
				"OSD: MSP (53 x 20)",
				"FLASH: JEDEC ID=0x00abcdef 16M",
				"",
				"BUILD KEY: fake-build-key (2025.12.1)",
				"",
				"System Uptime: 123 seconds, Current Time: 2026-06-23 12:34:56",
				"CPU:42%, cycle time: 250, GYRO rate: 4000, RX rate: 250, System rate: 10",
				"Voltage: 15.99V (4S battery - OK)",
				"Arming disable flags: RXLOSS THROTTLE",
			},
			"resource show all": {
				"# resources",
				"resource MOTOR 1 A00",
				"resource SERIAL_TX 1 A09",
				"# timer",
				"timer A00 AF1",
				"timer A09 NONE",
				"# dma",
				"dma pin A00 1",
				"dma SPI_TX 1 0",
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
			"beeper ARMING":                        {"Beeper ARMING enabled"},
			"beeper -ARMING":                       {"Beeper ARMING disabled"},
			"set transponder_provider = ARCITIMER":  {"transponder_provider set to ARCITIMER"},
			"set transponder_provider = NONE":       {"transponder_provider set to NONE"},
			"set transponder_data = 1,2,3":        {"transponder_data set to 1,2,3"},
			"set transponder_data = ":              {"transponder_data set to "},
			"get gyro_lpf1_static_hz":              {"gyro_lpf1_static_hz = 0"},
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
		Unsupported:        map[uint16]bool{},
		DataflashUsedBytes: 262144,
		DataflashData:      fakeDataflashLogData(),
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
	case msp.MSP2GetText:
		payload, ok := fakeTextPayload(frame.Payload)
		f.out.Write(response(frame.Code, payload, !ok))
	case msp.MSP2SetText:
		f.out.Write(response(frame.Code, nil, !fakeTextSetPayloadValid(frame.Payload)))
	case msp.MSPBoardInfo:
		payload := []byte("F405")
		payload = appendU16(payload, 0)
		payload = append(payload, 0, 65)
		payload = appendPString(payload, "STM32F405")
		payload = appendPString(payload, "FAKEF405")
		payload = appendPString(payload, "FAKE")
		for i := 0; i < 32; i++ {
			payload = append(payload, byte(i))
		}
		payload = append(payload, 254, 1)
		payload = appendU16(payload, 8000)
		payload = appendU32(payload, 3)
		payload = append(payload, 2, 1)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPBuildInfo:
		payload := []byte("Jan 01 2026")
		payload = append(payload, []byte("00:00:00")...)
		payload = append(payload, []byte("abc1234")...)
		payload = appendU16(payload, 16412)
		payload = appendU16(payload, 8231)
		payload = appendU16(payload, 65000)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSP2McuInfo:
		payload := []byte{254}
		payload = appendPString(payload, "STM32F405")
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPUID:
		payload := appendU32(nil, 0x01234567)
		payload = appendU32(payload, 0x89abcdef)
		payload = appendU32(payload, 0x00000042)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPName:
		f.out.Write(response(frame.Code, []byte("BetaFlight"), false))
	case msp.MSPFeatureConfig:
		f.out.Write(response(frame.Code, appendU32(nil, 0x00040488), false))
	case msp.MSPSetFeatureConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 4))
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
	case msp.MSPDebug:
		payload := appendS16(nil, -1)
		payload = appendS16(payload, 2)
		payload = appendS16(payload, -3)
		payload = appendS16(payload, 4)
		payload = appendS16(payload, -5)
		payload = appendS16(payload, 6)
		payload = appendS16(payload, -7)
		payload = appendS16(payload, 8)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPAltitude:
		payload := appendU32(nil, 12345)
		payload = appendS16(payload, -67)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSonarAltitude:
		f.out.Write(response(frame.Code, appendU32(nil, 4321), false))
	case msp.MSPAnalog:
		payload := []byte{160}
		payload = appendU16(payload, 321)
		payload = appendU16(payload, 900)
		payload = appendS16(payload, -123)
		payload = appendU16(payload, 1599)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPRtc:
		payload := appendU16(nil, 2026)
		payload = append(payload, 6, 23, 12, 34, 56)
		payload = appendU16(payload, 789)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetRtc:
		f.out.Write(response(frame.Code, nil, false))
	case msp.MSPAccCalibration, msp.MSPMagCalibration:
		f.out.Write(response(frame.Code, nil, false))
	case msp.MSPCopyProfile:
		f.out.Write(response(frame.Code, nil, false))
	case msp.MSPReboot:
		if len(frame.Payload) == 0 {
			f.out.Write(response(frame.Code, []byte{0}, false))
			return
		}
		mode := frame.Payload[0]
		payload := []byte{mode}
		if mode == 2 || mode == 3 {
			payload = append(payload, 1)
		}
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
	case msp.MSPSetBatteryConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 13))
	case msp.MSP2BatteryProfile:
		f.out.Write(response(frame.Code, fakeBatteryProfilePayload(), false))
	case msp.MSP2SetBatteryProfile:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 13))
	case msp.MSPVoltageMeters:
		f.out.Write(response(frame.Code, []byte{10, 160, 40, 120}, false))
	case msp.MSPCurrentMeters:
		payload := []byte{10}
		payload = appendU16(payload, 123)
		payload = appendU16(payload, 4560)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPVoltageMeterConfig:
		f.out.Write(response(frame.Code, []byte{1, 5, 10, 0, 110, 10, 1}, false))
	case msp.MSPSetVoltageMeterConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 4))
	case msp.MSPCurrentMeterConfig:
		payload := []byte{1, 6, 10, 1}
		payload = appendS16(payload, 400)
		payload = appendS16(payload, -10)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetCurrentMeterConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 5))
	case msp.MSPArmingConfig:
		f.out.Write(response(frame.Code, []byte{5, 0, 25, 1}, false))
	case msp.MSPSetArmingConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 4))
	case msp.MSPFailsafeConfig:
		payload := []byte{15, 60}
		payload = appendU16(payload, 1000)
		payload = append(payload, 2)
		payload = appendU16(payload, 100)
		payload = append(payload, 1)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetFailsafeConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 8))
	case msp.MSPBoardAlignmentConfig:
		payload := appendS16(nil, -2)
		payload = appendS16(payload, 3)
		payload = appendS16(payload, 90)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetBoardAlignmentConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 6))
	case msp.MSPBlackboxConfig:
		payload := []byte{1, 2, 1, 4}
		payload = appendU16(payload, 16)
		payload = append(payload, 2)
		payload = appendU32(payload, 0x1001)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetBlackboxConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 10))
	case msp.MSPDataflashSummary:
		payload := []byte{3}
		payload = appendU32(payload, 16)
		payload = appendU32(payload, 1048576)
		payload = appendU32(payload, f.DataflashUsedBytes)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPDataflashRead:
		if len(frame.Payload) < 7 {
			f.out.Write(response(frame.Code, nil, true))
			return
		}
		address := binary.LittleEndian.Uint32(frame.Payload[:4])
		blockSize := binary.LittleEndian.Uint16(frame.Payload[4:6])
		payload := appendU32(nil, address)
		if address >= uint32(len(f.DataflashData)) || address >= f.DataflashUsedBytes {
			payload = appendU16(payload, 0)
			payload = append(payload, 0)
			f.out.Write(response(frame.Code, payload, false))
			return
		}
		end := address + uint32(blockSize)
		if end > f.DataflashUsedBytes {
			end = f.DataflashUsedBytes
		}
		if end > uint32(len(f.DataflashData)) {
			end = uint32(len(f.DataflashData))
		}
		data := f.DataflashData[address:end]
		payload = appendU16(payload, uint16(len(data)))
		payload = append(payload, 0)
		payload = append(payload, data...)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPDataflashErase:
		f.DataflashUsedBytes = 0
		f.out.Write(response(frame.Code, nil, false))
	case msp.MSPSdcardSummary:
		payload := []byte{1, 4, 0}
		payload = appendU32(payload, 4096)
		payload = appendU32(payload, 32768)
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
	case msp.MSPSetModeRange:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 5 && len(frame.Payload) != 7))
	case msp.MSPAdjustmentRanges:
		payload := []byte{0, 0, 0, 16, 12, 0}
		payload = appendU16(payload, 1500)
		payload = appendU16(payload, 100)
		payload = append(payload, 0, 1, 4, 8, 33, 2)
		payload = appendU16(payload, 1600)
		payload = appendU16(payload, 50)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetAdjustmentRange:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 11))
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
	case msp.MSPSetRXConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 48))
	case msp.MSPRXMap:
		f.out.Write(response(frame.Code, []byte{0, 1, 3, 2}, false))
	case msp.MSPSetRXMap:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 4))
	case msp.MSPRSSIConfig:
		f.out.Write(response(frame.Code, []byte{8}, false))
	case msp.MSPSetRSSIConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 1))
	case msp.MSPRCDeadband:
		payload := []byte{5, 7, 3}
		payload = appendU16(payload, 50)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetRCDeadband:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 5))
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
	case msp.MSPSetRxfailConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 4))
	case msp.MSPGPSConfig:
		f.out.Write(response(frame.Code, []byte{1, 0, 1, 1, 1, 1}, false))
	case msp.MSPSetGPSConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 6))
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
	case msp.MSPSetGPSRescue:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 26))
	case msp.MSPGPSRescuePids:
		payload := appendU16(nil, 80)
		payload = appendU16(payload, 10)
		payload = appendU16(payload, 5)
		payload = appendU16(payload, 120)
		payload = appendU16(payload, 20)
		payload = appendU16(payload, 10)
		payload = appendU16(payload, 45)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetGPSRescuePids:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 14))
	case msp.MSPGpssvinfo:
		f.out.Write(response(frame.Code, []byte{2, 0, 12, 4, 45, 1, 24, 3, 39}, false))
	case msp.MSPSensorConfig:
		f.out.Write(response(frame.Code, []byte{1, 2, 3, 4, 5}, false))
	case msp.MSPSetSensorConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 4))
	case msp.MSP2SensorConfigActive:
		f.out.Write(response(frame.Code, []byte{10, 11, 12, 13, 14, 15}, false))
	case msp.MSP2GyroSensorActive:
		f.out.Write(response(frame.Code, []byte{2, 11, 19}, false))
	case msp.MSPAccTrim:
		payload := appendS16(nil, -12)
		payload = appendS16(payload, 34)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetAccTrim:
		f.out.Write(response(frame.Code, nil, false))
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
	case msp.MSPSetSensorAlignment:
		unsupported := len(frame.Payload) != 4 && len(frame.Payload) != 10
		f.out.Write(response(frame.Code, nil, unsupported))
	case msp.MSPCompassConfig:
		payload := appendS16(nil, 123)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetCompassConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 2))
	case msp.MSPBeeperConfig:
		payload := appendU32(nil, 0x00000012)
		payload = append(payload, 3)
		payload = appendU32(payload, 0x00000202)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetBeeperConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 9))
	case msp.MSPTransponderConfig:
		f.out.Write(response(frame.Code, []byte{3, 1, 6, 2, 9, 3, 1, 2, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x42}, false))
	case msp.MSPSetTransponderConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) < 1))
	case msp.MSPPID:
		f.out.Write(response(frame.Code, []byte{
			45, 80, 30,
			47, 84, 34,
			45, 80, 0,
			50, 50, 75,
			40, 0, 0,
		}, false))
	case msp.MSPSetPID:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 15))
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
	case msp.MSPSetRCTuning:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 24))
	case msp.MSPPIDAdvanced:
		f.out.Write(response(frame.Code, fakePIDAdvancedPayload(), false))
	case msp.MSPSetPIDAdvanced:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 61))
	case msp.MSPSimplifiedTuning:
		f.out.Write(response(frame.Code, fakeSimplifiedTuningPayload(), false))
	case msp.MSPCalculateSimplifiedPID:
		f.out.Write(response(frame.Code, fakeSimplifiedPIDCalculationPayload(), len(frame.Payload) != 17))
	case msp.MSPCalculateSimplifiedDterm:
		f.out.Write(response(frame.Code, fakeSimplifiedDtermPayload(), len(frame.Payload) != 18))
	case msp.MSPCalculateSimplifiedGyro:
		f.out.Write(response(frame.Code, fakeSimplifiedGyroPayload(), len(frame.Payload) != 18))
	case msp.MSPValidateSimplifiedTuning:
		f.out.Write(response(frame.Code, []byte{1, 1, 0}, false))
	case msp.MSPSetSimplifiedTuning:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 53))
	case msp.MSPAdvancedConfig:
		f.out.Write(response(frame.Code, fakeAdvancedConfigPayload(), false))
	case msp.MSPSetAdvancedConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 20))
	case msp.MSPFilterConfig:
		f.out.Write(response(frame.Code, fakeFilterConfigPayload(), false))
	case msp.MSPSetFilterConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 56))
	case msp.MSPMotorConfig:
		payload := appendU16(nil, 0)
		payload = appendU16(payload, 2000)
		payload = appendU16(payload, 1000)
		payload = append(payload, 4, 14, 1, 1)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetMotorConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 8))
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
	case msp.MSPSetMotor3dConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 6))
	case msp.MSPMixerConfig:
		f.out.Write(response(frame.Code, []byte{3, 1}, false))
	case msp.MSPSetMixerConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 2))
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
	case msp.MSPSetServoConfiguration:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 13))
	case msp.MSPSetServoMixRule:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 8))
	case msp.MSPVTXConfig:
		payload := []byte{3, 5, 8, 2, 1}
		payload = appendU16(payload, 5861)
		payload = append(payload, 1, 2)
		payload = appendU16(payload, 5662)
		payload = append(payload, 1, 5, 8, 3)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetVTXConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 11))
	case msp.MSPSetVtxtableBand:
		ok := len(frame.Payload) >= 7
		if ok {
			nameLen := int(frame.Payload[1])
			channelCountIndex := 2 + nameLen + 2
			ok = channelCountIndex < len(frame.Payload)
			if ok {
				channelCount := int(frame.Payload[channelCountIndex])
				ok = len(frame.Payload) == channelCountIndex+1+(channelCount*2)
			}
		}
		f.out.Write(response(frame.Code, nil, !ok))
	case msp.MSPSetVtxtablePowerlevel:
		ok := len(frame.Payload) >= 4
		if ok {
			labelLen := int(frame.Payload[3])
			ok = len(frame.Payload) == 4+labelLen
		}
		f.out.Write(response(frame.Code, nil, !ok))
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
	case msp.MSPSetLedColors:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) == 0 || len(frame.Payload)%4 != 0))
	case msp.MSPLedStripModecolor:
		f.out.Write(response(frame.Code, []byte{0, 0, 3, 1, 2, 5}, false))
	case msp.MSPSetLedStripModecolor:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 3))
	case msp.MSP2GetLedStripConfigValues:
		payload := []byte{50}
		payload = appendU16(payload, 20)
		payload = appendU16(payload, 120)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSP2SetLedStripConfigValues:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 5))
	case msp.MSP2CommonSerialConfig:
		payload := []byte{2}
		payload = append(payload, 20)
		payload = appendU32(payload, 1)
		payload = append(payload, 5, 0, 0, 5)
		payload = append(payload, 51)
		payload = appendU32(payload, 64|2)
		payload = append(payload, 5, 4, 0, 0)
		f.out.Write(response(frame.Code, payload, false))
	case msp.MSPSetCFSerialConfig:
		f.out.Write(response(frame.Code, nil, len(frame.Payload)%7 != 0))
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
	case msp.MSPSetOSDConfig:
		ok := false
		if len(frame.Payload) == 24 && frame.Payload[0] == 255 && frame.Payload[1] <= 3 {
			ok = true
		} else if len(frame.Payload) == 4 && frame.Payload[0] == 254 {
			ok = true
		} else if len(frame.Payload) == 4 {
			ok = true
		}
		f.out.Write(response(frame.Code, nil, !ok))
	case msp.MSPOSDCanvas:
		f.out.Write(response(frame.Code, []byte{53, 20}, false))
	case msp.MSPSetOSDCanvas:
		f.out.Write(response(frame.Code, nil, len(frame.Payload) != 2))
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

func fakeTextPayload(request []byte) ([]byte, bool) {
	if len(request) != 1 {
		return nil, false
	}
	values := map[byte]string{
		msp.MSP2TextPilotName:          "BF pilot",
		msp.MSP2TextCraftName:          "BetaFlight",
		msp.MSP2TextPIDProfileName:     "PID 1",
		msp.MSP2TextRateProfileName:    "Rate 1",
		msp.MSP2TextBuildKey:           "fake-build-key",
		msp.MSP2TextReleaseName:        "2025.12.1",
		msp.MSP2TextBatteryProfileName: "Battery 1",
	}
	value, ok := values[request[0]]
	if !ok {
		return nil, false
	}
	payload := []byte{request[0]}
	payload = appendPString(payload, value)
	return payload, true
}

func fakeTextSetPayloadValid(payload []byte) bool {
	if len(payload) < 2 {
		return false
	}
	textLength := int(payload[1])
	if len(payload) != textLength+2 {
		return false
	}
	switch payload[0] {
	case msp.MSP2TextPilotName, msp.MSP2TextCraftName:
		return textLength <= 16
	case msp.MSP2TextPIDProfileName, msp.MSP2TextRateProfileName, msp.MSP2TextBatteryProfileName:
		return textLength <= 8
	default:
		return false
	}
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

func fakeSimplifiedTuningPayload() []byte {
	payload := []byte{2, 100, 100, 100, 100, 100, 100, 100, 100}
	payload = appendU32(payload, 0)
	payload = appendU32(payload, 0)
	payload = append(payload, 1, 100)
	payload = appendU16(payload, 100)
	payload = appendU16(payload, 150)
	payload = appendU16(payload, 70)
	payload = appendU16(payload, 170)
	payload = appendU32(payload, 0)
	payload = appendU32(payload, 0)
	payload = append(payload, 1, 100)
	payload = appendU16(payload, 150)
	payload = appendU16(payload, 250)
	payload = appendU16(payload, 75)
	payload = appendU16(payload, 300)
	payload = appendU32(payload, 0)
	payload = appendU32(payload, 0)
	return payload
}

func fakeSimplifiedPIDCalculationPayload() []byte {
	payload := []byte{50, 85, 35, 45}
	payload = appendU16(payload, 120)
	payload = append(payload, 52, 87, 37, 47)
	payload = appendU16(payload, 125)
	payload = append(payload, 48, 80, 0, 0)
	payload = appendU16(payload, 110)
	return payload
}

func fakeSimplifiedDtermPayload() []byte {
	payload := []byte{1, 100}
	payload = appendU16(payload, 105)
	payload = appendU16(payload, 155)
	payload = appendU16(payload, 75)
	payload = appendU16(payload, 175)
	payload = appendU32(payload, 0)
	payload = appendU32(payload, 0)
	return payload
}

func fakeSimplifiedGyroPayload() []byte {
	payload := []byte{1, 100}
	payload = appendU16(payload, 155)
	payload = appendU16(payload, 255)
	payload = appendU16(payload, 80)
	payload = appendU16(payload, 305)
	payload = appendU32(payload, 0)
	payload = appendU32(payload, 0)
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

func fakeDataflashLogData() []byte {
	log := strings.Join([]string{
		"H Product:Blackbox flight data recorder by Nicholas Sherlock",
		"H Firmware revision:Betaflight 2025.12.1 (abc123) STM32F405",
		"H Field I name:loopIteration,time",
		"H Field I signed:0,0",
		"H Field I predictor:6,0",
		"H Field I encoding:1,1",
		"H Field P predictor:0,10",
		"H Field P encoding:0,0",
		"I\x02\x04E",
		"",
	}, "\n")
	data := bytes.Repeat([]byte(log), 1024)
	if len(data) < 262144 {
		padding := make([]byte, 262144-len(data))
		data = append(data, padding...)
	}
	return data
}
