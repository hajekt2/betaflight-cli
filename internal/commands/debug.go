package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type DebugStatus struct {
	DebugValues       []int16            `json:"debug_values,omitempty"`
	AccelerometerTrim *AccelerometerTrim `json:"accelerometer_trim,omitempty"`
	Sources           map[string]string  `json:"sources,omitempty"`
}

type AccelerometerTrim struct {
	Pitch int16 `json:"pitch"`
	Roll  int16 `json:"roll"`
}

type AccelerometerTrimSetResult struct {
	Trim         AccelerometerTrim `json:"trim"`
	MSPCode      uint16            `json:"msp_code"`
	MSPName      string            `json:"msp_name"`
	Acknowledged bool              `json:"acknowledged"`
}

func ReadDebugStatus(ctx context.Context, client *connection.Client) (*DebugStatus, []string, error) {
	status := &DebugStatus{Sources: map[string]string{}}
	warnings := []string{}
	if frame, err := client.Request(ctx, msp.MSPDebug, nil); err == nil {
		values, err := DecodeDebugValues(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_DEBUG decode failed: %v", err))
		} else {
			status.DebugValues = values
			status.Sources["debug_values"] = "MSP_DEBUG"
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_DEBUG unavailable: %v", err))
	}
	if frame, err := client.Request(ctx, msp.MSPAccTrim, nil); err == nil {
		trim, err := DecodeAccelerometerTrim(frame.Payload)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_ACC_TRIM decode failed: %v", err))
		} else {
			status.AccelerometerTrim = trim
			status.Sources["accelerometer_trim"] = "MSP_ACC_TRIM"
		}
	} else {
		warnings = append(warnings, fmt.Sprintf("MSP_ACC_TRIM unavailable: %v", err))
	}
	if len(status.DebugValues) == 0 && status.AccelerometerTrim == nil {
		return nil, warnings, fmt.Errorf("debug status unavailable")
	}
	return status, warnings, nil
}

func DecodeDebugValues(payload []byte) ([]int16, error) {
	if len(payload)%2 != 0 {
		return nil, fmt.Errorf("MSP_DEBUG payload length %d is not divisible by 2", len(payload))
	}
	r := msp.NewPayloadReader(payload)
	values := make([]int16, 0, len(payload)/2)
	for r.Remaining() > 0 {
		value, err := r.S16()
		if err != nil {
			return nil, msp.RequireNoShort(err, "debug value")
		}
		values = append(values, value)
	}
	return values, nil
}

func DecodeAccelerometerTrim(payload []byte) (*AccelerometerTrim, error) {
	const length = 4
	if len(payload) < length {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_ACC_TRIM size %d", len(payload), length)
	}
	r := msp.NewPayloadReader(payload)
	pitch, err := r.S16()
	if err != nil {
		return nil, msp.RequireNoShort(err, "accelerometer trim pitch")
	}
	roll, err := r.S16()
	if err != nil {
		return nil, msp.RequireNoShort(err, "accelerometer trim roll")
	}
	if r.Remaining() != 0 {
		return nil, fmt.Errorf("MSP_ACC_TRIM returned %d trailing byte(s)", r.Remaining())
	}
	return &AccelerometerTrim{Pitch: pitch, Roll: roll}, nil
}

func SetAccelerometerTrim(ctx context.Context, client *connection.Client, trim AccelerometerTrim) (*AccelerometerTrimSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetAccTrim, EncodeAccelerometerTrim(trim)); err != nil {
		return nil, fmt.Errorf("accelerometer trim request failed: %w", err)
	}
	return &AccelerometerTrimSetResult{
		Trim:         trim,
		MSPCode:      msp.MSPSetAccTrim,
		MSPName:      "MSP_SET_ACC_TRIM",
		Acknowledged: true,
	}, nil
}

func EncodeAccelerometerTrim(trim AccelerometerTrim) []byte {
	payload := make([]byte, 0, 4)
	payload = append(payload, byte(trim.Pitch), byte(uint16(trim.Pitch)>>8))
	payload = append(payload, byte(trim.Roll), byte(uint16(trim.Roll)>>8))
	return payload
}
