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

func TestListCommandsSortedByCode(t *testing.T) {
	commands := ListCommands()
	if len(commands) == 0 {
		t.Fatal("expected commands")
	}
	for i := 1; i < len(commands); i++ {
		if commands[i-1].Code > commands[i].Code {
			t.Fatalf("commands not sorted by code: index %d code %d > index %d code %d", i-1, commands[i-1].Code, i, commands[i].Code)
		}
	}
}
