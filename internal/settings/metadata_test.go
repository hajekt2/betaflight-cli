package settings

import "testing"

func TestLookupAndValidate(t *testing.T) {
	setting, ok := DefaultRegistry.Lookup("dshot_bidir")
	if !ok {
		t.Fatal("dshot_bidir not found")
	}
	if err := setting.Validate("ON"); err != nil {
		t.Fatalf("Validate(ON) error = %v", err)
	}
	if err := setting.Validate("MAYBE"); err == nil {
		t.Fatal("Validate(MAYBE) error = nil, want error")
	}
}

func TestRangeValidation(t *testing.T) {
	setting, ok := DefaultRegistry.Lookup("small_angle")
	if !ok {
		t.Fatal("small_angle not found")
	}
	if err := setting.Validate("181"); err == nil {
		t.Fatal("Validate(181) error = nil, want range error")
	}
}
