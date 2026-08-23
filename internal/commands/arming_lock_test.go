package commands

import (
	"bytes"
	"context"
	"encoding/binary"
	"sync"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

// buildStatusExPayload crafts an MSP_STATUS_EX frame body following the field
// order of src/main/msp/msp.c case MSP_STATUS_EX at tag 2026.6.1 (line 1131):
// u16 cycle, u16 i2c errors, u16 sensors, u32 mode flags, u8 pid profile,
// u16 cpu load, u8 pid profile count, u8 rate profile index,
// u8 extra-flag-byte-count (+ those bytes), u8 arming-disable flag count,
// u32 arming-disable flags, u8 config state flag, u16 cpu temperature,
// u8 rate profile count, u8 battery profile count, u8 battery profile.
func buildStatusExPayload(armingFlags uint32) []byte {
	payload := make([]byte, 0, 32)
	payload = binary.LittleEndian.AppendUint16(payload, 250)
	payload = binary.LittleEndian.AppendUint16(payload, 0)
	payload = binary.LittleEndian.AppendUint16(payload, 33)
	payload = binary.LittleEndian.AppendUint32(payload, 2)
	payload = append(payload, 0)
	payload = binary.LittleEndian.AppendUint16(payload, 42)
	payload = append(payload, 2, 0)
	payload = append(payload, 0)
	payload = append(payload, 30)
	payload = binary.LittleEndian.AppendUint32(payload, armingFlags)
	payload = append(payload, 0)
	payload = binary.LittleEndian.AppendUint16(payload, 425)
	payload = append(payload, 3, 1, 1)
	return payload
}

// armingLockStubPort answers MSP_STATUS_EX and MSP_SET_ARMING_DISABLED frames
// like a minimal FC, letting SetArmingLock be exercised end-to-end.
type armingLockStubPort struct {
	mu       sync.Mutex
	pending  bytes.Buffer
	requests []uint16
	commands []uint8
	flags    uint32
}

func (p *armingLockStubPort) reply(code uint16, payload []byte) {
	frame := msp.EncodeRequest(code, payload)
	frame[2] = '>'
	p.pending.Write(frame)
}

func (p *armingLockStubPort) Write(data []byte) (int, error) {
	if len(data) < 6 || data[0] != '$' || data[1] != 'M' {
		return len(data), nil
	}
	code := uint16(data[4])
	var payload []byte
	if n := int(data[3]); n > 0 {
		payload = data[5 : 5+n]
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.requests = append(p.requests, code)
	switch code {
	case msp.MSPStatusEx:
		p.reply(code, buildStatusExPayload(p.flags))
	case msp.MSPSetArmingDisabled:
		command := uint8(0)
		if len(payload) > 0 {
			command = payload[0]
		}
		p.commands = append(p.commands, command)
		if command != 0 {
			p.flags |= GCSArmingDisableMask
		} else {
			p.flags &^= GCSArmingDisableMask
		}
		p.reply(code, nil)
	default:
		frame := msp.EncodeRequest(code, nil)
		frame[2] = '!'
		p.pending.Write(frame)
	}
	return len(data), nil
}

func (p *armingLockStubPort) Read(buf []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.pending.Read(buf)
}

func (p *armingLockStubPort) ResetInputBuffer() error            { return nil }
func (p *armingLockStubPort) ResetOutputBuffer() error           { return nil }
func (p *armingLockStubPort) SetReadTimeout(time.Duration) error { return nil }
func (p *armingLockStubPort) Close() error                       { return nil }

func newArmingLockClient(t *testing.T, port *armingLockStubPort) *connection.Client {
	t.Helper()
	client, err := connection.NewClient(port, time.Second)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func TestArmingLockStateFromStatusExDecode(t *testing.T) {
	status, err := DecodeStatusEx(buildStatusExPayload(1<<5 | 1<<7))
	if err != nil {
		t.Fatalf("DecodeStatusEx: %v", err)
	}
	state := armingLockStateFromStatus(status)
	if state == nil {
		t.Fatal("state = nil")
	}
	if state.Enabled {
		t.Fatalf("Enabled = true, want false")
	}
	if state.Flags != 1<<5|1<<7 {
		t.Fatalf("Flags = %#x", state.Flags)
	}
	if state.Count != 30 {
		t.Fatalf("Count = %d, want 30", state.Count)
	}
	want := []string{"RUNAWAY", "THROTTLE"}
	if len(state.ActiveNames) != len(want) {
		t.Fatalf("ActiveNames = %v, want %v", state.ActiveNames, want)
	}
	for i, name := range want {
		if state.ActiveNames[i] != name {
			t.Fatalf("ActiveNames = %v, want %v", state.ActiveNames, want)
		}
	}
}

func TestArmingLockStateEnabledWhenMSPBitSet(t *testing.T) {
	status, err := DecodeStatusEx(buildStatusExPayload(GCSArmingDisableMask))
	if err != nil {
		t.Fatalf("DecodeStatusEx: %v", err)
	}
	state := armingLockStateFromStatus(status)
	if state == nil || !state.Enabled {
		t.Fatalf("state = %+v, want Enabled true", state)
	}
	found := false
	for _, name := range state.ActiveNames {
		if name == "MSP" {
			found = true
		}
	}
	if !found {
		t.Fatalf("ActiveNames = %v, want MSP present", state.ActiveNames)
	}
}

func TestArmingLockStateNilWithoutArmingFields(t *testing.T) {
	if got := armingLockStateFromStatus(&Status{Source: "MSP_STATUS"}); got != nil {
		t.Fatalf("state = %+v, want nil", got)
	}
	if got := armingLockStateFromStatus(nil); got != nil {
		t.Fatalf("state = %+v, want nil", got)
	}
}

func TestSetArmingLockRoundTrip(t *testing.T) {
	port := &armingLockStubPort{flags: 1 << 2}
	client := newArmingLockClient(t, port)
	result, err := SetArmingLock(context.Background(), client, true)
	if err != nil {
		t.Fatalf("SetArmingLock: %v", err)
	}
	if !result.Acknowledged || !result.Changed {
		t.Fatalf("result = %+v", result)
	}
	if result.MSPCode != msp.MSPSetArmingDisabled || result.MSPName != "MSP_SET_ARMING_DISABLED" {
		t.Fatalf("result = %+v", result)
	}
	if result.Flags&GCSArmingDisableMask == 0 {
		t.Fatalf("Flags = %#x, want MSP bit set", result.Flags)
	}
	port.mu.Lock()
	sent := append([]uint8(nil), port.commands...)
	port.mu.Unlock()
	if len(sent) != 1 || sent[0] != 1 {
		t.Fatalf("commands sent = %v, want [1]", sent)
	}
}

func TestSetArmingLockReleaseAndIdempotence(t *testing.T) {
	port := &armingLockStubPort{flags: GCSArmingDisableMask | 1<<9}
	client := newArmingLockClient(t, port)
	result, err := SetArmingLock(context.Background(), client, false)
	if err != nil {
		t.Fatalf("SetArmingLock: %v", err)
	}
	if !result.Changed {
		t.Fatalf("Changed = false, want true: %+v", result)
	}
	if result.Flags&GCSArmingDisableMask != 0 || result.Flags&(1<<9) == 0 {
		t.Fatalf("Flags = %#x, want MSP bit clear and BOOTGRACE kept", result.Flags)
	}
	again, err := SetArmingLock(context.Background(), client, false)
	if err != nil {
		t.Fatalf("SetArmingLock again: %v", err)
	}
	if again.Changed {
		t.Fatalf("Changed = true on idempotent release: %+v", again)
	}
}

func TestReadArmingLockStateUsesStatusEx(t *testing.T) {
	port := &armingLockStubPort{flags: 0}
	client := newArmingLockClient(t, port)
	state, err := ReadArmingLockState(context.Background(), client)
	if err != nil {
		t.Fatalf("ReadArmingLockState: %v", err)
	}
	if state.Enabled || state.Flags != 0 {
		t.Fatalf("state = %+v", state)
	}
	port.mu.Lock()
	requested := append([]uint16(nil), port.requests...)
	port.mu.Unlock()
	if len(requested) != 1 || requested[0] != msp.MSPStatusEx {
		t.Fatalf("requests = %v, want [%d]", requested, msp.MSPStatusEx)
	}
}
