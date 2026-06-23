package commands

import "testing"

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
