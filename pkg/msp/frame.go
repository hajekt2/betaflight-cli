package msp

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

type Frame struct {
	Version     int
	Direction   byte
	Code        uint16
	Payload     []byte
	Unsupported bool
}

// maxMSPv2Payload is the largest payload MSPv2 can encode: the wire
// length field is uint16.
const maxMSPv2Payload = int(^uint16(0))

func EncodeRequest(code uint16, payload []byte) []byte {
	if code <= 254 && len(payload) < 255 {
		return encodeV1(byte(code), payload)
	}
	return encodeV2(code, payload)
}

func encodeV1(code byte, payload []byte) []byte {
	frame := make([]byte, 0, len(payload)+6)
	frame = append(frame, '$', 'M', '<', byte(len(payload)), code)
	checksum := byte(len(payload)) ^ code
	for _, b := range payload {
		frame = append(frame, b)
		checksum ^= b
	}
	frame = append(frame, checksum)
	return frame
}

func encodeV2(code uint16, payload []byte) []byte {
	// MSPv2 carries the payload length in a uint16 field; callers pass
	// typed payloads far below this bound. Clamping keeps the size
	// arithmetic provably in range for the allocation below.
	n := len(payload)
	if n > maxMSPv2Payload {
		n = maxMSPv2Payload
	}
	frame := make([]byte, 0, n+9)
	frame = append(frame, '$', 'X', '<', 0)
	var tmp [2]byte
	binary.LittleEndian.PutUint16(tmp[:], code)
	frame = append(frame, tmp[:]...)
	binary.LittleEndian.PutUint16(tmp[:], uint16(len(payload)))
	frame = append(frame, tmp[:]...)
	frame = append(frame, payload...)
	frame = append(frame, CRC8DVB(frame[3:]))
	return frame
}

func ReadFrame(r io.Reader) (Frame, error) {
	for {
		var b [1]byte
		if err := readFull(r, b[:]); err != nil {
			return Frame{}, err
		}
		if b[0] != '$' {
			continue
		}
		if err := readFull(r, b[:]); err != nil {
			return Frame{}, err
		}
		switch b[0] {
		case 'M':
			return readV1(r)
		case 'X':
			return readV2(r)
		default:
			continue
		}
	}
}

func readV1(r io.Reader) (Frame, error) {
	var header [3]byte
	if err := readFull(r, header[:]); err != nil {
		return Frame{}, err
	}
	direction := header[0]
	size := int(header[1])
	code := uint16(header[2])
	checksumBytes := []byte{header[1], header[2]}
	if direction != '<' && direction != '>' && direction != '!' {
		return Frame{}, fmt.Errorf("unexpected MSPv1 direction %q", direction)
	}
	if size == 255 {
		var jumbo [2]byte
		if err := readFull(r, jumbo[:]); err != nil {
			return Frame{}, err
		}
		size = int(binary.LittleEndian.Uint16(jumbo[:]))
		checksumBytes = append(checksumBytes, jumbo[:]...)
	}
	payload := make([]byte, size)
	if err := readFull(r, payload); err != nil {
		return Frame{}, err
	}
	var got [1]byte
	if err := readFull(r, got[:]); err != nil {
		return Frame{}, err
	}
	checksum := byte(0)
	for _, b := range checksumBytes {
		checksum ^= b
	}
	for _, b := range payload {
		checksum ^= b
	}
	if checksum != got[0] {
		return Frame{}, fmt.Errorf("invalid MSPv1 checksum: got 0x%02x want 0x%02x", got[0], checksum)
	}
	return Frame{
		Version:     1,
		Direction:   direction,
		Code:        code,
		Payload:     payload,
		Unsupported: direction == '!',
	}, nil
}

func readV2(r io.Reader) (Frame, error) {
	var header [6]byte
	if err := readFull(r, header[:]); err != nil {
		return Frame{}, err
	}
	direction := header[0]
	if direction != '<' && direction != '>' && direction != '!' {
		return Frame{}, fmt.Errorf("unexpected MSPv2 direction %q", direction)
	}
	flag := header[1]
	code := binary.LittleEndian.Uint16(header[2:4])
	size := int(binary.LittleEndian.Uint16(header[4:6]))
	payload := make([]byte, size)
	if err := readFull(r, payload); err != nil {
		return Frame{}, err
	}
	var got [1]byte
	if err := readFull(r, got[:]); err != nil {
		return Frame{}, err
	}
	// Checksum covers flag, code, and length followed by the payload;
	// computed incrementally so no concatenated buffer is needed.
	var head [5]byte
	head[0] = flag
	copy(head[1:], header[2:6])
	want := CRC8DVBUpdate(CRC8DVBUpdate(0, head[:]), payload)
	if got[0] != want {
		return Frame{}, fmt.Errorf("invalid MSPv2 checksum: got 0x%02x want 0x%02x", got[0], want)
	}
	return Frame{
		Version:     2,
		Direction:   direction,
		Code:        code,
		Payload:     payload,
		Unsupported: direction == '!',
	}, nil
}

var ErrShortPayload = errors.New("short MSP payload")

func readFull(r io.Reader, buf []byte) error {
	for len(buf) > 0 {
		n, err := r.Read(buf)
		if n > 0 {
			buf = buf[n:]
		}
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrNoProgress
		}
	}
	return nil
}
