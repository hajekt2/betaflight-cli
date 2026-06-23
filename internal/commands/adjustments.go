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
