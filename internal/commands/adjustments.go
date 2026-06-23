package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const (
	adjustmentRangeLegacySize = 6
	adjustmentRangeModernSize = 10
)

var adjustmentFunctionNames = []string{
	"NONE",
	"RC_RATE",
	"RC_EXPO",
	"THROTTLE_EXPO",
	"PITCH_ROLL_RATE",
	"YAW_RATE",
	"PITCH_ROLL_P",
	"PITCH_ROLL_I",
	"PITCH_ROLL_D",
	"YAW_P",
	"YAW_I",
	"YAW_D",
	"RATE_PROFILE",
	"PITCH_RATE",
	"ROLL_RATE",
	"PITCH_P",
	"PITCH_I",
	"PITCH_D",
	"ROLL_P",
	"ROLL_I",
	"ROLL_D",
	"RC_RATE_YAW",
	"PITCH_ROLL_F",
	"FEEDFORWARD_TRANSITION",
	"HORIZON_STRENGTH",
	"PID_AUDIO",
	"PITCH_F",
	"ROLL_F",
	"YAW_F",
	"OSD_PROFILE",
	"LED_PROFILE",
	"LED_DIMMER",
	"SIMPLIFIED_MASTER_MULTIPLIER",
	"BATTERY_PROFILE",
}

type AdjustmentStatus struct {
	Source string            `json:"source"`
	Ranges []AdjustmentRange `json:"ranges"`
}

type AdjustmentRange struct {
	Index                  int    `json:"index"`
	SlotIndex              uint8  `json:"slot_index"`
	AuxChannelIndex        uint8  `json:"aux_channel_index"`
	AuxChannelName         string `json:"aux_channel_name,omitempty"`
	RangeStartStep         uint8  `json:"range_start_step"`
	RangeEndStep           uint8  `json:"range_end_step"`
	RangeStartUS           uint16 `json:"range_start_us"`
	RangeEndUS             uint16 `json:"range_end_us"`
	AdjustmentFunction     uint8  `json:"adjustment_function"`
	AdjustmentFunctionName string `json:"adjustment_function_name,omitempty"`
	AuxSwitchChannelIndex  uint8  `json:"aux_switch_channel_index"`
	AuxSwitchChannelName   string `json:"aux_switch_channel_name,omitempty"`
	AdjustmentCenter       uint16 `json:"adjustment_center"`
	AdjustmentScale        uint16 `json:"adjustment_scale"`
	Active                 bool   `json:"active"`
	CLICommand             string `json:"cli_command"`
}

type AdjustmentRangeSetResult struct {
	Range        AdjustmentRange `json:"range"`
	MSPCode      uint16          `json:"msp_code"`
	MSPName      string          `json:"msp_name"`
	Acknowledged bool            `json:"acknowledged"`
	SaveRequired bool            `json:"save_required"`
}

type AdjustmentTableSetConfig struct {
	Ranges []AdjustmentRange `json:"ranges"`
}

type AdjustmentTableSetResult struct {
	Ranges       []AdjustmentRange `json:"ranges"`
	RangeCount   int               `json:"range_count"`
	MSPName      string            `json:"msp_name"`
	Acknowledged bool              `json:"acknowledged"`
	SaveRequired bool              `json:"save_required"`
}

func ReadAdjustmentStatus(ctx context.Context, client *connection.Client) (*AdjustmentStatus, error) {
	frame, err := client.Request(ctx, msp.MSPAdjustmentRanges, nil)
	if err != nil {
		return nil, fmt.Errorf("adjustment ranges unavailable: %w", err)
	}
	ranges, err := DecodeAdjustmentRanges(frame.Payload)
	if err != nil {
		return nil, err
	}
	return &AdjustmentStatus{Source: "MSP_ADJUSTMENT_RANGES", Ranges: ranges}, nil
}

