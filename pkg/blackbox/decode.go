package blackbox

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	fieldPredictorZero              = 0
	fieldPredictorPrevious          = 1
	fieldPredictorStraightLine      = 2
	fieldPredictorAverage2          = 3
	fieldPredictorMinThrottle       = 4
	fieldPredictorMotor0            = 5
	fieldPredictorIncrement         = 6
	fieldPredictorHomeCoord         = 7
	fieldPredictor1500              = 8
	fieldPredictorVBATRef           = 9
	fieldPredictorLastMainFrameTime = 10
	fieldPredictorMinMotor          = 11
	fieldPredictorHomeCoord1        = 256

	fieldEncodingSignedVB   = 0
	fieldEncodingUnsignedVB = 1
	fieldEncodingNeg14Bit   = 3
	fieldEncodingTag8_8SVB  = 6
	fieldEncodingTag2_3S32  = 7
	fieldEncodingTag8_4S16  = 8
	fieldEncodingNull       = 9
	fieldEncodingTag2_3SVar = 10
)

type decodeContext struct {
	dataVersion       int
	minThrottle       int
	minMotor          int
	vbatRef           int
	mainNameToIndex   map[string]int
	homeNameToIndex   map[string]int
	lastMain          []int
	lastMain2         []int
	lastGPSHome       []int
	lastSlow          []int
	lastGPS           []int
	lastMainFrameTime int
	mainHistoryValid  bool
	homeHistoryValid  bool
}

func (ctx decodeContext) clone() decodeContext {
	out := ctx
	out.lastMain = append([]int(nil), ctx.lastMain...)
	out.lastMain2 = append([]int(nil), ctx.lastMain2...)
	out.lastGPSHome = append([]int(nil), ctx.lastGPSHome...)
	out.lastSlow = append([]int(nil), ctx.lastSlow...)
	out.lastGPS = append([]int(nil), ctx.lastGPS...)
	out.mainNameToIndex = cloneStringIntMap(ctx.mainNameToIndex)
	out.homeNameToIndex = cloneStringIntMap(ctx.homeNameToIndex)
	return out
}

