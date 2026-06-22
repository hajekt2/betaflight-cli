package connection

import "testing"

func TestClassifyPortIgnoresDebugAndBluetooth(t *testing.T) {
	tests := []string{"/dev/cu.Bluetooth-Incoming-Port", "/dev/cu.debug-console"}
	for _, name := range tests {
		candidate, _ := classifyPort(name)
		if candidate {
			t.Fatalf("classifyPort(%q) candidate = true, want false", name)
		}
	}
}
