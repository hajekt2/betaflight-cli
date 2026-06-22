package bfserial

import "testing"

func TestFunctionNames(t *testing.T) {
	names := FunctionNames((1 << 0) | (1 << 6) | (1 << 17))
	if len(names) != 3 || names[0] != "MSP" || names[1] != "RX_SERIAL" || names[2] != "VTX_MSP" {
		t.Fatalf("names = %+v", names)
	}
	none := FunctionNames(0)
	if len(none) != 1 || none[0] != "NONE" {
		t.Fatalf("none = %+v", none)
	}
}

func TestBaudRateName(t *testing.T) {
	if BaudRateName(5) != "115200" || BaudRateName(16) != "" {
		t.Fatalf("baud names: %q %q", BaudRateName(5), BaudRateName(16))
	}
}

func TestPortIdentifierName(t *testing.T) {
	if PortIdentifierName(20) != "USB_VCP" || PortIdentifierName(51) != "UART1" || PortIdentifierName(30) != "SOFTSERIAL1" {
		t.Fatalf("port names: %q %q %q", PortIdentifierName(20), PortIdentifierName(51), PortIdentifierName(30))
	}
}
