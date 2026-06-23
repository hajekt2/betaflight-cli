package commands

import "testing"

func TestDecodeBuildInfo(t *testing.T) {
	payload := []byte("Jan 01 2026")
	payload = append(payload, []byte("12:34:56")...)
	payload = append(payload, []byte("abc1234")...)
	payload = appendU16Test(payload, 16412)
	payload = appendU16Test(payload, 8231)
	payload = appendU16Test(payload, 65000)
	info, err := DecodeBuildInfo(payload)
	if err != nil {
		t.Fatalf("DecodeBuildInfo() error = %v", err)
	}
	if info.Date != "Jan 01 2026" || info.Time != "12:34:56" || info.DateTime != "Jan 01 2026 12:34:56" || info.GitRevision != "abc1234" {
		t.Fatalf("info = %+v", info)
	}
	if len(info.BuildOptions) != 3 || len(info.BuildOptionCodes) != 3 {
		t.Fatalf("build options = %+v", info.BuildOptions)
	}
	if len(info.BuildOptionNames) != 2 || info.BuildOptionNames[0] != "USE_GPS" || info.BuildOptionNames[1] != "USE_DSHOT" {
		t.Fatalf("build option names = %+v", info.BuildOptionNames)
	}
	if len(info.UnknownOptions) != 1 || info.UnknownOptions[0].Code != 65000 {
		t.Fatalf("unknown options = %+v", info.UnknownOptions)
	}
}

func TestDecodeMCUInfo(t *testing.T) {
	info, err := DecodeMCUInfo([]byte{254, 9, 'S', 'T', 'M', '3', '2', 'F', '4', '0', '5'})
	if err != nil {
		t.Fatalf("DecodeMCUInfo() error = %v", err)
	}
	if info.Source != "MSP2_MCU_INFO" || info.ID != 254 || info.Name != "STM32F405" {
		t.Fatalf("info = %+v", info)
	}
}

func TestDecodeMCUInfoRejectsTrailingBytes(t *testing.T) {
	if _, err := DecodeMCUInfo([]byte{254, 0, 1}); err == nil {
		t.Fatal("DecodeMCUInfo() error = nil, want trailing byte error")
	}
}

func TestDecodeDeviceUID(t *testing.T) {
	payload := appendU32Test(nil, 0x01234567)
	payload = appendU32Test(payload, 0x89abcdef)
	payload = appendU32Test(payload, 0x00000042)
	uid, err := DecodeDeviceUID(payload)
	if err != nil {
		t.Fatalf("DecodeDeviceUID() error = %v", err)
	}
	if uid.Source != "MSP_UID" || uid.Hex != "0123456789abcdef00000042" || uid.ConfiguratorIdentifier != "123456789abcdef42" {
		t.Fatalf("uid = %+v", uid)
	}
	if len(uid.Words) != 3 || uid.Words[0] != 0x01234567 || uid.Words[2] != 0x00000042 {
		t.Fatalf("uid words = %+v", uid.Words)
	}
}

func TestDecodeDeviceUIDRejectsShortPayload(t *testing.T) {
	if _, err := DecodeDeviceUID([]byte{1, 2, 3}); err == nil {
		t.Fatal("DecodeDeviceUID() error = nil, want short payload error")
	}
}

func TestDecodeDeviceUIDRejectsTrailingBytes(t *testing.T) {
	payload := appendU32Test(nil, 1)
	payload = appendU32Test(payload, 2)
	payload = appendU32Test(payload, 3)
	payload = append(payload, 4)
	if _, err := DecodeDeviceUID(payload); err == nil {
		t.Fatal("DecodeDeviceUID() error = nil, want trailing byte error")
	}
}

func TestDecodeBuildInfoRejectsShortPayload(t *testing.T) {
	if _, err := DecodeBuildInfo([]byte("too short")); err == nil {
		t.Fatal("DecodeBuildInfo() error = nil, want short payload error")
	}
}

func TestDecodeBuildInfoRejectsOddTrailingOptionByte(t *testing.T) {
	payload := []byte("Jan 01 2026")
	payload = append(payload, []byte("12:34:56")...)
	payload = append(payload, []byte("abc1234")...)
	payload = append(payload, 1)
	if _, err := DecodeBuildInfo(payload); err == nil {
		t.Fatal("DecodeBuildInfo() error = nil, want odd trailing byte error")
	}
}

func TestDecodeNameTrimsTrailingNUL(t *testing.T) {
	if got := DecodeName([]byte{'Q', 'u', 'a', 'd', 0, 0}); got != "Quad" {
		t.Fatalf("DecodeName() = %q", got)
	}
}
