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
				"set gyro_lpf1_static_hz = 0",
				"set osd_units = METRIC",
				"save",
			},
			"set gyro_lpf1_static_hz = 0":           {"gyro_lpf1_static_hz set to 0"},
			"feature GPS":                           {"Enabled GPS"},
			"feature -GPS":                          {"Disabled GPS"},
			"serial UART1 64 115200 57600 0 115200": {"serial updated"},
			"aux 0 0 0 1700 2100 0 0":               {"aux updated"},
			"resource MOTOR 1 A00":                  {"resource updated"},
			"profile 1":                             {"profile 1"},
			"rateprofile 2":                         {"rateprofile 2"},
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
	case msp.MSPStatusEx:
		payload := make([]byte, 0, 32)
		payload = appendU16(payload, 250)
		payload = appendU16(payload, 0)
		payload = appendU16(payload, 33)
		payload = appendU32(payload, 2)
		payload = append(payload, 0)
		payload = appendU16(payload, 42)
		payload = append(payload, 4, 0, 0, 29)
		payload = appendU32(payload, 0x1234)
		payload = append(payload, 0)
		payload = appendU16(payload, 425)
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

func appendU32(dst []byte, v uint32) []byte {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], v)
	return append(dst, buf[:]...)
}
