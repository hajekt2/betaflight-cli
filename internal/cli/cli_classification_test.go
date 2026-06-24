package cli

import "testing"

func TestClassifyCLI(t *testing.T) {
	tests := []struct {
		command string
		want    cliClass
	}{
		{"diff all", cliReadOnly},
		{"dump all", cliReadOnly},
		{"get gyro_lpf1_static_hz", cliReadOnly},
		{"help", cliReadOnly},
		{"resource", cliReadOnly},
		{"resource show", cliReadOnly},
		{"resource list", cliReadOnly},
		{"resource serialrx 1 A06", cliWrite},
		{"set gyro_lpf1_static_hz = 0", cliWrite},
		{"beeper 1", cliDangerous},
		{"save", cliDangerous},
		{"defaults", cliDangerous},
		{"motor 0 1000", cliDangerous},
		{"    ", cliReadOnly},
	}
	for _, tt := range tests {
		if got := classifyCLI(tt.command); got != tt.want {
			t.Fatalf("classifyCLI(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestClassifyCLISequence(t *testing.T) {
	tests := []struct {
		command string
		want    cliClass
	}{
		{"diff all; status", cliReadOnly},
		{"diff all; set small_angle = 25", cliWrite},
		{"diff all; save", cliDangerous},
		{"get small_angle\nmotor 0 1100", cliDangerous},
		{" ; \n ", cliReadOnly},
	}
	for _, tt := range tests {
		if got := classifyCLISequence(tt.command); got != tt.want {
			t.Fatalf("classifyCLISequence(%q) = %v, want %v", tt.command, got, tt.want)
		}
	}
}

func TestIsKnownCLICommand(t *testing.T) {
	tests := map[string]bool{
		"set foo = 1":         true,
		"resource serialrx 1": true,
		"help":                true,
		"impossible line":     false,
		"":                    false,
		"   ":                 false,
	}
	for line, want := range tests {
		if got := isKnownCLICommand(line); got != want {
			t.Fatalf("isKnownCLICommand(%q) = %v, want %v", line, got, want)
		}
	}
}

func TestIsBatchAllowed(t *testing.T) {
	tests := map[string]bool{
		"set foo = 1":         true,
		"serial 0 1 1":        true,
		"resource serialrx 1": true,
		"save":                false,
		"diff all":            false,
		"reboot":              false,
		"feature GPS":         true,
		"beeper 1":            true,
		"map 1 2 3":           true,
		"timer 2":             true,
		"dma 1":               true,
	}
	for line, want := range tests {
		if got := isBatchAllowed(line); got != want {
			t.Fatalf("isBatchAllowed(%q) = %v, want %v", line, got, want)
		}
	}
}
