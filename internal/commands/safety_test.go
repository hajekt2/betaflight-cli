package commands

import "testing"

func TestDecodeSafetyConfigs(t *testing.T) {
	gyroCal := true
	arming, err := DecodeArmingConfig([]byte{5, 0, 25, 1})
	if err != nil {
		t.Fatalf("DecodeArmingConfig() error = %v", err)
	}
	if arming.AutoDisarmDelayS != 5 || arming.SmallAngleDegrees != 25 || arming.GyroCalOnFirstArm == nil || *arming.GyroCalOnFirstArm != gyroCal {
		t.Fatalf("arming = %+v", arming)
	}

	failsafePayload := []byte{15, 60}
	failsafePayload = appendU16Test(failsafePayload, 1000)
	failsafePayload = append(failsafePayload, 2)
	failsafePayload = appendU16Test(failsafePayload, 100)
	failsafePayload = append(failsafePayload, 1)
	failsafe, err := DecodeFailsafeConfig(failsafePayload)
	if err != nil {
		t.Fatalf("DecodeFailsafeConfig() error = %v", err)
	}
	if failsafe.DelayTenthsS != 15 || failsafe.SwitchModeName != "STAGE2" || failsafe.ProcedureName != "DROP" {
		t.Fatalf("failsafe = %+v", failsafe)
	}

	boardPayload := appendS16Test(nil, -2)
	boardPayload = appendS16Test(boardPayload, 3)
	boardPayload = appendS16Test(boardPayload, 90)
	board, err := DecodeBoardAlignment(boardPayload)
	if err != nil {
		t.Fatalf("DecodeBoardAlignment() error = %v", err)
	}
	if board.RollDegrees != -2 || board.PitchDegrees != 3 || board.YawDegrees != 90 {
		t.Fatalf("board = %+v", board)
	}
}

func TestDecodeArmingDisableState(t *testing.T) {
	count := uint8(len(armingDisableFlagNames))
	flags := uint32(0x1234)
	configState := uint8(1)
	state := DecodeArmingDisableState(&Status{
		Source:             "MSP_STATUS_EX",
		ArmingDisableCount: &count,
		ArmingDisableFlags: &flags,
		ConfigStateFlag:    &configState,
	})
	if state == nil {
		t.Fatal("DecodeArmingDisableState() = nil")
	}
	if !state.Disabled || state.FlagCount != count || len(state.FlagCatalog) != int(count) {
		t.Fatalf("state = %+v", state)
	}
	want := []string{"RXLOSS", "BOXFAILSAFE", "RUNAWAY", "BOOTGRACE", "CALIB"}
	if len(state.ActiveNames) != len(want) {
		t.Fatalf("active names = %+v", state.ActiveNames)
	}
	for i, name := range want {
		if state.ActiveNames[i] != name {
			t.Fatalf("active names = %+v, want %v", state.ActiveNames, want)
		}
	}
	if state.UnknownMask != 0 {
		t.Fatalf("unknown mask = %d", state.UnknownMask)
	}

	count = uint8(len(armingDisableFlagNames) + 1)
	flags = uint32(1) << uint(len(armingDisableFlagNames))
	state = DecodeArmingDisableState(&Status{
		Source:             "MSP_STATUS_EX",
		ArmingDisableCount: &count,
		ArmingDisableFlags: &flags,
	})
	if len(state.ActiveNames) != 1 || state.ActiveNames[0] != "UNKNOWN_29" || state.UnknownMask != flags {
		t.Fatalf("future flag state = %+v", state)
	}
}

func TestDecodeSafetyRejectsShortPayloads(t *testing.T) {
	if _, err := DecodeArmingConfig([]byte{1, 2}); err == nil {
		t.Fatal("DecodeArmingConfig() error = nil, want short payload error")
	}
	if _, err := DecodeFailsafeConfig([]byte{1, 2}); err == nil {
		t.Fatal("DecodeFailsafeConfig() error = nil, want short payload error")
	}
	if _, err := DecodeBoardAlignment([]byte{1, 2}); err == nil {
		t.Fatal("DecodeBoardAlignment() error = nil, want short payload error")
	}
}
