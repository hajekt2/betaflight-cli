package blackbox

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strconv"
	"strings"
)

type Inspection struct {
	SizeBytes               int64                      `json:"size_bytes"`
	HeaderBytes             int64                      `json:"header_bytes"`
	DataBytes               int64                      `json:"data_bytes"`
	Product                 string                     `json:"product,omitempty"`
	FirmwareRevision        string                     `json:"firmware_revision,omitempty"`
	FirmwareDate            string                     `json:"firmware_date,omitempty"`
	BoardInformation        string                     `json:"board_information,omitempty"`
	CraftName               string                     `json:"craft_name,omitempty"`
	LogStartDatetime        string                     `json:"log_start_datetime,omitempty"`
	Headers                 map[string]string          `json:"headers"`
	HeaderOrder             []string                   `json:"header_order,omitempty"`
	FieldDefinitions        map[string]FieldDefinition `json:"field_definitions"`
	FrameMarkerCountsApprox map[string]int             `json:"frame_marker_counts_approx"`
	FrameSummaryApprox      FrameSummary               `json:"frame_summary_approx"`
	DecodedFrames           DecodedFrameSummary        `json:"decoded_frames"`
	Warnings                []string                   `json:"warnings,omitempty"`
}

type FieldDefinition struct {
	Frame     string   `json:"frame"`
	Names     []string `json:"names,omitempty"`
	Signed    []string `json:"signed,omitempty"`
	Predictor []string `json:"predictor,omitempty"`
	Encoding  []string `json:"encoding,omitempty"`
}

type FrameSummary struct {
	CandidateCount int                  `json:"candidate_count"`
	IndexedCount   int                  `json:"indexed_count"`
	Truncated      bool                 `json:"truncated"`
	ByType         map[string]FrameStat `json:"by_type"`
	Candidates     []FrameCandidate     `json:"candidates,omitempty"`
}

type FrameStat struct {
	Count       int   `json:"count"`
	FirstOffset int64 `json:"first_offset"`
	LastOffset  int64 `json:"last_offset"`
	MinSpan     int64 `json:"min_span,omitempty"`
	MaxSpan     int64 `json:"max_span,omitempty"`
}

type FrameCandidate struct {
	Type        string `json:"type"`
	Offset      int64  `json:"offset"`
	DataOffset  int64  `json:"data_offset"`
	BytesToNext int64  `json:"bytes_to_next,omitempty"`
}

type DecodedFrameSummary struct {
	AttemptedCount        int                    `json:"attempted_count"`
	DecodedCount          int                    `json:"decoded_count"`
	FailedCount           int                    `json:"failed_count"`
	Truncated             bool                   `json:"truncated"`
	UnsupportedEncodings  map[string]int         `json:"unsupported_encodings,omitempty"`
	UnsupportedFrameTypes map[string]int         `json:"unsupported_frame_types,omitempty"`
	ByType                map[string]DecodedStat `json:"by_type"`
	Streams               map[string]StreamStat  `json:"streams,omitempty"`
	Groups                map[string]StreamGroup `json:"groups,omitempty"`
	Samples               []DecodedFrame         `json:"samples,omitempty"`
	Warnings              []string               `json:"warnings,omitempty"`
}

type DecodedStat struct {
	Attempted int `json:"attempted"`
	Decoded   int `json:"decoded"`
	Failed    int `json:"failed"`
}

type DecodedFrame struct {
	Type       string         `json:"type"`
	Offset     int64          `json:"offset"`
	DataOffset int64          `json:"data_offset"`
	Values     map[string]int `json:"values,omitempty"`
	Error      string         `json:"error,omitempty"`
}

type StreamStat struct {
	Field      string `json:"field"`
	FrameType  string `json:"frame_type"`
	Count      int    `json:"count"`
	First      int    `json:"first"`
	Last       int    `json:"last"`
	Min        int    `json:"min"`
	Max        int    `json:"max"`
	Delta      int    `json:"delta"`
	Monotonic  bool   `json:"monotonic"`
	LastOffset int64  `json:"last_offset"`
}

type StreamGroup struct {
	Name    string       `json:"name"`
	Fields  []string     `json:"fields"`
	Streams []StreamStat `json:"streams"`
}

