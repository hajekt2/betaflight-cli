package commands

import "testing"

func TestDecodeTransponderConfig(t *testing.T) {
	config, err := DecodeTransponderConfig([]byte{3, 1, 6, 2, 9, 3, 1, 2, 0x12, 0x34, 0x56, 0x78, 0x9a, 0xbc, 0xde, 0xf0, 0x42})
	if err != nil {
		t.Fatalf("DecodeTransponderConfig() error = %v", err)
	}
	if config.Source != "MSP_TRANSPONDER_CONFIG" || !config.Available || config.Provider != 2 || config.ProviderName != "ARCITIMER" {
		t.Fatalf("config = %+v", config)
	}
	if len(config.Providers) != 3 || config.Providers[0].Name != "ILAP" || config.Providers[1].DataLength != 9 || config.Providers[2].Name != "ERLT" {
		t.Fatalf("providers = %+v", config.Providers)
	}
	if config.DataHex != "123456789ABCDEF042" || len(config.Data) != 9 || config.Data[8] != 0x42 {
		t.Fatalf("data = %x %q", config.Data, config.DataHex)
	}
	if len(config.CLICommands) != 2 || config.CLICommands[0] != "set transponder_provider = ARCITIMER" || config.CLICommands[1] != "set transponder_data = 18,52,86,120,154,188,222,240,66" {
		t.Fatalf("cli commands = %+v", config.CLICommands)
	}
}

func TestDecodeTransponderConfigDisabled(t *testing.T) {
	config, err := DecodeTransponderConfig([]byte{1, 1, 6, 0})
	if err != nil {
		t.Fatalf("DecodeTransponderConfig() error = %v", err)
	}
	if config.ProviderName != "NONE" || len(config.Data) != 0 || len(config.CLICommands) != 1 {
		t.Fatalf("config = %+v", config)
	}
}

func TestDecodeTransponderConfigUnavailable(t *testing.T) {
	config, err := DecodeTransponderConfig(nil)
	if err != nil {
		t.Fatalf("DecodeTransponderConfig() error = %v", err)
	}
	if config.Available || config.ProviderName != "" || len(config.Warnings) != 1 || len(config.CLICommands) != 1 {
		t.Fatalf("config = %+v", config)
	}
}

func TestDecodeTransponderConfigUnavailableProviderCount(t *testing.T) {
	config, err := DecodeTransponderConfig([]byte{0})
	if err != nil {
		t.Fatalf("DecodeTransponderConfig() error = %v", err)
	}
	if config.Available || len(config.Warnings) != 1 {
		t.Fatalf("config = %+v", config)
	}
}

func TestDecodeTransponderConfigRejectsUnknownActiveProvider(t *testing.T) {
	if _, err := DecodeTransponderConfig([]byte{1, 1, 6, 3}); err == nil {
		t.Fatal("DecodeTransponderConfig() error = nil, want unknown provider error")
	}
}

func TestDecodeTransponderConfigRejectsTrailingBytes(t *testing.T) {
	if _, err := DecodeTransponderConfig([]byte{1, 1, 6, 1, 1, 2, 3}); err == nil {
		t.Fatal("DecodeTransponderConfig() error = nil, want trailing byte error")
	}
}

func TestValidateTransponderProviderName(t *testing.T) {
	for _, tc := range []struct {
		input string
		want  string
	}{
		{"arcitimer", "ARCITIMER"},
		{"ILAP", "ILAP"},
		{"0", "0"},
		{"3", "3"},
	} {
		got, err := ValidateTransponderProviderName(tc.input)
		if err != nil || got != tc.want {
			t.Fatalf("ValidateTransponderProviderName(%q) = %q, %v, want %q", tc.input, got, err, tc.want)
		}
	}
	if _, err := ValidateTransponderProviderName("UNKNOWN"); err == nil {
		t.Fatal("ValidateTransponderProviderName(\"UNKNOWN\") = nil, want error")
	}
}

func TestValidateTransponderData(t *testing.T) {
	data, err := ValidateTransponderData("1,2,3,255")
	if err != nil {
		t.Fatalf("ValidateTransponderData() error = %v", err)
	}
	if len(data) != 4 || data[0] != 1 || data[3] != 255 {
		t.Fatalf("data = %+v", data)
	}
	if _, err := ValidateTransponderData("1,,2"); err == nil {
		t.Fatal("ValidateTransponderData(\"1,,2\") error = nil, want error")
	}
	if _, err := ValidateTransponderData("1,256"); err == nil {
		t.Fatal("ValidateTransponderData(\"1,256\") error = nil, want error")
	}
	if _, err := ValidateTransponderData("bad"); err == nil {
		t.Fatal("ValidateTransponderData(\"bad\") error = nil, want error")
	}
}

func TestEncodeTransponderConfig(t *testing.T) {
	payload := EncodeTransponderConfig(2, []uint8{0x12, 0x34, 0x56})
	want := []byte{2, 0x12, 0x34, 0x56}
	if string(payload) != string(want) {
		t.Fatalf("payload = %v, want %v", payload, want)
	}
}
