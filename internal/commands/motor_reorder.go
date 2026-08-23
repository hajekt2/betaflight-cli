package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

// Motor output reordering protocol, derived from betaflight 2026.6.1:
//
//   - Read MSP2_MOTOR_OUTPUT_REORDERING (src/main/msp/msp.c:1322):
//     sbufWriteU8(dst, MAX_SUPPORTED_MOTORS) followed by one u8 per entry of
//     motorConfig()->dev.motorOutputReordering[i].
//
//   - Write MSP2_SET_MOTOR_OUTPUT_REORDERING (src/main/msp/msp.c:3818):
//     first byte is arraySize, followed by arraySize u8 values. Entries beyond
//     arraySize are padded on the FC side with identity mapping (value == i).
//
// MAX_SUPPORTED_MOTORS is defined in src/main/target/common_defaults_post.h:434
// as 8 (4 for USE_QUAD_MIXER_ONLY builds, line 430).
const MaxSupportedMotors = 8

// MotorOrder is a motor output ordering slice. It marshals as a JSON number
// array; a plain []uint8 would encode as a base64 string.
type MotorOrder []uint8

func (o MotorOrder) MarshalJSON() ([]byte, error) {
	parts := make([]string, len(o))
	for i, value := range o {
		parts[i] = strconv.Itoa(int(value))
	}
	return []byte("[" + strings.Join(parts, ",") + "]"), nil
}

func (o *MotorOrder) UnmarshalJSON(data []byte) error {
	var values []int
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	order := make(MotorOrder, len(values))
	for i, value := range values {
		if value < 0 || value > 255 {
			return fmt.Errorf("motor order value %d out of byte range", value)
		}
		order[i] = uint8(value)
	}
	*o = order
	return nil
}

// MotorOutputReordering is the motor output ordering map. Order[physical]
// gives the motor index whose output is routed to that position, matching the
// "remap motors" feature noted on motorOutputReordering in pg/motor.h:78.
type MotorOutputReordering struct {
	Order MotorOrder `json:"order"`
}

// MotorReorderSetResult reports an acknowledged MSP2_SET_MOTOR_OUTPUT_REORDERING write.
type MotorReorderSetResult struct {
	Order        MotorOrder `json:"order"`
	MSPCode      uint16     `json:"msp_code"`
	MSPName      string     `json:"msp_name"`
	Acknowledged bool       `json:"acknowledged"`
	SaveRequired bool       `json:"save_required"`
}

// ValidateMotorReordering rejects empty orders, duplicate targets, and values
// outside [0, motorCount).
func ValidateMotorReordering(order []uint8, motorCount int) error {
	if len(order) == 0 {
		return fmt.Errorf("motor reordering requires at least one entry")
	}
	if len(order) > motorCount {
		return fmt.Errorf("motor reordering has %d entries but the target supports %d", len(order), motorCount)
	}
	seen := make(map[uint8]bool, len(order))
	for _, value := range order {
		if int(value) >= motorCount {
			return fmt.Errorf("motor reordering target %d out of range (motor count %d)", value, motorCount)
		}
		if seen[value] {
			return fmt.Errorf("motor reordering target %d appears more than once", value)
		}
		seen[value] = true
	}
	return nil
}

// EncodeMotorReordering serializes the MSP2_SET_MOTOR_OUTPUT_REORDERING
// payload: u8 arraySize followed by arraySize u8 values (msp.c:3818).
func EncodeMotorReordering(order []uint8) []byte {
	payload := make([]byte, 0, len(order)+1)
	payload = append(payload, uint8(len(order)))
	return append(payload, order...)
}

// DecodeMotorOutputReordering parses the MSP2_MOTOR_OUTPUT_REORDERING
// read-back payload: u8 count followed by count u8 values (msp.c:1322).
func DecodeMotorOutputReordering(payload []byte) (*MotorOutputReordering, error) {
	if len(payload) < 1 {
		return nil, fmt.Errorf("motor output reordering payload too short")
	}
	count := int(payload[0])
	if len(payload) < 1+count {
		return nil, fmt.Errorf("motor output reordering payload declares %d entries but carries %d bytes", count, len(payload)-1)
	}
	order := make(MotorOrder, count)
	copy(order, payload[1:])
	return &MotorOutputReordering{Order: order}, nil
}

// SetMotorOutputReordering writes the reordering map through
// MSP2_SET_MOTOR_OUTPUT_REORDERING. Entries are validated against the
// protocol maximum; the FC pads any tail with identity mapping (msp.c:3818).
// The change lives in RAM until a save is issued.
func SetMotorOutputReordering(ctx context.Context, client *connection.Client, order MotorOrder) (*MotorReorderSetResult, error) {
	if err := ValidateMotorReordering(order, MaxSupportedMotors); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSP2SetMotorOutputReordering, EncodeMotorReordering(order)); err != nil {
		return nil, fmt.Errorf("motor reorder request failed: %w", err)
	}
	result := &MotorReorderSetResult{
		Order:        append(MotorOrder(nil), order...),
		MSPCode:      msp.MSP2SetMotorOutputReordering,
		MSPName:      "MSP2_SET_MOTOR_OUTPUT_REORDERING",
		Acknowledged: true,
		SaveRequired: true,
	}
	return result, nil
}

// ReadMotorOutputReordering reads back the current reordering map through
// MSP2_MOTOR_OUTPUT_REORDERING (msp.c:1322).
func ReadMotorOutputReordering(ctx context.Context, client *connection.Client) (*MotorOutputReordering, error) {
	frame, err := client.Request(ctx, msp.MSP2MotorOutputReordering, nil)
	if err != nil {
		return nil, fmt.Errorf("motor output reordering unavailable: %w", err)
	}
	return DecodeMotorOutputReordering(frame.Payload)
}
