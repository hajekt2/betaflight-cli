package blackbox

import (
	"bytes"
	"fmt"
	"io"
	"sort"
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
