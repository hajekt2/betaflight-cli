package commands

import "testing"

func TestDecodeFirmwareSetting(t *testing.T) {
	setting, err := DecodeFirmwareSetting([]byte("gyro_lpf1_static_hz = 42"))
	if err != nil {
		t.Fatalf("DecodeFirmwareSetting() error = %v", err)
	}
	if !setting.Supported || setting.Name != "gyro_lpf1_static_hz" || setting.Value != "42" || setting.Source != "MSP2_CLI_SETTING" {
		t.Fatalf("setting = %+v", setting)
	}
}

func TestDecodeFirmwareSettingInfo(t *testing.T) {
	payload := appendU16Payload(nil, 12)
	payload = append(payload, []byte("hello world!")...)
	info, err := DecodeFirmwareSettingInfo(payload)
	if err != nil {
		t.Fatalf("DecodeFirmwareSettingInfo() error = %v", err)
	}
	if !info.Supported || info.TotalBytes != 12 || info.ChunkBytes != 12 || !info.Complete || info.Text != "hello world!" {
		t.Fatalf("info = %+v", info)
	}
}

func TestDecodeFirmwareSettingInfoRejectsShortPayload(t *testing.T) {
	if _, err := DecodeFirmwareSettingInfo([]byte{1}); err == nil {
		t.Fatal("DecodeFirmwareSettingInfo() error = nil")
	}
}
