package bfconfig

import (
	"strconv"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/bfserial"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

type Document struct {
	Sections  map[string][]string `json:"sections"`
	Commands  []Command           `json:"commands"`
	Settings  []Setting           `json:"settings"`
	Features  []Feature           `json:"features"`
	Serial    []Serial            `json:"serial"`
	Aux       []AuxRange          `json:"aux"`
	Resources []Resource          `json:"resources"`
	Profiles  []Profile           `json:"profiles"`
	VTXTable  []Command           `json:"vtx_table"`
	VTX       *VTXTableSummary    `json:"vtx,omitempty"`
	OSD       []Command           `json:"osd"`
	LEDs      []IndexedCommand    `json:"leds"`
	Servos    []Servo             `json:"servos"`
	SMix      []Command           `json:"smix"`
	AdjRanges []AdjustmentRange   `json:"adjranges"`
	RXRanges  []RXRange           `json:"rxranges"`
	RXFail    []RXFail            `json:"rxfail"`
	Beeper    []Command           `json:"beeper"`
	Beacon    []Command           `json:"beacon"`
	Board     []Command           `json:"board"`
	Batch     []Command           `json:"batch"`
	Defaults  []Command           `json:"defaults"`
	Save      []Command           `json:"save"`
	Comments  []string            `json:"comments"`
	Unknown   []string            `json:"unknown"`
}

type Command struct {
	Kind string   `json:"kind"`
	Line string   `json:"line"`
	Args []string `json:"args,omitempty"`
}

type Setting struct {
	Name     string `json:"name"`
	Value    string `json:"value"`
	Line     string `json:"line"`
	Known    bool   `json:"known"`
	Type     string `json:"type,omitempty"`
	Scope    string `json:"scope,omitempty"`
	Mode     string `json:"mode,omitempty"`
	PG       string `json:"pg,omitempty"`
	Metadata string `json:"metadata,omitempty"`
}

type Feature struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Line    string `json:"line"`
}

type Serial struct {
	PortIdentifier    string   `json:"port_identifier"`
	FunctionMask      string   `json:"function_mask,omitempty"`
	FunctionMaskValue *uint32  `json:"function_mask_value,omitempty"`
	Functions         []string `json:"functions,omitempty"`
	BaudRates         []string `json:"baud_rates,omitempty"`
	MSPBaudRate       string   `json:"msp_baudrate,omitempty"`
	GPSBaudRate       string   `json:"gps_baudrate,omitempty"`
	TelemetryBaudRate string   `json:"telemetry_baudrate,omitempty"`
	BlackboxBaudRate  string   `json:"blackbox_baudrate,omitempty"`
	Line              string   `json:"line"`
}

type VTXTableSummary struct {
	Bands       *int      `json:"bands,omitempty"`
	Channels    *int      `json:"channels,omitempty"`
	PowerLevels *int      `json:"power_levels,omitempty"`
	BandRows    []VTXBand `json:"band_rows,omitempty"`
	PowerValues []int     `json:"power_values,omitempty"`
	PowerLabels []string  `json:"power_labels,omitempty"`
	Lines       []string  `json:"lines,omitempty"`
}

type VTXBand struct {
	Index          *int   `json:"index,omitempty"`
	Name           string `json:"name,omitempty"`
	Letter         string `json:"letter,omitempty"`
	Factory        *bool  `json:"factory,omitempty"`
	FrequenciesMHz []int  `json:"frequencies_mhz,omitempty"`
	Line           string `json:"line"`
}

type AuxRange struct {
	Index       *int     `json:"index,omitempty"`
	ModeID      *int     `json:"mode_id,omitempty"`
	Channel     *int     `json:"channel,omitempty"`
	RangeStart  *int     `json:"range_start,omitempty"`
	RangeEnd    *int     `json:"range_end,omitempty"`
	ExtraFields []string `json:"extra_fields,omitempty"`
	Line        string   `json:"line"`
}

type Resource struct {
	Kind   string `json:"kind"`
	Index  string `json:"index,omitempty"`
	Target string `json:"target,omitempty"`
	Line   string `json:"line"`
}

type Profile struct {
	Kind  string `json:"kind"`
	Index string `json:"index,omitempty"`
	Line  string `json:"line"`
}

type IndexedCommand struct {
	Index string   `json:"index,omitempty"`
	Value string   `json:"value,omitempty"`
	Line  string   `json:"line"`
	Args  []string `json:"args,omitempty"`
}

type Servo struct {
	Index   *int   `json:"index,omitempty"`
	Min     *int   `json:"min,omitempty"`
	Max     *int   `json:"max,omitempty"`
	Middle  *int   `json:"middle,omitempty"`
	Rate    *int   `json:"rate,omitempty"`
	Forward *int   `json:"forward,omitempty"`
	Line    string `json:"line"`
}

