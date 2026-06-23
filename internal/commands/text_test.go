package commands

import (
	"testing"

	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

func TestEncodeTextSet(t *testing.T) {
	field, ok := TextFieldByKey("craft_name")
	if !ok {
		t.Fatal("craft_name field not found")
	}
	got, err := EncodeTextSet(TextSetRequest{TextField: field, Value: "Quad One"})
	if err != nil {
		t.Fatalf("EncodeTextSet() error = %v", err)
	}
	want := []byte{msp.MSP2TextCraftName, 8, 'Q', 'u', 'a', 'd', ' ', 'O', 'n', 'e'}
	if string(got) != string(want) {
		t.Fatalf("EncodeTextSet() = %v, want %v", got, want)
	}
}

func TestEncodeTextSetRejectsReadOnlyField(t *testing.T) {
	field, ok := TextFieldByKey("build_key")
	if !ok {
		t.Fatal("build_key field not found")
	}
	if _, err := EncodeTextSet(TextSetRequest{TextField: field, Value: "abc"}); err == nil {
		t.Fatal("EncodeTextSet() error = nil, want read-only error")
	}
}

func TestEncodeTextSetRejectsLongValue(t *testing.T) {
	field, ok := TextFieldByKey("battery_profile_name")
	if !ok {
		t.Fatal("battery_profile_name field not found")
	}
	if _, err := EncodeTextSet(TextSetRequest{TextField: field, Value: "too-long-name"}); err == nil {
		t.Fatal("EncodeTextSet() error = nil, want length error")
	}
}

func TestDecodeTextResponse(t *testing.T) {
	got, err := DecodeTextResponse([]byte{2, 8, 'Q', 'u', 'a', 'd', ' ', 'O', 'n', 'e'}, 2)
	if err != nil {
		t.Fatalf("DecodeTextResponse() error = %v", err)
	}
	if got != "Quad One" {
		t.Fatalf("DecodeTextResponse() = %q", got)
	}
}

func TestDecodeTextResponseRejectsWrongType(t *testing.T) {
	_, err := DecodeTextResponse([]byte{1, 5, 'T', 'o', 'm', 'a', 's'}, 2)
	if err == nil {
		t.Fatal("DecodeTextResponse() error = nil, want wrong type error")
	}
}

func TestDecodeTextResponseRejectsShortPayload(t *testing.T) {
	_, err := DecodeTextResponse([]byte{2, 5, 'Q'}, 2)
	if err == nil {
		t.Fatal("DecodeTextResponse() error = nil, want short payload error")
	}
}

func TestDecodeTextResponseRejectsTrailingBytes(t *testing.T) {
	_, err := DecodeTextResponse([]byte{2, 0, 1}, 2)
	if err == nil {
		t.Fatal("DecodeTextResponse() error = nil, want trailing byte error")
	}
}
