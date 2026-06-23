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