type AdjustmentRange struct {
	Index         *int   `json:"index,omitempty"`
	Unused        *int   `json:"unused,omitempty"`
	RangeChannel  *int   `json:"range_channel,omitempty"`
	RangeStart    *int   `json:"range_start,omitempty"`
	RangeEnd      *int   `json:"range_end,omitempty"`
	Function      *int   `json:"function,omitempty"`
	SelectChannel *int   `json:"select_channel,omitempty"`
	Center        *int   `json:"center,omitempty"`
	Scale         *int   `json:"scale,omitempty"`
	Line          string `json:"line"`
}

type RXRange struct {
	Channel *int   `json:"channel,omitempty"`
	Min     *int   `json:"min,omitempty"`
	Max     *int   `json:"max,omitempty"`
	Line    string `json:"line"`
}

type RXFail struct {
	Channel *int     `json:"channel,omitempty"`
	Mode    string   `json:"mode,omitempty"`
	Value   *int     `json:"value,omitempty"`
	Args    []string `json:"args,omitempty"`
	Line    string   `json:"line"`
}

func Parse(lines []string, registry settings.Registry) Document {
	doc := Document{
		Sections: map[string][]string{
			"settings":  {},
			"profiles":  {},
			"serial":    {},
			"modes":     {},
			"features":  {},
			"resources": {},
			"vtx_table": {},
			"osd":       {},
			"leds":      {},
			"servos":    {},
			"smix":      {},
			"adjranges": {},
			"rxranges":  {},
			"rxfail":    {},
			"beeper":    {},
			"beacon":    {},
			"board":     {},
			"batch":     {},
			"defaults":  {},
			"save":      {},
			"unknown":   {},
		},
	}
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			doc.Comments = append(doc.Comments, trimmed)
			continue
		}
		fields := strings.Fields(trimmed)
		if len(fields) == 0 {
			continue
		}
		command := Command{Kind: fields[0], Line: trimmed}
		if len(fields) > 1 {
			command.Args = fields[1:]
		}
		doc.Commands = append(doc.Commands, command)
		classify(&doc, trimmed, fields, registry)
	}
	doc.VTX = parseVTXTableSummary(doc.VTXTable, doc.Sections["vtx_table"])
	return doc
}

