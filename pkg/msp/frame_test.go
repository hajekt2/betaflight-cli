package msp

import (
	"bytes"
	"encoding/hex"
	"testing"
)

func TestEncodeRequestV1(t *testing.T) {
	got := EncodeRequest(MSPStatus, nil)
	want := []byte{'$', 'M', '<', 0, byte(MSPStatus), byte(MSPStatus)}
	if !bytes.Equal(got, want) {
		t.Fatalf("EncodeRequest() = %s, want %s", hex.EncodeToString(got), hex.EncodeToString(want))
	}
}

func TestReadFrameV1(t *testing.T) {
	frameBytes := EncodeRequest(MSPAttitude, []byte{1, 2})
	frameBytes[2] = '>'
	frame, err := ReadFrame(bytes.NewReader(frameBytes))
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if frame.Version != 1 || frame.Code != MSPAttitude || !bytes.Equal(frame.Payload, []byte{1, 2}) {
		t.Fatalf("unexpected frame: %+v", frame)
	}
}

func TestEncodeReadFrameV2(t *testing.T) {
	frameBytes := EncodeRequest(MSP2CLISettingInfo, []byte{0xaa, 0xbb})
	frameBytes[2] = '>'
	frame, err := ReadFrame(bytes.NewReader(frameBytes))
	if err != nil {
		t.Fatalf("ReadFrame() error = %v", err)
	}
	if frame.Version != 2 || frame.Code != MSP2CLISettingInfo || !bytes.Equal(frame.Payload, []byte{0xaa, 0xbb}) {
		t.Fatalf("unexpected frame: %+v", frame)
	}
}

func TestReadFrameRejectsBadChecksum(t *testing.T) {
	frameBytes := EncodeRequest(MSPStatus, []byte{1})
	frameBytes[2] = '>'
	frameBytes[len(frameBytes)-1] ^= 0xff
	if _, err := ReadFrame(bytes.NewReader(frameBytes)); err == nil {
		t.Fatal("ReadFrame() error = nil, want checksum error")
	}
}