func cloneStringIntMap(in map[string]int) map[string]int {
	if in == nil {
		return nil
	}
	out := make(map[string]int, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}

func newDecodeContext(headers map[string]string, definitions map[string]FieldDefinition) decodeContext {
	return decodeContext{
		dataVersion:     parseHeaderInt(headers, "Data version", 2),
		minThrottle:     parseHeaderInt(headers, "minthrottle", 0),
		minMotor:        parseHeaderArrayFirst(headers, "motorOutput", 0),
		vbatRef:         parseHeaderInt(headers, "vbatref", 0),
		mainNameToIndex: fieldNameIndex(definitions["I"].Names),
		homeNameToIndex: fieldNameIndex(definitions["H"].Names),
	}
}

func fieldNameIndex(names []string) map[string]int {
	if len(names) == 0 {
		return nil
	}
	out := make(map[string]int, len(names))
	for i, name := range names {
		if name != "" {
			out[name] = i
		}
	}
	return out
}

func parseHeaderInt(headers map[string]string, name string, fallback int) int {
	value := strings.TrimSpace(headers[name])
	if value == "" {
		return fallback
	}
	n, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return n
}

func parseHeaderArrayFirst(headers map[string]string, name string, fallback int) int {
	value := strings.TrimSpace(headers[name])
	if value == "" {
		return fallback
	}
	parts := splitCSV(value)
	if len(parts) == 0 {
		return fallback
	}
	n, err := strconv.Atoi(parts[0])
	if err != nil {
		return fallback
	}
	return n
}

func applyPrediction(ctx *decodeContext, frameType string, fieldIndex int, predictor int, value int, current []int, previous []int, previous2 []int) (int, error) {
	switch predictor {
	case fieldPredictorZero:
		return value, nil
	case fieldPredictorMinThrottle:
		return value + ctx.minThrottle, nil
	case fieldPredictorMinMotor:
		return value + ctx.minMotor, nil
	case fieldPredictor1500:
		return value + 1500, nil
	case fieldPredictorMotor0:
		index, ok := ctx.mainNameToIndex["motor[0]"]
		if !ok || index >= len(current) {
			return 0, fmt.Errorf("missing motor[0] field for predictor")
		}
		return value + current[index], nil
	case fieldPredictorVBATRef:
		return value + ctx.vbatRef, nil
	case fieldPredictorPrevious:
		if previous == nil || fieldIndex >= len(previous) {
			return value, nil
		}
		return value + previous[fieldIndex], nil
	case fieldPredictorStraightLine:
		if previous == nil || previous2 == nil || fieldIndex >= len(previous) || fieldIndex >= len(previous2) {
			return value, nil
		}
		return value + 2*previous[fieldIndex] - previous2[fieldIndex], nil
	case fieldPredictorAverage2:
		if previous == nil || previous2 == nil || fieldIndex >= len(previous) || fieldIndex >= len(previous2) {
			return value, nil
		}
		return value + int((int64(previous[fieldIndex])+int64(previous2[fieldIndex]))/2), nil
	case fieldPredictorHomeCoord:
		index, ok := ctx.homeNameToIndex["GPS_home[0]"]
		if !ok || !ctx.homeHistoryValid || index >= len(ctx.lastGPSHome) {
			return value, nil
		}
		return value + ctx.lastGPSHome[index], nil
	case fieldPredictorHomeCoord1:
		index, ok := ctx.homeNameToIndex["GPS_home[1]"]
		if !ok || !ctx.homeHistoryValid || index >= len(ctx.lastGPSHome) {
			return value, nil
		}
		return value + ctx.lastGPSHome[index], nil
	case fieldPredictorLastMainFrameTime:
		if !ctx.mainHistoryValid {
			return value, nil
		}
		return value + ctx.lastMainFrameTime, nil
	case fieldPredictorIncrement:
		base := 1
		if previous != nil && fieldIndex < len(previous) {
			base += previous[fieldIndex]
		}
		return base, nil
	default:
		return 0, fmt.Errorf("unsupported predictor %d", predictor)
	}
}

func signExtend(value int, bits uint) int {
	shift := 32 - bits
	return (value << shift) >> shift
}

func decodeTag8_8SVB(data []byte, valueCount int) ([]int, int, error) {
	if valueCount <= 0 {
		return nil, 0, nil
	}
	values := make([]int, valueCount)
	offset := 0
	if valueCount == 1 {
		value, consumed, err := decodeSignedVB(data)
		if err != nil {
			return nil, 0, err
		}
		values[0] = value
		return values, consumed, nil
	}
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("truncated tag8_8svb header")
	}
	header := data[offset]
	offset++
	for i := 0; i < valueCount && i < 8; i++ {
		if header&(1<<i) == 0 {
			values[i] = 0
			continue
		}
		value, consumed, err := decodeSignedVB(data[offset:])
		if err != nil {
			return nil, 0, err
		}
		offset += consumed
		values[i] = value
	}
	return values, offset, nil
}