func Inspect(r io.Reader) (Inspection, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return Inspection{}, err
	}
	if len(data) == 0 {
		return Inspection{}, fmt.Errorf("blackbox log is empty")
	}
	out := Inspection{
		SizeBytes:               int64(len(data)),
		Headers:                 map[string]string{},
		FieldDefinitions:        map[string]FieldDefinition{},
		FrameMarkerCountsApprox: map[string]int{},
	}
	headerEnd, err := parseHeaders(data, &out)
	if err != nil {
		return Inspection{}, err
	}
	out.HeaderBytes = int64(headerEnd)
	out.DataBytes = int64(len(data) - headerEnd)
	out.FrameMarkerCountsApprox = countFrameMarkers(data[headerEnd:])
	out.FrameSummaryApprox = summarizeFrameCandidates(data[headerEnd:], int64(headerEnd), 200)
	out.DecodedFrames = decodeFrames(data[headerEnd:], int64(headerEnd), out.Headers, out.FieldDefinitions, out.FrameSummaryApprox, 50)
	out.Warnings = validationWarnings(out)
	return out, nil
}

func parseHeaders(data []byte, out *Inspection) (int, error) {
	offset := 0
	sawHeader := false
	for offset < len(data) {
		lineEnd := bytes.IndexByte(data[offset:], '\n')
		if lineEnd < 0 {
			break
		}
		lineEnd += offset
		line := strings.TrimSuffix(string(data[offset:lineEnd]), "\r")
		nextOffset := lineEnd + 1
		if !strings.HasPrefix(line, "H ") {
			if !sawHeader {
				offset = nextOffset
				continue
			}
			return offset, nil
		}
		sawHeader = true
		parseHeaderLine(line, out)
		offset = nextOffset
	}
	if !sawHeader {
		return 0, fmt.Errorf("blackbox header not found")
	}
	return offset, nil
}

func parseHeaderLine(line string, out *Inspection) {
	body := strings.TrimPrefix(line, "H ")
	name, value, ok := strings.Cut(body, ":")
	if !ok {
		out.Warnings = append(out.Warnings, fmt.Sprintf("ignored malformed header line %q", line))
		return
	}
	name = strings.TrimSpace(name)
	value = strings.TrimSpace(value)
	if _, exists := out.Headers[name]; !exists {
		out.HeaderOrder = append(out.HeaderOrder, name)
	}
	out.Headers[name] = value
	assignCommonHeader(out, name, value)
	if strings.HasPrefix(name, "Field ") {
		parseFieldDefinition(name, value, out)
	}
}

func assignCommonHeader(out *Inspection, name, value string) {
	switch name {
	case "Product":
		out.Product = value
	case "Firmware revision":
		out.FirmwareRevision = value
	case "Firmware date":
		out.FirmwareDate = value
	case "Board information":
		out.BoardInformation = value
	case "Craft name":
		out.CraftName = value
	case "Log start datetime":
		out.LogStartDatetime = value
	}
}

func parseFieldDefinition(name, value string, out *Inspection) {
	parts := strings.Fields(name)
	if len(parts) < 3 {
		out.Warnings = append(out.Warnings, fmt.Sprintf("ignored malformed field definition %q", name))
		return
	}
	frame := parts[1]
	info := strings.Join(parts[2:], " ")
	def := out.FieldDefinitions[frame]
	if def.Frame == "" {
		def.Frame = frame
	}
	values := splitCSV(value)
	switch info {
	case "name":
		def.Names = values
	case "signed":
		def.Signed = values
	case "predictor":
		def.Predictor = values
	case "encoding":
		def.Encoding = values
	default:
		out.Warnings = append(out.Warnings, fmt.Sprintf("ignored unsupported field definition %q", name))
	}
	out.FieldDefinitions[frame] = def
}

