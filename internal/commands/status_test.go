package commands

import "testing"

func TestDecodeFlightModeState(t *testing.T) {
	status := &Status{Source: "MSP_STATUS_EX", ModeFlags: 0b1010}
	modes := []ModeDefinition{
		{ID: 0, Name: "ARM"},
		{ID: 1, Name: "ANGLE"},
		{ID: 2, Name: "HORIZON"},
		{ID: 3, Name: "ALTHOLD"},
	}
	state := DecodeFlightModeState(status, modes)
	if state == nil {
		t.Fatal("DecodeFlightModeState() = nil")
	}
	if len(state.ActiveNames) != 2 || state.ActiveNames[0] != "ANGLE" || state.ActiveNames[1] != "ALTHOLD" {
		t.Fatalf("active names = %+v", state.ActiveNames)
	}
	if len(state.ModeCatalog) != 4 || state.ModeCatalog[1].ID != 1 || !state.ModeCatalog[1].Active {
		t.Fatalf("mode catalog = %+v", state.ModeCatalog)
	}
	if state.UnknownMask != 0 {
		t.Fatalf("unknown mask = %d", state.UnknownMask)
	}
}

func TestDecodeFlightModeStateReportsUnknownMask(t *testing.T) {
	status := &Status{Source: "MSP_STATUS_EX", ModeFlags: 0b100}
	state := DecodeFlightModeState(status, []ModeDefinition{{ID: 0, Name: "ARM"}})
	if state.UnknownMask != 0b100 {
		t.Fatalf("unknown mask = %d", state.UnknownMask)
	}
}

func TestDecodeFlightModeStateUsesExtendedFlagBytes(t *testing.T) {
	status := &Status{
		Source:         "MSP_STATUS_EX",
		ModeFlags:      0,
		ModeFlagsBytes: make([]int, 5),
	}
	status.ModeFlagsBytes[4] = 1
	modes := make([]ModeDefinition, 33)
	for i := range modes {
		modes[i] = ModeDefinition{ID: uint8(i), Name: "MODE"}
	}
	modes[32].Name = "EXTENDED"
	state := DecodeFlightModeState(status, modes)
	if len(state.ActiveNames) != 1 || state.ActiveNames[0] != "EXTENDED" {
		t.Fatalf("active names = %+v", state.ActiveNames)
	}
	active := state.ActiveModes[0]
	if active.Bit != 32 || active.ByteIndex != 4 || active.BitIndex != 0 || active.Mask != 0 {
		t.Fatalf("active extended mode = %+v", active)
	}
}