func classify(doc *Document, line string, fields []string, registry settings.Registry) {
	switch {
	case strings.HasPrefix(line, "set "):
		doc.Sections["settings"] = append(doc.Sections["settings"], line)
		if setting, ok := parseSetting(line, registry); ok {
			doc.Settings = append(doc.Settings, setting)
			if strings.HasPrefix(setting.Name, "osd_") {
				doc.Sections["osd"] = append(doc.Sections["osd"], line)
				doc.OSD = append(doc.OSD, Command{Kind: fields[0], Line: line, Args: fields[1:]})
			}
		}
	case fields[0] == "profile" || fields[0] == "rateprofile":
		doc.Sections["profiles"] = append(doc.Sections["profiles"], line)
		profile := Profile{Kind: fields[0], Line: line}
		if len(fields) > 1 {
			profile.Index = fields[1]
		}
		doc.Profiles = append(doc.Profiles, profile)
	case fields[0] == "serial":
		doc.Sections["serial"] = append(doc.Sections["serial"], line)
		doc.Serial = append(doc.Serial, parseSerial(line, fields))
	case fields[0] == "aux":
		doc.Sections["modes"] = append(doc.Sections["modes"], line)
		doc.Aux = append(doc.Aux, parseAux(line, fields))
	case fields[0] == "mode_color" || fields[0] == "color":
		doc.Sections["modes"] = append(doc.Sections["modes"], line)
	case fields[0] == "feature":
		doc.Sections["features"] = append(doc.Sections["features"], line)
		if len(fields) > 1 {
			name := fields[1]
			enabled := true
			if strings.HasPrefix(name, "-") {
				name = strings.TrimPrefix(name, "-")
				enabled = false
			}
			doc.Features = append(doc.Features, Feature{Name: name, Enabled: enabled, Line: line})
		}
	case fields[0] == "resource":
		doc.Sections["resources"] = append(doc.Sections["resources"], line)
		doc.Resources = append(doc.Resources, parseResource(line, fields))
	case fields[0] == "vtxtable":
		doc.Sections["vtx_table"] = append(doc.Sections["vtx_table"], line)
		doc.VTXTable = append(doc.VTXTable, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case fields[0] == "led":
		doc.Sections["leds"] = append(doc.Sections["leds"], line)
		doc.LEDs = append(doc.LEDs, parseIndexedCommand(line, fields))
	case fields[0] == "servo":
		doc.Sections["servos"] = append(doc.Sections["servos"], line)
		doc.Servos = append(doc.Servos, parseServo(line, fields))
	case fields[0] == "smix":
		doc.Sections["smix"] = append(doc.Sections["smix"], line)
		doc.SMix = append(doc.SMix, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case fields[0] == "adjrange":
		doc.Sections["adjranges"] = append(doc.Sections["adjranges"], line)
		doc.AdjRanges = append(doc.AdjRanges, parseAdjustmentRange(line, fields))
	case fields[0] == "rxrange":
		doc.Sections["rxranges"] = append(doc.Sections["rxranges"], line)
		doc.RXRanges = append(doc.RXRanges, parseRXRange(line, fields))
	case fields[0] == "rxfail":
		doc.Sections["rxfail"] = append(doc.Sections["rxfail"], line)
		doc.RXFail = append(doc.RXFail, parseRXFail(line, fields))
	case fields[0] == "beeper":
		doc.Sections["beeper"] = append(doc.Sections["beeper"], line)
		doc.Beeper = append(doc.Beeper, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case fields[0] == "beacon":
		doc.Sections["beacon"] = append(doc.Sections["beacon"], line)
		doc.Beacon = append(doc.Beacon, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case fields[0] == "board_name" || fields[0] == "manufacturer_id" || fields[0] == "mcu_id" || fields[0] == "signature":
		doc.Sections["board"] = append(doc.Sections["board"], line)
		doc.Board = append(doc.Board, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case fields[0] == "batch":
		doc.Sections["batch"] = append(doc.Sections["batch"], line)
		doc.Batch = append(doc.Batch, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case fields[0] == "defaults":
		doc.Sections["defaults"] = append(doc.Sections["defaults"], line)
		doc.Defaults = append(doc.Defaults, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case fields[0] == "save":
		doc.Sections["save"] = append(doc.Sections["save"], line)
		doc.Save = append(doc.Save, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	case strings.HasPrefix(line, "osd_") || strings.HasPrefix(line, "set osd_"):
		doc.Sections["osd"] = append(doc.Sections["osd"], line)
		doc.OSD = append(doc.OSD, Command{Kind: fields[0], Line: line, Args: fields[1:]})
	default:
		doc.Sections["unknown"] = append(doc.Sections["unknown"], line)
		doc.Unknown = append(doc.Unknown, line)
	}
}

func parseSetting(line string, registry settings.Registry) (Setting, bool) {
	body := strings.TrimSpace(strings.TrimPrefix(line, "set "))
	parts := strings.SplitN(body, "=", 2)
	if len(parts) != 2 {
		return Setting{}, false
	}
	name := strings.TrimSpace(parts[0])
	value := strings.TrimSpace(parts[1])
	out := Setting{Name: name, Value: value, Line: line}
	if metadata, ok := registry.Lookup(name); ok {
		out.Known = true
		out.Type = string(metadata.Type)
		out.Scope = metadata.Scope
		out.Mode = metadata.Mode
		out.PG = metadata.PG
		out.Metadata = metadata.Name
	}
	return out, true
}

func parseSerial(line string, fields []string) Serial {
	serial := Serial{Line: line}
	if len(fields) > 1 {
		serial.PortIdentifier = fields[1]
	}
	if len(fields) > 2 {
		serial.FunctionMask = fields[2]
		if mask, err := strconv.ParseUint(fields[2], 0, 32); err == nil {
			value := uint32(mask)
			serial.FunctionMaskValue = &value
			serial.Functions = bfserial.FunctionNames(value)
		}
	}
	if len(fields) > 3 {
		serial.BaudRates = append(serial.BaudRates, fields[3:]...)
	}
	if len(fields) > 3 {
		serial.MSPBaudRate = fields[3]
	}
	if len(fields) > 4 {
		serial.GPSBaudRate = fields[4]
	}
	if len(fields) > 5 {
		serial.TelemetryBaudRate = fields[5]
	}
	if len(fields) > 6 {
		serial.BlackboxBaudRate = fields[6]
	}
	return serial
}

func parseAux(line string, fields []string) AuxRange {
	aux := AuxRange{Line: line}
	ptrs := []*int{nil, nil, nil, nil, nil}
	for i := 1; i < len(fields) && i <= len(ptrs); i++ {
		if n, err := strconv.Atoi(fields[i]); err == nil {
			value := n
			ptrs[i-1] = &value
		} else {
			aux.ExtraFields = append(aux.ExtraFields, fields[i])
		}
	}
	aux.Index = ptrs[0]
	aux.ModeID = ptrs[1]
	aux.Channel = ptrs[2]
	aux.RangeStart = ptrs[3]
	aux.RangeEnd = ptrs[4]
	if len(fields) > 6 {
		aux.ExtraFields = append(aux.ExtraFields, fields[6:]...)
	}
	return aux
}

func parseResource(line string, fields []string) Resource {
	resource := Resource{Line: line}
	if len(fields) > 1 {
		resource.Kind = fields[1]
	}
	if len(fields) > 2 {
		resource.Index = fields[2]
	}
	if len(fields) > 3 {
		resource.Target = fields[3]
	}
	return resource
}

func parseIndexedCommand(line string, fields []string) IndexedCommand {
	out := IndexedCommand{Line: line}
	if len(fields) > 1 {
		out.Index = fields[1]
	}
	if len(fields) > 2 {
		out.Value = strings.Join(fields[2:], " ")
		out.Args = fields[1:]
	}
	return out
}

func parseServo(line string, fields []string) Servo {
	out := Servo{Line: line}
	values := parseIntFields(fields[1:], 6)
	out.Index = values[0]
	out.Min = values[1]
	out.Max = values[2]
	out.Middle = values[3]
	out.Rate = values[4]
	out.Forward = values[5]
	return out
}

func parseAdjustmentRange(line string, fields []string) AdjustmentRange {
	out := AdjustmentRange{Line: line}
	values := parseIntFields(fields[1:], 9)
	out.Index = values[0]
	out.Unused = values[1]
	out.RangeChannel = values[2]
	out.RangeStart = values[3]
	out.RangeEnd = values[4]
	out.Function = values[5]
	out.SelectChannel = values[6]
	out.Center = values[7]
	out.Scale = values[8]
	return out
}

func parseRXRange(line string, fields []string) RXRange {
	out := RXRange{Line: line}
	values := parseIntFields(fields[1:], 3)
	out.Channel = values[0]
	out.Min = values[1]
	out.Max = values[2]
	return out
}

func parseRXFail(line string, fields []string) RXFail {
	out := RXFail{Line: line}
	values := parseIntFields(fields[1:], 3)
	out.Channel = values[0]
	if len(fields) > 2 {
		out.Mode = fields[2]
	}
	out.Value = values[2]
	if len(fields) > 1 {
		out.Args = fields[1:]
	}
	return out
}

func parseIntFields(fields []string, count int) []*int {
	parsed := make([]*int, count)
	for i := 0; i < len(fields) && i < len(parsed); i++ {
		if n, err := strconv.Atoi(fields[i]); err == nil {
			value := n
			parsed[i] = &value
		}
	}
	return parsed
}

func parseVTXTableSummary(commands []Command, lines []string) *VTXTableSummary {
	if len(commands) == 0 && len(lines) == 0 {
		return nil
	}
	summary := &VTXTableSummary{Lines: append([]string(nil), lines...)}
	for _, command := range commands {
		if len(command.Args) == 0 {
			continue
		}
		switch command.Args[0] {
		case "bands":
			summary.Bands = firstInt(command.Args[1:])
		case "channels":
			summary.Channels = firstInt(command.Args[1:])
		case "powerlevels":
			summary.PowerLevels = firstInt(command.Args[1:])
		case "band":
			summary.BandRows = append(summary.BandRows, parseVTXBand(command))
		case "powervalues":
			summary.PowerValues = parseIntSlice(command.Args[1:])
		case "powerlabels":
			summary.PowerLabels = append([]string(nil), command.Args[1:]...)
		}
	}
	return summary
}

func parseVTXBand(command Command) VTXBand {
	band := VTXBand{Line: command.Line}
	args := command.Args
	if len(args) > 1 {
		band.Index = firstInt(args[1:2])
	}
	if len(args) > 2 {
		band.Name = args[2]
	}
	if len(args) > 3 {
		band.Letter = args[3]
	}
	start := 4
	if len(args) > 4 && (args[4] == "FACTORY" || args[4] == "CUSTOM") {
		factory := args[4] == "FACTORY"
		band.Factory = &factory
		start = 5
	}
	if len(args) > start {
		band.FrequenciesMHz = parseIntSlice(args[start:])
	}
	return band
}

func firstInt(fields []string) *int {
	if len(fields) == 0 {
		return nil
	}
	if n, err := strconv.Atoi(fields[0]); err == nil {
		value := n
		return &value
	}
	return nil
}

func parseIntSlice(fields []string) []int {
	out := []int{}
	for _, field := range fields {
		if n, err := strconv.Atoi(field); err == nil {
			out = append(out, n)
		}
	}
	return out
}