func splitCSV(value string) []string {
	if value == "" {
		return nil
	}
	parts := strings.Split(value, ",")
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func countFrameMarkers(data []byte) map[string]int {
	counts := map[string]int{}
	for _, marker := range []byte{'I', 'P', 'G', 'H', 'S', 'E'} {
		counts[string(marker)] = 0
	}
	for _, b := range data {
		if _, ok := counts[string(b)]; ok {
			counts[string(b)]++
		}
	}
	return counts
}

func summarizeFrameCandidates(data []byte, headerBytes int64, maxIndex int) FrameSummary {
	types := map[byte]bool{'I': true, 'P': true, 'G': true, 'H': true, 'S': true, 'E': true}
	summary := FrameSummary{
		ByType: map[string]FrameStat{},
	}
	var previousIndex int
	previousCandidateIndex := -1
	for i, b := range data {
		if !types[b] {
			continue
		}
		summary.CandidateCount++
		if previousCandidateIndex >= 0 {
			span := int64(i - previousIndex)
			summary.Candidates[previousCandidateIndex].BytesToNext = span
			stat := summary.ByType[summary.Candidates[previousCandidateIndex].Type]
			if stat.MinSpan == 0 || span < stat.MinSpan {
				stat.MinSpan = span
			}
			if span > stat.MaxSpan {
				stat.MaxSpan = span
			}
			summary.ByType[summary.Candidates[previousCandidateIndex].Type] = stat
		}
		candidate := FrameCandidate{
			Type:       string(b),
			Offset:     headerBytes + int64(i),
			DataOffset: int64(i),
		}
		if summary.IndexedCount < maxIndex {
			summary.Candidates = append(summary.Candidates, candidate)
			previousCandidateIndex = len(summary.Candidates) - 1
		} else {
			previousCandidateIndex = -1
			summary.Truncated = true
		}
		summary.IndexedCount = len(summary.Candidates)
		stat := summary.ByType[candidate.Type]
		if stat.Count == 0 {
			stat.FirstOffset = candidate.Offset
		}
		stat.Count++
		stat.LastOffset = candidate.Offset
		summary.ByType[candidate.Type] = stat
		previousIndex = i
	}
	return summary
}

func decodeFrames(data []byte, headerBytes int64, headers map[string]string, definitions map[string]FieldDefinition, candidates FrameSummary, maxSamples int) DecodedFrameSummary {
	out := DecodedFrameSummary{
		UnsupportedEncodings:  map[string]int{},
		UnsupportedFrameTypes: map[string]int{},
		ByType:                map[string]DecodedStat{},
		Streams:               map[string]StreamStat{},
	}
	ctx := newDecodeContext(headers, definitions)
	for i, candidate := range candidates.Candidates {
		if candidate.BytesToNext == 0 || candidate.DataOffset < 0 || candidate.DataOffset >= int64(len(data)) {
			continue
		}
		if out.AttemptedCount >= maxSamples {
			out.Truncated = true
			break
		}
		nextOffset := candidate.DataOffset + candidate.BytesToNext
		if nextOffset > int64(len(data)) {
			nextOffset = int64(len(data))
		}
		payloadStart := candidate.DataOffset + 1
		if payloadStart > nextOffset {
			payloadStart = nextOffset
		}
		payload := data[payloadStart:nextOffset]
		frameType := candidate.Type
		def, ok := definitionForFrame(frameType, definitions)
		if !ok {
			out.UnsupportedFrameTypes[frameType]++
			out.recordFailed(frameType, candidate, "missing field definition")
			continue
		}
		out.AttemptedCount++
		decoded, err := decodeFramePayload(&ctx, frameType, candidate, payload, def)
		if err != nil {
			out.recordFailed(frameType, candidate, err.Error())
			if encoding, ok := unsupportedEncodingValue(err.Error()); ok {
				out.UnsupportedEncodings[encoding]++
			}
			continue
		}
		out.DecodedCount++
		stat := out.ByType[frameType]
		stat.Attempted++
		stat.Decoded++
		out.ByType[frameType] = stat
		out.recordStreams(decoded)
		out.Samples = append(out.Samples, decoded)
		if i == len(candidates.Candidates)-1 && candidates.Truncated {
			out.Truncated = true
		}
	}
	out.FailedCount = out.AttemptedCount - out.DecodedCount
	if len(out.UnsupportedEncodings) == 0 {
		out.UnsupportedEncodings = nil
	}
	if len(out.UnsupportedFrameTypes) == 0 {
		out.UnsupportedFrameTypes = nil
	}
	if len(out.Streams) == 0 {
		out.Streams = nil
	} else {
		out.Groups = groupStreams(out.Streams)
	}
	for frameType, stat := range out.ByType {
		if stat.Failed > 0 {
			out.Warnings = append(out.Warnings, fmt.Sprintf("failed to decode %d %s frame samples", stat.Failed, frameType))
		}
	}
	sort.Strings(out.Warnings)
	return out
}

func groupStreams(streams map[string]StreamStat) map[string]StreamGroup {
	groups := map[string]StreamGroup{}
	keys := make([]string, 0, len(streams))
	for key := range streams {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		stat := streams[key]
		groupName := classifyStreamField(stat.Field)
		if groupName == "" {
			continue
		}
		group := groups[groupName]
		if group.Name == "" {
			group.Name = groupName
		}
		group.Fields = append(group.Fields, stat.Field)
		group.Streams = append(group.Streams, stat)
		groups[groupName] = group
	}
	if len(groups) == 0 {
		return nil
	}
	for name, group := range groups {
		sort.Strings(group.Fields)
		sort.Slice(group.Streams, func(i, j int) bool {
			if group.Streams[i].FrameType == group.Streams[j].FrameType {
				return group.Streams[i].Field < group.Streams[j].Field
			}
			return group.Streams[i].FrameType < group.Streams[j].FrameType
		})
		groups[name] = group
	}
	return groups
}

func classifyStreamField(field string) string {
	normalized := strings.ToLower(field)
	switch {
	case normalized == "time" || normalized == "looptime" || normalized == "loopiteration" || strings.Contains(normalized, "iteration"):
		return "timing"
	case strings.HasPrefix(normalized, "gyro") || strings.Contains(normalized, "gyro"):
		return "gyro"
	case strings.HasPrefix(normalized, "acc") || strings.Contains(normalized, "accsmooth"):
		return "accelerometer"
	case strings.HasPrefix(normalized, "motor") || strings.HasPrefix(normalized, "motor["):
		return "motors"
	case strings.HasPrefix(normalized, "rccommand") || strings.HasPrefix(normalized, "rccommands") || strings.HasPrefix(normalized, "rccommand["):
		return "rc_command"
	case strings.HasPrefix(normalized, "setpoint"):
		return "setpoint"
	case strings.HasPrefix(normalized, "axisp") || strings.HasPrefix(normalized, "axisi") || strings.HasPrefix(normalized, "axisd") || strings.HasPrefix(normalized, "axisf"):
		return "pid"
	case strings.HasPrefix(normalized, "pid"):
		return "pid"
	case strings.HasPrefix(normalized, "attitude") || strings.Contains(normalized, "heading"):
		return "attitude"
	case strings.Contains(normalized, "vbat") || strings.Contains(normalized, "amperage") || strings.Contains(normalized, "current") || strings.Contains(normalized, "mah"):
		return "battery"
	case strings.Contains(normalized, "rssi") || strings.Contains(normalized, "linkquality") || strings.Contains(normalized, "rxsignal"):
		return "radio_link"
	default:
		return ""
	}
}

func (out *DecodedFrameSummary) recordStreams(frame DecodedFrame) {
	for field, value := range frame.Values {
		key := frame.Type + "." + field
		stat := out.Streams[key]
		if stat.Count == 0 {
			stat = StreamStat{
				Field:     field,
				FrameType: frame.Type,
				First:     value,
				Min:       value,
				Max:       value,
				Monotonic: true,
			}
		}
		if value < stat.Min {
			stat.Min = value
		}
		if value > stat.Max {
			stat.Max = value
		}
		if stat.Count > 0 && value < stat.Last {
			stat.Monotonic = false
		}
		stat.Count++
		stat.Last = value
		stat.Delta = stat.Last - stat.First
		stat.LastOffset = frame.Offset
		out.Streams[key] = stat
	}
}

func (out *DecodedFrameSummary) recordFailed(frameType string, candidate FrameCandidate, message string) {
	out.FailedCount++
	stat := out.ByType[frameType]
	stat.Attempted++
	stat.Failed++
	out.ByType[frameType] = stat
	out.Samples = append(out.Samples, DecodedFrame{
		Type:       frameType,
		Offset:     candidate.Offset,
		DataOffset: candidate.DataOffset,
		Error:      message,
	})
}

func definitionForFrame(frameType string, definitions map[string]FieldDefinition) (FieldDefinition, bool) {
	if def, ok := definitions[frameType]; ok {
		if frameType == "P" && len(def.Names) == 0 {
			if iDef, ok := definitions["I"]; ok {
				def.Names = iDef.Names
				def.Signed = iDef.Signed
				def.Predictor = iDef.Predictor
			}
		}
		return def, true
	}
	if frameType == "P" {
		if def, ok := definitions["I"]; ok {
			pDef := definitions["P"]
			pDef.Frame = "P"
			pDef.Names = def.Names
			pDef.Signed = def.Signed
			pDef.Predictor = def.Predictor
			return pDef, true
		}
	}
	return FieldDefinition{}, false
}

func decodeFramePayload(ctx *decodeContext, frameType string, candidate FrameCandidate, payload []byte, def FieldDefinition) (DecodedFrame, error) {
	values := map[string]int{}
	current := make([]int, len(def.Encoding))
	var previous []int
	var previous2 []int
	switch frameType {
	case "I":
		previous = ctx.lastMain
		previous2 = ctx.lastMain2
	case "P":
		previous = ctx.lastMain
		previous2 = ctx.lastMain2
	case "H":
		previous = ctx.lastGPSHome
	case "G":
		previous = ctx.lastGPS
	case "S":
		previous = ctx.lastSlow
	}
	offset := 0
	fieldCount := len(def.Encoding)
	if len(def.Names) > 0 && len(def.Names) < fieldCount {
		fieldCount = len(def.Names)
	}
	if len(def.Predictor) > 0 && len(def.Predictor) < fieldCount {
		fieldCount = len(def.Predictor)
	}
	for i := 0; i < fieldCount; i++ {
		groupValues, consumed, span, err := decodeFieldValue(payload[offset:], def.Encoding, i, signedField(def, i), ctx.dataVersion)
		if err != nil {
			return DecodedFrame{}, err
		}
		offset += consumed
		for j, rawValue := range groupValues {
			index := i + j
			if index >= fieldCount {
				break
			}
			fieldName := fmt.Sprintf("field_%d", index)
			if index < len(def.Names) && def.Names[index] != "" {
				fieldName = def.Names[index]
			}
			fieldPredictor := fieldPredictorZero
			if index < len(def.Predictor) {
				fieldPredictor = parsePredictor(def.Predictor[index])
			}
			decodedValue, err := applyPrediction(ctx, frameType, index, fieldPredictor, rawValue, current, previous, previous2)
			if err != nil {
				return DecodedFrame{}, err
			}
			current[index] = decodedValue
			values[fieldName] = decodedValue
		}
		i += span - 1
	}
	if offset < len(payload) {
		return DecodedFrame{}, fmt.Errorf("decoded %d of %d payload bytes", offset, len(payload))
	}
	switch frameType {
	case "I":
		ctx.lastMain2 = ctx.lastMain
		ctx.lastMain = append([]int(nil), current...)
		if index, ok := ctx.mainNameToIndex["time"]; ok && index < len(current) {
			ctx.lastMainFrameTime = current[index]
		}
		ctx.mainHistoryValid = true
	case "P":
		ctx.lastMain2 = ctx.lastMain
		ctx.lastMain = append([]int(nil), current...)
		if index, ok := ctx.mainNameToIndex["time"]; ok && index < len(current) {
			ctx.lastMainFrameTime = current[index]
		}
		ctx.mainHistoryValid = true
	case "H":
		ctx.lastGPSHome = append([]int(nil), current...)
		ctx.homeHistoryValid = true
	case "G":
		ctx.lastGPS = append([]int(nil), current...)
	case "S":
		ctx.lastSlow = append([]int(nil), current...)
	}
	return DecodedFrame{
		Type:       frameType,
		Offset:     candidate.Offset,
		DataOffset: candidate.DataOffset,
		Values:     values,
	}, nil
}

func unsupportedEncodingValue(message string) (string, bool) {
	const prefix = "unsupported encoding "
	if !strings.HasPrefix(message, prefix) {
		return "", false
	}
	return strings.TrimPrefix(message, prefix), true
}

func parsePredictor(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return fieldPredictorZero
	}
	return n
}

