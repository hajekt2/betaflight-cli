package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const maxModeDefinitionPages = 16

type ModeConfiguration struct {
	Definitions []ModeDefinition `json:"definitions"`
	Ranges      []ModeRange      `json:"ranges"`
}

type ModeDefinition struct {
	ID   uint8  `json:"id"`
	Name string `json:"name,omitempty"`
}

type ModeRange struct {
	Index           int       `json:"index"`
	ID              uint8     `json:"id"`
	Name            string    `json:"name,omitempty"`
	AuxChannelIndex uint8     `json:"aux_channel_index"`
	AuxChannelName  string    `json:"aux_channel_name"`
	Range           StepRange `json:"range"`
	Active          bool      `json:"active"`
	ModeLogic       *uint8    `json:"mode_logic,omitempty"`
	ModeLogicName   string    `json:"mode_logic_name,omitempty"`
	LinkedTo        *uint8    `json:"linked_to,omitempty"`
	LinkedToName    string    `json:"linked_to_name,omitempty"`
}

type StepRange struct {
	StartStep uint8  `json:"start_step"`
	EndStep   uint8  `json:"end_step"`
	StartUS   uint16 `json:"start_us"`
	EndUS     uint16 `json:"end_us"`
}

type modeRangeExtra struct {
	ID        uint8
	Logic     uint8
	LinkedTo  uint8
	HasLinked bool
}

func ReadModeConfiguration(ctx context.Context, client *connection.Client) (*ModeConfiguration, []string, error) {
	definitions, err := readModeDefinitions(ctx, client)
	if err != nil {
		return nil, nil, err
	}
	frame, err := client.Request(ctx, msp.MSPModeRanges, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("mode ranges unavailable: %w", err)
	}
	ranges, err := DecodeModeRanges(frame.Payload)
	if err != nil {
		return nil, nil, fmt.Errorf("mode ranges decode failed: %w", err)
	}
	warnings := []string{}
	extrasFrame, err := client.Request(ctx, msp.MSPModeRangesExtra, nil)
	if err != nil {
		warnings = append(warnings, fmt.Sprintf("MSP_MODE_RANGES_EXTRA unavailable: %v", err))
	} else {
		extras, decodeErr := DecodeModeRangeExtras(extrasFrame.Payload)
		if decodeErr != nil {
			warnings = append(warnings, fmt.Sprintf("MSP_MODE_RANGES_EXTRA decode failed: %v", decodeErr))
		} else {
			attachModeRangeExtras(ranges, extras)
		}
	}
	names := modeNameMap(definitions)
	for i := range ranges {
		ranges[i].Name = names[ranges[i].ID]
		if ranges[i].LinkedTo != nil && *ranges[i].LinkedTo != 0 {
			ranges[i].LinkedToName = names[*ranges[i].LinkedTo]
		}
	}
	return &ModeConfiguration{Definitions: definitions, Ranges: ranges}, warnings, nil
}

func readModeDefinitions(ctx context.Context, client *connection.Client) ([]ModeDefinition, error) {
	var definitions []ModeDefinition
	for page := 0; page < maxModeDefinitionPages; page++ {
		payload := []byte{byte(page)}
		namesFrame, err := client.Request(ctx, msp.MSPBoxnames, payload)
		if err != nil {
			return nil, fmt.Errorf("mode names unavailable: %w", err)
		}
		idsFrame, err := client.Request(ctx, msp.MSPBoxids, payload)
		if err != nil {
			return nil, fmt.Errorf("mode ids unavailable: %w", err)
		}
		names := DecodeBoxNames(namesFrame.Payload)
		ids := DecodeBoxIDs(idsFrame.Payload)
		if len(names) == 0 && len(ids) == 0 {
			break
		}
		if len(names) != len(ids) {
			return nil, fmt.Errorf("mode names and ids page %d have different lengths: %d names, %d ids", page, len(names), len(ids))
		}
		for i, id := range ids {
			definitions = append(definitions, ModeDefinition{ID: id, Name: names[i]})
		}
	}
	return definitions, nil
}

func DecodeBoxNames(payload []byte) []string {
	if len(payload) == 0 {
		return nil
	}
	raw := strings.Split(string(payload), ";")
	names := make([]string, 0, len(raw))
	for _, name := range raw {
		if name != "" {
			names = append(names, name)
		}
	}
	return names
}

func DecodeBoxIDs(payload []byte) []uint8 {
	ids := make([]uint8, len(payload))
	copy(ids, payload)
	return ids
}

func DecodeModeRanges(payload []byte) ([]ModeRange, error) {
	if len(payload)%4 != 0 {
		return nil, fmt.Errorf("payload length %d is not a multiple of 4", len(payload))
	}
	ranges := make([]ModeRange, 0, len(payload)/4)
	r := msp.NewPayloadReader(payload)
	for i := 0; r.Remaining() > 0; i++ {
		id, err := r.U8()
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
		ranges = append(ranges, ModeRange{
			Index:           i,
			ID:              id,
			AuxChannelIndex: aux,
			AuxChannelName:  fmt.Sprintf("AUX%d", aux+1),
			Range: StepRange{
				StartStep: start,
				EndStep:   end,
				StartUS:   stepToMicroseconds(start),
				EndUS:     stepToMicroseconds(end),
			},
			Active: start != 0 || end != 0,
		})
	}
	return ranges, nil
}

func DecodeModeRangeExtras(payload []byte) ([]modeRangeExtra, error) {
	if len(payload) == 0 {
		return nil, nil
	}
	r := msp.NewPayloadReader(payload)
	count, err := r.U8()
	if err != nil {
		return nil, err
	}
	extras := make([]modeRangeExtra, 0, count)
	for i := 0; i < int(count); i++ {
		id, err := r.U8()
		if err != nil {
			return nil, err
		}
		logic, err := r.U8()
		if err != nil {
			return nil, err
		}
		linkedTo, err := r.U8()
		if err != nil {
			return nil, err
		}
		extras = append(extras, modeRangeExtra{
			ID:        id,
			Logic:     logic,
			LinkedTo:  linkedTo,
			HasLinked: linkedTo != 0,
		})
	}
	return extras, nil
}

func attachModeRangeExtras(ranges []ModeRange, extras []modeRangeExtra) {
	for i := range ranges {
		if i >= len(extras) {
			return
		}
		logic := extras[i].Logic
		ranges[i].ModeLogic = &logic
		ranges[i].ModeLogicName = modeLogicName(logic)
		if extras[i].HasLinked {
			linkedTo := extras[i].LinkedTo
			ranges[i].LinkedTo = &linkedTo
		}
	}
}

func modeNameMap(definitions []ModeDefinition) map[uint8]string {
	names := make(map[uint8]string, len(definitions))
	for _, definition := range definitions {
		names[definition.ID] = definition.Name
	}
	return names
}

func modeLogicName(logic uint8) string {
	switch logic {
	case 0:
		return "OR"
	case 1:
		return "AND"
	default:
		return ""
	}
}

func stepToMicroseconds(step uint8) uint16 {
	return 900 + uint16(step)*25
}
