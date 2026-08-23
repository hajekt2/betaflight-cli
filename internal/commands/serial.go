package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/bfserial"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type SerialPortStatus struct {
	Source string       `json:"source"`
	Ports  []SerialPort `json:"ports"`
}

type SerialPort struct {
	Identifier         uint8    `json:"identifier"`
	IdentifierName     string   `json:"identifier_name,omitempty"`
	FunctionMask       uint32   `json:"function_mask"`
	Functions          []string `json:"functions"`
	MSPBaudRateIndex   uint8    `json:"msp_baudrate_index"`
	MSPBaudRate        string   `json:"msp_baudrate,omitempty"`
	GPSBaudRateIndex   uint8    `json:"gps_baudrate_index"`
	GPSBaudRate        string   `json:"gps_baudrate,omitempty"`
	TelemetryBaudIndex uint8    `json:"telemetry_baudrate_index"`
	TelemetryBaudRate  string   `json:"telemetry_baudrate,omitempty"`
	BlackboxBaudIndex  uint8    `json:"blackbox_baudrate_index"`
	BlackboxBaudRate   string   `json:"blackbox_baudrate,omitempty"`
}

type SerialPortConfigSetResult struct {
	Ports        []SerialPort `json:"ports"`
	MSPCode      uint16       `json:"msp_code"`
	MSPName      string       `json:"msp_name"`
	Acknowledged bool         `json:"acknowledged"`
	SaveRequired bool         `json:"save_required"`
}

func ReadSerialPortStatus(ctx context.Context, client *connection.Client) (*SerialPortStatus, []string, error) {
	frame, err := client.Request(ctx, msp.MSP2CommonSerialConfig, nil)
	if err == nil {
		ports, decodeErr := DecodeSerialPortConfigV2(frame.Payload)
		if decodeErr != nil {
			return nil, nil, fmt.Errorf("serial config decode failed: %w", decodeErr)
		}
		return &SerialPortStatus{Source: "MSP2_COMMON_SERIAL_CONFIG", Ports: ports}, nil, nil
	}
	warnings := []string{fmt.Sprintf("MSP2_COMMON_SERIAL_CONFIG unavailable: %v", err)}
	frame, err = client.Request(ctx, msp.MSPCFSerialConfig, nil)
	if err != nil {
		return nil, warnings, fmt.Errorf("legacy serial config unavailable: %w", err)
	}
	ports, decodeErr := DecodeSerialPortConfigV1(frame.Payload)
	if decodeErr != nil {
		return nil, warnings, fmt.Errorf("legacy serial config decode failed: %w", decodeErr)
	}
	return &SerialPortStatus{Source: "MSP_CF_SERIAL_CONFIG", Ports: ports}, warnings, nil
}

func SetSerialPortConfig(ctx context.Context, client *connection.Client, ports []SerialPort) (*SerialPortConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetCFSerialConfig, EncodeSerialPortConfigV1(ports)); err != nil {
		return nil, fmt.Errorf("serial port config request failed: %w", err)
	}
	copied := make([]SerialPort, len(ports))
	for i, port := range ports {
		copied[i] = serialPortWithDecodedNames(port)
	}
	return &SerialPortConfigSetResult{
		Ports:        copied,
		MSPCode:      msp.MSPSetCFSerialConfig,
		MSPName:      "MSP_SET_CF_SERIAL_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeSerialPortConfigV1(ports []SerialPort) []byte {
	payload := make([]byte, 0, len(ports)*7)
	for _, port := range ports {
		payload = append(payload, port.Identifier)
		payload = appendU16Payload(payload, uint16(port.FunctionMask))
		payload = append(payload, port.MSPBaudRateIndex, port.GPSBaudRateIndex, port.TelemetryBaudIndex, port.BlackboxBaudIndex)
	}
	return payload
}