func signedField(def FieldDefinition, index int) bool {
	if index >= len(def.Signed) {
		return false
	}
	return def.Signed[index] == "1"
}

func decodeFieldValue(data []byte, encodings []string, index int, signed bool, dataVersion int) ([]int, int, int, error) {
	if index >= len(encodings) {
		return nil, 0, 0, fmt.Errorf("missing field encoding")
	}
	encoding := strings.TrimSpace(encodings[index])
	switch encoding {
	case "0":
		value, consumed, err := decodeSignedVB(data)
		return []int{value}, consumed, 1, err
	case "1":
		value, consumed, err := decodeUnsignedVB(data)
		if err != nil {
			return nil, 0, 0, err
		}
		if signed {
			return []int{zigzagDecode(value)}, consumed, 1, nil
		}
		return []int{int(value)}, consumed, 1, nil
	case "3":
		value, consumed, err := decodeUnsignedVB(data)
		if err != nil {
			return nil, 0, 0, err
		}
		return []int{-signExtend(int(value), 14)}, consumed, 1, nil
	case "6":
		groupCount := 1
		for j := index + 1; j < len(encodings) && j < index+8; j++ {
			if strings.TrimSpace(encodings[j]) != encoding {
				break
			}
			groupCount++
		}
		values, consumed, err := decodeTag8_8SVB(data, groupCount)
		if err != nil {
			return nil, 0, 0, err
		}
		return values, consumed, len(values), nil
	case "7":
		values, consumed, err := decodeTag2_3S32(data)
		if err != nil {
			return nil, 0, 0, err
		}
		return values, consumed, len(values), nil
	case "8":
		if dataVersion < 2 {
			return nil, 0, 0, fmt.Errorf("unsupported encoding %s", encoding)
		}
		values, consumed, err := decodeTag8_4S16V2(data)
		if err != nil {
			return nil, 0, 0, err
		}
		return values, consumed, len(values), nil
	case "9":
		return []int{0}, 0, 1, nil
	case "10":
		return nil, 0, 0, fmt.Errorf("unsupported encoding %s", encoding)
	default:
		return nil, 0, 0, fmt.Errorf("unsupported encoding %s", encoding)
	}
}