func decodeTag2_3S32(data []byte) ([]int, int, error) {
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("truncated tag2_3s32 header")
	}
	lead := int(data[0])
	offset := 1
	values := make([]int, 3)
	switch lead >> 6 {
	case 0:
		values[0] = signExtend((lead>>4)&0x03, 2)
		values[1] = signExtend((lead>>2)&0x03, 2)
		values[2] = signExtend(lead&0x03, 2)
	case 1:
		values[0] = signExtend(lead&0x0f, 4)
		if len(data[offset:]) < 1 {
			return nil, 0, fmt.Errorf("truncated tag2_3s32 4-bit payload")
		}
		lead2 := int(data[offset])
		offset++
		values[1] = signExtend((lead2>>4)&0x0f, 4)
		values[2] = signExtend(lead2&0x0f, 4)
	case 2:
		values[0] = signExtend(lead&0x3f, 6)
		if len(data[offset:]) < 2 {
			return nil, 0, fmt.Errorf("truncated tag2_3s32 6-bit payload")
		}
		values[1] = signExtend(int(data[offset])&0x3f, 6)
		offset++
		values[2] = signExtend(int(data[offset])&0x3f, 6)
		offset++
	case 3:
		selector := lead
		for i := 0; i < 3; i++ {
			var value int
			switch selector & 0x03 {
			case 0:
				if len(data[offset:]) < 1 {
					return nil, 0, fmt.Errorf("truncated tag2_3s32 8-bit payload")
				}
				value = signExtend(int(data[offset]), 8)
				offset++
			case 1:
				if len(data[offset:]) < 2 {
					return nil, 0, fmt.Errorf("truncated tag2_3s32 16-bit payload")
				}
				value = signExtend(int(data[offset])|(int(data[offset+1])<<8), 16)
				offset += 2
			case 2:
				if len(data[offset:]) < 3 {
					return nil, 0, fmt.Errorf("truncated tag2_3s32 24-bit payload")
				}
				value = signExtend(int(data[offset])|(int(data[offset+1])<<8)|(int(data[offset+2])<<16), 24)
				offset += 3
			case 3:
				if len(data[offset:]) < 4 {
					return nil, 0, fmt.Errorf("truncated tag2_3s32 32-bit payload")
				}
				value = int(uint32(data[offset]) | (uint32(data[offset+1]) << 8) | (uint32(data[offset+2]) << 16) | (uint32(data[offset+3]) << 24))
				offset += 4
			}
			values[i] = value
			selector >>= 2
		}
	default:
		return nil, 0, fmt.Errorf("unsupported tag2_3s32 selector")
	}
	return values, offset, nil
}

func decodeTag8_4S16V2(data []byte) ([]int, int, error) {
	if len(data) == 0 {
		return nil, 0, fmt.Errorf("truncated tag8_4s16 header")
	}
	const (
		fieldZero = iota
		field4Bit
		field8Bit
		field16Bit
	)
	selector := int(data[0])
	offset := 1
	values := make([]int, 4)
	nibbleIndex := 0
	buffer := 0
	for i := 0; i < 4; i++ {
		switch selector & 0x03 {
		case fieldZero:
			values[i] = 0
		case field4Bit:
			if nibbleIndex == 0 {
				if len(data[offset:]) < 1 {
					return nil, 0, fmt.Errorf("truncated tag8_4s16 4-bit payload")
				}
				buffer = int(data[offset])
				offset++
				values[i] = signExtend(buffer>>4, 4)
				nibbleIndex = 1
			} else {
				values[i] = signExtend(buffer&0x0f, 4)
				nibbleIndex = 0
			}
		case field8Bit:
			if nibbleIndex == 0 {
				if len(data[offset:]) < 1 {
					return nil, 0, fmt.Errorf("truncated tag8_4s16 8-bit payload")
				}
				values[i] = signExtend(int(data[offset]), 8)
				offset++
			} else {
				if len(data[offset:]) < 1 {
					return nil, 0, fmt.Errorf("truncated tag8_4s16 8-bit payload")
				}
				char1 := (buffer & 0x0f) << 4
				buffer = int(data[offset])
				offset++
				char1 |= buffer >> 4
				values[i] = signExtend(char1, 8)
			}
		case field16Bit:
			if nibbleIndex == 0 {
				if len(data[offset:]) < 2 {
					return nil, 0, fmt.Errorf("truncated tag8_4s16 16-bit payload")
				}
				values[i] = signExtend((int(data[offset])<<8)|int(data[offset+1]), 16)
				offset += 2
			} else {
				if len(data[offset:]) < 2 {
					return nil, 0, fmt.Errorf("truncated tag8_4s16 16-bit payload")
				}
				char1 := int(data[offset])
				char2 := int(data[offset+1])
				offset += 2
				values[i] = signExtend(((buffer&0x0f)<<12)|(char1<<4)|(char2>>4), 16)
				buffer = char2
			}
		}
		selector >>= 2
	}
	return values, offset, nil
}