func DecodeSerialPortConfigV2(payload []byte) ([]SerialPort, error) {
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return []SerialPort{}, nil
	}
	if r.Remaining()%int(count) != 0 {
		return nil, fmt.Errorf("payload has %d bytes for %d serial ports", r.Remaining(), count)
	}
	size := r.Remaining() / int(count)
	if size < 9 {
		return nil, fmt.Errorf("serial port entry size %d is smaller than MSPv2 minimum 9", size)
	}
	ports := make([]SerialPort, 0, count)
	for i := 0; i < int(count); i++ {
		startRemaining := r.Remaining()
		identifier, err := r.U8()
		if err != nil {
			return nil, err
		}
		mask, err := r.U32()
		if err != nil {
			return nil, err
		}
		port, err := readSerialPortBauds(r, identifier, mask)
		if err != nil {
			return nil, err
		}
		for startRemaining-r.Remaining() < size && r.Remaining() > 0 {
			if _, err := r.U8(); err != nil {
				return nil, err
			}
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func DecodeSerialPortConfigV1(payload []byte) ([]SerialPort, error) {
	if len(payload)%7 != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of legacy serial port size 7", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	ports := make([]SerialPort, 0, len(payload)/7)
	for r.Remaining() > 0 {
		identifier, err := r.U8()
		if err != nil {
			return nil, err
		}
		mask, err := r.U16()
		if err != nil {
			return nil, err
		}
		port, err := readSerialPortBauds(r, identifier, uint32(mask))
		if err != nil {
			return nil, err
		}
		ports = append(ports, port)
	}
	return ports, nil
}

func readSerialPortBauds(r *msp.PayloadReader, identifier uint8, mask uint32) (SerialPort, error) {
	mspBaud, err := r.U8()
	if err != nil {
		return SerialPort{}, err
	}
	gpsBaud, err := r.U8()
	if err != nil {
		return SerialPort{}, err
	}
	telemetryBaud, err := r.U8()
	if err != nil {
		return SerialPort{}, err
	}
	blackboxBaud, err := r.U8()
	if err != nil {
		return SerialPort{}, err
	}
	return SerialPort{
		Identifier:         identifier,
		IdentifierName:     bfserial.PortIdentifierName(identifier),
		FunctionMask:       mask,
		Functions:          bfserial.FunctionNames(mask),
		MSPBaudRateIndex:   mspBaud,
		MSPBaudRate:        bfserial.BaudRateName(mspBaud),
		GPSBaudRateIndex:   gpsBaud,
		GPSBaudRate:        bfserial.BaudRateName(gpsBaud),
		TelemetryBaudIndex: telemetryBaud,
		TelemetryBaudRate:  bfserial.BaudRateName(telemetryBaud),
		BlackboxBaudIndex:  blackboxBaud,
		BlackboxBaudRate:   bfserial.BaudRateName(blackboxBaud),
	}, nil
}

func serialPortWithDecodedNames(port SerialPort) SerialPort {
	port.IdentifierName = bfserial.PortIdentifierName(port.Identifier)
	port.Functions = bfserial.FunctionNames(port.FunctionMask)
	port.MSPBaudRate = bfserial.BaudRateName(port.MSPBaudRateIndex)
	port.GPSBaudRate = bfserial.BaudRateName(port.GPSBaudRateIndex)
	port.TelemetryBaudRate = bfserial.BaudRateName(port.TelemetryBaudIndex)
	port.BlackboxBaudRate = bfserial.BaudRateName(port.BlackboxBaudIndex)
	return port
}

// CommonSerialPortRecord is one per-port record of the MSP v2 common serial
// config payload. Layout per upstream Betaflight 2026.6.1
// src/main/msp/msp.c: MSP2_COMMON_SERIAL_CONFIG read handler (sbufWriteU8
// identifier, sbufWriteU32 functionMask little-endian, then msp_baudrateIndex,
// gps_baudrateIndex, telemetry_baudrateIndex, blackbox_baudrateIndex as U8)
// and MSP2_COMMON_SET_SERIAL_CONFIG write handler (same order; minimum record
// size 9 bytes, trailing unknown bytes are skipped by the FC).
type CommonSerialPortRecord struct {
	Identifier         uint8  `json:"identifier"`
	FunctionMask       uint32 `json:"function_mask"`
	MSPBaudRateIndex   uint8  `json:"msp_baudrate_index"`
	GPSBaudRateIndex   uint8  `json:"gps_baudrate_index"`
	TelemetryBaudIndex uint8  `json:"telemetry_baudrate_index"`
	BlackboxBaudIndex  uint8  `json:"blackbox_baudrate_index"`
}

type CommonSerialConfigSetResult struct {
	Ports        []CommonSerialPortRecord `json:"ports"`
	MSPCode      uint16                   `json:"msp_code"`
	MSPName      string                   `json:"msp_name"`
	Acknowledged bool                     `json:"acknowledged"`
	SaveRequired bool                     `json:"save_required"`
}

// EncodeCommonSerialConfig renders the MSP2_COMMON_SET_SERIAL_CONFIG payload:
// one U8 port count followed by per-port records (identifier U8, functionMask
// U32 LE, four baud indexes U8) exactly as parsed by upstream msp.c.
func EncodeCommonSerialConfig(records []CommonSerialPortRecord) []byte {
	payload := make([]byte, 0, len(records)*9)
	payload = append(payload, byte(len(records)))
	for _, record := range records {
		payload = append(payload, record.Identifier)
		payload = appendU32Payload(payload, record.FunctionMask)
		payload = append(payload, record.MSPBaudRateIndex, record.GPSBaudRateIndex, record.TelemetryBaudIndex, record.BlackboxBaudIndex)
	}
	return payload
}

// SetCommonSerialConfig sends the full replacement port table through
// MSP2_COMMON_SET_SERIAL_CONFIG. Upstream applies each record immediately;
// persisting still requires an MSP_EEPROM_WRITE afterwards.
func SetCommonSerialConfig(ctx context.Context, client *connection.Client, records []CommonSerialPortRecord) (*CommonSerialConfigSetResult, error) {
	if _, err := client.Request(ctx, msp.MSP2CommonSetSerialConfig, EncodeCommonSerialConfig(records)); err != nil {
		return nil, fmt.Errorf("common serial config request failed: %w", err)
	}
	copied := make([]CommonSerialPortRecord, len(records))
	copy(copied, records)
	return &CommonSerialConfigSetResult{
		Ports:        copied,
		MSPCode:      msp.MSP2CommonSetSerialConfig,
		MSPName:      "MSP2_COMMON_SET_SERIAL_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}
