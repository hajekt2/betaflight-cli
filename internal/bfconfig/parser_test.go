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
		"vtxtable channels 8",
		"vtxtable band 1 RACEBAND R FACTORY 5658 5695 5732 5769 5806 5843 5880 5917",
		"vtxtable powerlevels 2",
		"vtxtable powervalues 25 200",
		"vtxtable powerlabels 25 200",
		"led 0 0,0::C:0",
		"servo 0 1000 2000 1500 100 -1",
		"smix reverse 0 2 r",
		"adjrange 0 0 0 900 1300 12 0 0 0",
		"rxrange 0 1000 2000",
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
	if doc.Serial[0].FunctionMaskValue == nil || *doc.Serial[0].FunctionMaskValue != 64 || len(doc.Serial[0].Functions) != 1 || doc.Serial[0].Functions[0] != "RX_SERIAL" {
		t.Fatalf("serial functions = %+v", doc.Serial[0])
	}
	if doc.Serial[0].MSPBaudRate != "115200" || doc.Serial[0].TelemetryBaudRate != "0" {
		t.Fatalf("serial baud rates = %+v", doc.Serial[0])
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
	if len(doc.VTXTable) != 6 {
		t.Fatalf("vtx = %+v", doc.VTXTable)
	}
	if doc.VTX == nil || doc.VTX.Bands == nil || *doc.VTX.Bands != 5 || doc.VTX.Channels == nil || *doc.VTX.Channels != 8 {
		t.Fatalf("vtx summary = %+v", doc.VTX)
	}
	if len(doc.VTX.BandRows) != 1 || doc.VTX.BandRows[0].Name != "RACEBAND" || len(doc.VTX.BandRows[0].FrequenciesMHz) != 8 {
		t.Fatalf("vtx bands = %+v", doc.VTX.BandRows)
	}
	if len(doc.VTX.PowerValues) != 2 || doc.VTX.PowerValues[1] != 200 || len(doc.VTX.PowerLabels) != 2 {
		t.Fatalf("vtx power = %+v", doc.VTX)
	}
	if len(doc.LEDs) != 1 || doc.LEDs[0].Index != "0" || doc.LEDs[0].Value != "0,0::C:0" {
		t.Fatalf("leds = %+v", doc.LEDs)
	}
	if len(doc.Servos) != 1 || doc.Servos[0].Middle == nil || *doc.Servos[0].Middle != 1500 {
		t.Fatalf("servos = %+v", doc.Servos)
	}
	if len(doc.SMix) != 1 {
		t.Fatalf("smix = %+v", doc.SMix)
	}
	if len(doc.AdjRanges) != 1 || doc.AdjRanges[0].Function == nil || *doc.AdjRanges[0].Function != 12 {
		t.Fatalf("adjranges = %+v", doc.AdjRanges)
	}
	if len(doc.RXRanges) != 1 || doc.RXRanges[0].Max == nil || *doc.RXRanges[0].Max != 2000 {
		t.Fatalf("rxranges = %+v", doc.RXRanges)
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