func decodeUnsignedVB(data []byte) (uint32, int, error) {
	var value uint32
	for i := 0; i < len(data) && i < 5; i++ {
		b := data[i]
		value |= uint32(b&0x7f) << (7 * i)
		if b&0x80 == 0 {
			return value, i + 1, nil
		}
	}
	return 0, 0, fmt.Errorf("truncated variable-byte integer")
}

func decodeSignedVB(data []byte) (int, int, error) {
	value, consumed, err := decodeUnsignedVB(data)
	if err != nil {
		return 0, 0, err
	}
	return zigzagDecode(value), consumed, nil
}

func zigzagDecode(value uint32) int {
	n := int(value >> 1)
	if value&1 == 0 {
		return n
	}
	return -n - 1
}

func validationWarnings(in Inspection) []string {
	warnings := append([]string{}, in.Warnings...)
	if in.Product == "" {
		warnings = append(warnings, "missing Product header")
	} else if !strings.Contains(strings.ToLower(in.Product), "blackbox") {
		warnings = append(warnings, "Product header does not look like a Blackbox log")
	}
	if _, ok := in.FieldDefinitions["I"]; !ok {
		warnings = append(warnings, "missing I frame field definitions")
	}
	if _, ok := in.FieldDefinitions["P"]; !ok {
		warnings = append(warnings, "missing P frame field definitions")
	}
	for frame, def := range in.FieldDefinitions {
		if len(def.Names) == 0 && frame != "P" {
			warnings = append(warnings, fmt.Sprintf("missing Field %s name header", frame))
		}
		if len(def.Predictor) == 0 {
			warnings = append(warnings, fmt.Sprintf("missing Field %s predictor header", frame))
		}
		if len(def.Encoding) == 0 {
			warnings = append(warnings, fmt.Sprintf("missing Field %s encoding header", frame))
		}
	}
	sort.Strings(warnings)
	return warnings
}
