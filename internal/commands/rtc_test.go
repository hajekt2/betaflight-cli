package commands

import (
	"testing"
	"time"
)

func TestDecodeRTC(t *testing.T) {
	payload := appendU16Test(nil, 2026)
	payload = append(payload, 6, 23, 12, 34, 56)
	payload = appendU16Test(payload, 789)
	rtc, err := DecodeRTC(payload)
	if err != nil {
		t.Fatalf("DecodeRTC() error = %v", err)
	}
	if !rtc.Available || rtc.ISOUTC != "2026-06-23T12:34:56.789Z" || rtc.Millis != 789 {
		t.Fatalf("rtc = %+v", rtc)
	}
}

func TestDecodeRTCEmptyPayloadIsUnavailable(t *testing.T) {
	rtc, err := DecodeRTC(nil)
	if err != nil {
		t.Fatalf("DecodeRTC() error = %v", err)
	}
	if rtc.Available {
		t.Fatalf("rtc.Available = true")
	}
}

func TestDecodeRTCRejectsShortPayload(t *testing.T) {
	if _, err := DecodeRTC([]byte{1, 2}); err == nil {
		t.Fatal("DecodeRTC() error = nil, want short payload error")
	}
}

func TestDecodeRTCRejectsTrailingBytes(t *testing.T) {
	payload := appendU16Test(nil, 2026)
	payload = append(payload, 6, 23, 12, 34, 56)
	payload = appendU16Test(payload, 789)
	payload = append(payload, 0)
	if _, err := DecodeRTC(payload); err == nil {
		t.Fatal("DecodeRTC() error = nil, want trailing payload error")
	}
}

func TestDecodeRTCRejectsInvalidDate(t *testing.T) {
	payload := appendU16Test(nil, 2026)
	payload = append(payload, 13, 23, 12, 34, 56)
	payload = appendU16Test(payload, 789)
	if _, err := DecodeRTC(payload); err == nil {
		t.Fatal("DecodeRTC() error = nil, want invalid datetime error")
	}
}

func TestEncodeRTC(t *testing.T) {
	timestamp := time.Date(2026, 6, 23, 12, 34, 56, 789123000, time.UTC)
	payload, err := EncodeRTC(timestamp)
	if err != nil {
		t.Fatalf("EncodeRTC() error = %v", err)
	}
	rtc, err := DecodeRTC(payload)
	if err != nil {
		t.Fatalf("DecodeRTC(EncodeRTC()) error = %v", err)
	}
	if rtc.ISOUTC != "2026-06-23T12:34:56.789Z" {
		t.Fatalf("rtc = %+v", rtc)
	}
}

func TestEncodeRTCNormalizesUTC(t *testing.T) {
	location := time.FixedZone("test", 2*60*60)
	timestamp := time.Date(2026, 6, 23, 14, 34, 56, 123000000, location)
	payload, err := EncodeRTC(timestamp)
	if err != nil {
		t.Fatalf("EncodeRTC() error = %v", err)
	}
	rtc, err := DecodeRTC(payload)
	if err != nil {
		t.Fatalf("DecodeRTC(EncodeRTC()) error = %v", err)
	}
	if rtc.ISOUTC != "2026-06-23T12:34:56.123Z" {
		t.Fatalf("rtc = %+v", rtc)
	}
}
