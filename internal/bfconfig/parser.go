package bfconfig

import (
	"strconv"
	"strings"

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
	OSD       []Command           `json:"osd"`
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
	PortIdentifier string   `json:"port_identifier"`
	FunctionMask   string   `json:"function_mask,omitempty"`
	BaudRates      []string `json:"baud_rates,omitempty"`
	Line           string   `json:"line"`
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
	}
	if len(fields) > 3 {
		serial.BaudRates = append(serial.BaudRates, fields[3:]...)
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
