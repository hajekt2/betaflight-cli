package commands

import "testing"

func TestDecodeSerialPortConfigV2(t *testing.T) {
	payload := []byte{2}
	payload = append(payload, 20)
	payload = appendU32Test(payload, 1)
	payload = append(payload, 5, 0, 0, 5)
	payload = append(payload, 51)
	payload = appendU32Test(payload, 64|2)
	payload = append(payload, 5, 4, 0, 0)
	ports, err := DecodeSerialPortConfigV2(payload)
	if err != nil {
		t.Fatalf("DecodeSerialPortConfigV2() error = %v", err)
	}
	if len(ports) != 2 {
		t.Fatalf("ports = %+v", ports)
	}
	if ports[0].IdentifierName != "USB_VCP" || ports[0].Functions[0] != "MSP" || ports[0].MSPBaudRate != "115200" {
		t.Fatalf("port 0 = %+v", ports[0])
	}
	if ports[1].IdentifierName != "UART1" || ports[1].FunctionMask != 66 || ports[1].GPSBaudRate != "57600" {
		t.Fatalf("port 1 = %+v", ports[1])
	}
}

func TestDecodeSerialPortConfigV1(t *testing.T) {
	payload := []byte{51}
	payload = appendU16Test(payload, 64)
	payload = append(payload, 5, 4, 0, 0)
	ports, err := DecodeSerialPortConfigV1(payload)
	if err != nil {
		t.Fatalf("DecodeSerialPortConfigV1() error = %v", err)
	}
	if len(ports) != 1 || ports[0].IdentifierName != "UART1" || ports[0].Functions[0] != "RX_SERIAL" {
		t.Fatalf("ports = %+v", ports)
	}
}

func TestEncodeSerialPortConfigV1(t *testing.T) {
	payload := EncodeSerialPortConfigV1([]SerialPort{{
		Identifier:         51,
		FunctionMask:       64 | 2,
		MSPBaudRateIndex:   5,
		GPSBaudRateIndex:   4,
		TelemetryBaudIndex: 0,
		BlackboxBaudIndex:  0,
	}})
	want := []byte{51, 66, 0, 5, 4, 0, 0}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}

// Captured-frame style fixture for MSP2_COMMON_SET_SERIAL_CONFIG: one U8 port
// count followed by per-port records exactly as upstream msp.c writes them
// (identifier U8, functionMask U32 LE, four baud-index U8).
func TestEncodeCommonSerialConfig(t *testing.T) {
	records := []CommonSerialPortRecord{
		{Identifier: 20, FunctionMask: 1, MSPBaudRateIndex: 5, GPSBaudRateIndex: 0, TelemetryBaudIndex: 0, BlackboxBaudIndex: 5},
		{Identifier: 51, FunctionMask: 0x000F0042, MSPBaudRateIndex: 5, GPSBaudRateIndex: 4, TelemetryBaudIndex: 2, BlackboxBaudIndex: 0},
	}
	want := []byte{
		2,
		20, 0x01, 0x00, 0x00, 0x00, 5, 0, 0, 5,
		51, 0x42, 0x00, 0x0F, 0x00, 5, 4, 2, 0,
	}
	payload := EncodeCommonSerialConfig(records)
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}