func SetAdjustmentRange(ctx context.Context, client *connection.Client, row AdjustmentRange) (*AdjustmentRangeSetResult, error) {
	if err := ValidateAdjustmentRange(row); err != nil {
		return nil, err
	}
	if _, err := client.Request(ctx, msp.MSPSetAdjustmentRange, EncodeAdjustmentRange(row)); err != nil {
		return nil, fmt.Errorf("adjustment range request failed: %w", err)
	}
	normalized := adjustmentRange(row.Index, row.SlotIndex, row.AuxChannelIndex, row.RangeStartStep, row.RangeEndStep, row.AdjustmentFunction, row.AuxSwitchChannelIndex, row.AdjustmentCenter, row.AdjustmentScale)
	return &AdjustmentRangeSetResult{
		Range:        normalized,
		MSPCode:      msp.MSPSetAdjustmentRange,
		MSPName:      "MSP_SET_ADJUSTMENT_RANGE",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func EncodeAdjustmentRange(row AdjustmentRange) []byte {
	payload := []byte{
		byte(row.Index),
		row.SlotIndex,
		row.AuxChannelIndex,
		row.RangeStartStep,
		row.RangeEndStep,
		row.AdjustmentFunction,
		row.AuxSwitchChannelIndex,
	}
	payload = appendU16Payload(payload, row.AdjustmentCenter)
	payload = appendU16Payload(payload, row.AdjustmentScale)
	return payload
}

func SetAdjustmentTable(ctx context.Context, client *connection.Client, config AdjustmentTableSetConfig) (*AdjustmentTableSetResult, error) {
	if err := ValidateAdjustmentTable(config); err != nil {
		return nil, err
	}
	ranges := make([]AdjustmentRange, 0, len(config.Ranges))
	for _, row := range config.Ranges {
		result, err := SetAdjustmentRange(ctx, client, row)
		if err != nil {
			return nil, err
		}
		ranges = append(ranges, result.Range)
	}
	return &AdjustmentTableSetResult{
		Ranges:       ranges,
		RangeCount:   len(ranges),
		MSPName:      "MSP_SET_ADJUSTMENT_RANGE",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func ValidateAdjustmentTable(config AdjustmentTableSetConfig) error {
	if len(config.Ranges) == 0 {
		return fmt.Errorf("at least one adjustment range is required")
	}
	for i, row := range config.Ranges {
		if err := ValidateAdjustmentRange(row); err != nil {
			return fmt.Errorf("ranges[%d]: %w", i, err)
		}
	}
	return nil
}

func ValidateAdjustmentRange(row AdjustmentRange) error {
	if row.Index < 0 || row.Index > 255 {
		return fmt.Errorf("index must be 0..255")
	}
	if row.RangeStartStep > row.RangeEndStep {
		return fmt.Errorf("range_start_step must be less than or equal to range_end_step")
	}
	return nil
}

func DecodeAdjustmentRanges(payload []byte) ([]AdjustmentRange, error) {
	rowSize, err := adjustmentRangeRowSize(len(payload))
	if err != nil {
		return nil, err
	}
	r := msp.NewPayloadReader(payload)
	ranges := make([]AdjustmentRange, 0, len(payload)/rowSize)
	for index := 0; r.Remaining() > 0; index++ {
		slot, err := r.U8()
		if err != nil {
			return nil, err
		}
		aux, err := r.U8()
		if err != nil {
			return nil, err
		}
		start, err := r.U8()
		if err != nil {
			return nil, err
		}
		end, err := r.U8()
		if err != nil {
			return nil, err
		}
		function, err := r.U8()
		if err != nil {
			return nil, err
		}
		selectChannel, err := r.U8()
		if err != nil {
			return nil, err
		}
		var center, scale uint16
		if rowSize == adjustmentRangeModernSize {
			center, err = r.U16()
			if err != nil {
				return nil, err
			}
			scale, err = r.U16()
			if err != nil {
				return nil, err
			}
		}
		ranges = append(ranges, adjustmentRange(index, slot, aux, start, end, function, selectChannel, center, scale))
	}
	return ranges, nil
}

func adjustmentRangeRowSize(length int) (int, error) {
	if length%adjustmentRangeModernSize == 0 {
		return adjustmentRangeModernSize, nil
	}
	if length%adjustmentRangeLegacySize == 0 {
		return adjustmentRangeLegacySize, nil
	}
	return 0, fmt.Errorf("payload length %d is not divisible by MSP_ADJUSTMENT_RANGES row sizes %d or %d", length, adjustmentRangeLegacySize, adjustmentRangeModernSize)
}

func adjustmentRange(index int, slotIndex, aux, start, end, function, selectChannel uint8, center, scale uint16) AdjustmentRange {
	startUS := adjustmentStepToUS(start)
	endUS := adjustmentStepToUS(end)
	return AdjustmentRange{
		Index:                  index,
		SlotIndex:              slotIndex,
		AuxChannelIndex:        aux,
		AuxChannelName:         auxChannelName(aux),
		RangeStartStep:         start,
		RangeEndStep:           end,
		RangeStartUS:           startUS,
		RangeEndUS:             endUS,
		AdjustmentFunction:     function,
		AdjustmentFunctionName: indexedName(adjustmentFunctionNames, function),
		AuxSwitchChannelIndex:  selectChannel,
		AuxSwitchChannelName:   auxChannelName(selectChannel),
		AdjustmentCenter:       center,
		AdjustmentScale:        scale,
		Active:                 function != 0,
		CLICommand:             fmt.Sprintf("adjrange %d 0 %d %d %d %d %d %d %d", index, aux, startUS, endUS, function, selectChannel, center, scale),
	}
}

func adjustmentStepToUS(step uint8) uint16 {
	return 900 + uint16(step)*25
}

func auxChannelName(index uint8) string {
	return fmt.Sprintf("AUX%d", int(index)+1)
}
