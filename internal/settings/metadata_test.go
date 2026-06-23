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

func TestValidateSkipsIncompleteLookupMetadata(t *testing.T) {
	setting := Metadata{Name: "acc_hardware", Type: TypeLookup, LookupTable: "TABLE_ACC_HARDWARE"}
	if err := setting.Validate("AUTO"); err != nil {
		t.Fatalf("Validate(AUTO) error = %v", err)
	}
}

func TestValidateSkipsArrayMetadata(t *testing.T) {
	setting := Metadata{Name: "acc_calibration", Type: TypeInt, Mode: "array"}
	if err := setting.Validate("0,0,0,0"); err != nil {
		t.Fatalf("Validate(array) error = %v", err)
	}
}

func TestValidateAcceptsBitsetText(t *testing.T) {
	setting := Metadata{Name: "blackbox_disable_pids", Type: TypeUint, Mode: "bitset"}
	if err := setting.Validate("OFF"); err != nil {
		t.Fatalf("Validate(OFF) error = %v", err)
	}
}
