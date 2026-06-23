package msp

import "testing"

func TestLookupCommandByName(t *testing.T) {
	meta, ok := LookupCommandByName("msp_name")
	if !ok {
		t.Fatalf("lookup failed")
	}
	if meta.Code != MSPName || meta.Name != "MSP_NAME" {
		t.Fatalf("meta = %+v", meta)
	}
	if _, ok := LookupCommandByName("does_not_exist"); ok {
		t.Fatal("unexpected lookup success")
	}
}

