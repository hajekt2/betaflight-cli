package msp

import (
	"encoding/binary"
	"fmt"
)

type PayloadReader struct {
	data []byte
	pos  int
}

func NewPayloadReader(data []byte) *PayloadReader {
	return &PayloadReader{data: data}
}

func (r *PayloadReader) Remaining() int {
	return len(r.data) - r.pos
}

func (r *PayloadReader) U8() (uint8, error) {
	if r.Remaining() < 1 {
		return 0, ErrShortPayload
	}
	v := r.data[r.pos]
	r.pos++
	return v, nil
}

func (r *PayloadReader) U16() (uint16, error) {
	if r.Remaining() < 2 {
		return 0, ErrShortPayload
	}
	v := binary.LittleEndian.Uint16(r.data[r.pos:])
	r.pos += 2
	return v, nil
}

func (r *PayloadReader) S16() (int16, error) {
	v, err := r.U16()
	return int16(v), err
}

func (r *PayloadReader) U32() (uint32, error) {
	if r.Remaining() < 4 {
		return 0, ErrShortPayload
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *PayloadReader) Bytes(n int) ([]byte, error) {
	if r.Remaining() < n {
		return nil, ErrShortPayload
	}
	v := r.data[r.pos : r.pos+n]
	r.pos += n
	return v, nil
}

func (r *PayloadReader) PString() (string, error) {
	n, err := r.U8()
	if err != nil {
		return "", err
	}
	b, err := r.Bytes(int(n))
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func RequireNoShort(err error, field string) error {
	if err == nil {
		return nil
	}
	if err == ErrShortPayload {
		return fmt.Errorf("%w reading %s", err, field)
	}
	return err
}
