package bfconfig

import (
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/settings"
)

func TestParseDocument(t *testing.T) {
	doc := Parse([]string{
		"# version",
		"batch start",
		"profile 1",
		"rateprofile 2",
		"feature GPS",
		"feature -AIRMODE",
		"serial UART1 64 115200 57600 0 115200",
		"aux 0 0 0 1700 2100 0 0",
		"resource MOTOR 1 A00",
		"vtxtable bands 5",
		"set gyro_lpf1_static_hz = 0",
		"set imaginary_setting = value",
		"set osd_units = METRIC",
		"unknown stuff",
	}, settings.DefaultRegistry)

	if len(doc.Comments) != 1 {
		t.Fatalf("comments = %+v", doc.Comments)
	}
	if len(doc.Features) != 2 || !doc.Features[0].Enabled || doc.Features[1].Enabled {
		t.Fatalf("features = %+v", doc.Features)
	}
	if len(doc.Settings) != 3 {
		t.Fatalf("settings = %+v", doc.Settings)
	}
	if !doc.Settings[0].Known || doc.Settings[0].Type != "uint" {
		t.Fatalf("first setting = %+v", doc.Settings[0])
	}
	if doc.Settings[1].Known {
		t.Fatalf("unknown setting = %+v", doc.Settings[1])
	}
	if len(doc.Serial) != 1 || doc.Serial[0].PortIdentifier != "UART1" || len(doc.Serial[0].BaudRates) != 4 {
		t.Fatalf("serial = %+v", doc.Serial)
	}
	if len(doc.Aux) != 1 || doc.Aux[0].RangeStart == nil || *doc.Aux[0].RangeStart != 1700 {
		t.Fatalf("aux = %+v", doc.Aux)
	}
	if len(doc.Resources) != 1 || doc.Resources[0].Kind != "MOTOR" || doc.Resources[0].Target != "A00" {
		t.Fatalf("resources = %+v", doc.Resources)
	}
	if len(doc.Profiles) != 2 {
		t.Fatalf("profiles = %+v", doc.Profiles)
	}
	if len(doc.VTXTable) != 1 {
		t.Fatalf("vtx = %+v", doc.VTXTable)
	}
	if len(doc.OSD) != 1 {
		t.Fatalf("osd = %+v", doc.OSD)
	}
	if len(doc.Unknown) != 2 {
		t.Fatalf("unknown = %+v", doc.Unknown)
	}
	if len(doc.Sections["settings"]) != 3 || len(doc.Sections["unknown"]) != 2 {
		t.Fatalf("sections = %+v", doc.Sections)
	}
}
