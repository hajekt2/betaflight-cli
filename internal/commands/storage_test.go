package commands

import "testing"

func TestDecodeDataflashSummary(t *testing.T) {
	payload := []byte{3}
	payload = appendU32Test(payload, 16)
	payload = appendU32Test(payload, 1048576)
	payload = appendU32Test(payload, 262144)
	summary, err := DecodeDataflashSummary(payload)
	if err != nil {
		t.Fatalf("DecodeDataflashSummary() error = %v", err)
	}
	if !summary.Ready || !summary.Supported || summary.Sectors != 16 || summary.TotalBytes != 1048576 || summary.UsedBytes != 262144 || summary.FreeBytes != 786432 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestDecodeDataflashSummaryRejectsShortPayload(t *testing.T) {
	if _, err := DecodeDataflashSummary([]byte{1, 2}); err == nil {
		t.Fatal("DecodeDataflashSummary() error = nil, want short payload error")
	}
}

func TestDecodeSDCardSummary(t *testing.T) {
	payload := []byte{1, 4, 0}
	payload = appendU32Test(payload, 4096)
	payload = appendU32Test(payload, 32768)
	summary, err := DecodeSDCardSummary(payload)
	if err != nil {
		t.Fatalf("DecodeSDCardSummary() error = %v", err)
	}
	if !summary.Supported || summary.State != 4 || summary.StateName != "READY" || summary.FreeKilobytes != 4096 || summary.TotalKilobytes != 32768 {
		t.Fatalf("summary = %+v", summary)
	}
}

func TestDecodeSDCardSummaryRejectsShortPayload(t *testing.T) {
	if _, err := DecodeSDCardSummary([]byte{1, 2}); err == nil {
		t.Fatal("DecodeSDCardSummary() error = nil, want short payload error")
	}
}
