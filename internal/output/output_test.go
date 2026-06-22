package output

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestRenderJSONEnvelope(t *testing.T) {
	env := Success("version", nil, map[string]string{"version": "dev"})
	var buf bytes.Buffer
	if err := Render(&buf, "json", env); err != nil {
		t.Fatalf("Render() error = %v", err)
	}
	var decoded Envelope
	if err := json.Unmarshal(buf.Bytes(), &decoded); err != nil {
		t.Fatalf("invalid JSON: %v", err)
	}
	if decoded.SchemaVersion != SchemaVersion || !decoded.OK || decoded.Command != "version" {
		t.Fatalf("unexpected envelope: %+v", decoded)
	}
}

func TestFailureEnvelope(t *testing.T) {
	env := Failure("save", nil, "confirmation_required", "pass --yes")
	if env.OK {
		t.Fatal("Failure().OK = true, want false")
	}
	if len(env.Errors) != 1 || env.Errors[0].Code != "confirmation_required" {
		t.Fatalf("unexpected errors: %+v", env.Errors)
	}
}
