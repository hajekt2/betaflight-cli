package bfconfig

import (
	"testing"

	"github.com/hajekt2/betaflight-cli/internal/settings"
)

func TestParseDocument(t *testing.T) {
	doc := Parse([]string{
		"# version",
		"batch start",
		"defaults nosave",
		"board_name JHEF405PRO",
		"manufacturer_id JHEF",
		"mcu_id 004c00223130500620353248",
		"signature",
		"profile 1",
		"rateprofile 2",
		"feature GPS",
		"feature -AIRMODE",
		"beeper -BAT_LOW",
		"beacon RX_SET",
		"serial UART1 64 115200 57600 0 115200",
		"aux 0 0 0 1700 2100 0 0",
		"resource MOTOR 1 A00",
		"timer B00 AF2",
		"timer B01 NONE",
		"dma ADC 1 0",
		"dma SPI_TX 1 NONE",
		"dma pin B00 0",
		"mixer QUADX",
		"mmix reset",
		"mmix 0 1.000 -1.000 1.000 -1.000",
		"map AETR1234",
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
		"rxfail 5 s 1800",
		"set gyro_lpf1_static_hz = 0",
		"set imaginary_setting = value",
		"set osd_units = METRIC",
		"save",
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
	if len(doc.Timers) != 2 || doc.Timers[0].Pin != "B00" || doc.Timers[0].AlternateFunction != "AF2" || doc.Timers[0].Raw != "timer B00 AF2" || doc.Timers[1].Pin != "B01" || !doc.Timers[1].None {
		t.Fatalf("timers/dma = %+v/%+v", doc.Timers, doc.DMA)
	}
	if len(doc.DMA) != 3 || doc.DMA[0].Scope != "ADC" || doc.DMA[0].Device != "1" || doc.DMA[0].Index != "" || doc.DMA[0].Option != "0" || doc.DMA[0].Raw != "dma ADC 1 0" || doc.DMA[1].Scope != "SPI_TX" || doc.DMA[1].Index != "" || !doc.DMA[1].None || doc.DMA[2].Scope != "pin" || doc.DMA[2].Device != "B00" || doc.DMA[2].Index != "" || doc.DMA[2].Option != "0" {
		t.Fatalf("dma = %+v", doc.DMA)
	}
	if len(doc.Mixer) != 1 || doc.Mixer[0].Name != "QUADX" || doc.Mixer[0].Raw != "mixer QUADX" {
		t.Fatalf("mixer/mmix/map = %+v/%+v/%+v", doc.Mixer, doc.MMix, doc.RCMap)
	}
	if len(doc.MMix) != 2 || !doc.MMix[0].Reset || doc.MMix[1].Index == nil || *doc.MMix[1].Index != 0 || doc.MMix[1].Throttle == nil || *doc.MMix[1].Throttle != 1 || doc.MMix[1].Roll == nil || *doc.MMix[1].Roll != -1 {
		t.Fatalf("mmix = %+v", doc.MMix)
	}
	if len(doc.RCMap) != 1 || doc.RCMap[0].Order != "AETR1234" || len(doc.RCMap[0].Channels) != 8 || doc.RCMap[0].Channels[0] != "A" || doc.RCMap[0].Channels[7] != "4" {
		t.Fatalf("map = %+v", doc.RCMap)
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
	if len(doc.RXFail) != 1 || doc.RXFail[0].Channel == nil || *doc.RXFail[0].Channel != 5 || doc.RXFail[0].Mode != "s" || doc.RXFail[0].Value == nil || *doc.RXFail[0].Value != 1800 {
		t.Fatalf("rxfail = %+v", doc.RXFail)
	}
	if len(doc.Beeper) != 1 || len(doc.Beacon) != 1 {
		t.Fatalf("beeper/beacon = %+v/%+v", doc.Beeper, doc.Beacon)
	}
	if len(doc.Board) != 4 {
		t.Fatalf("board = %+v", doc.Board)
	}
	if len(doc.Batch) != 1 || len(doc.Defaults) != 1 || len(doc.Save) != 1 {
		t.Fatalf("batch/defaults/save = %+v/%+v/%+v", doc.Batch, doc.Defaults, doc.Save)
	}
	if len(doc.OSD) != 1 {
		t.Fatalf("osd = %+v", doc.OSD)
	}
	if len(doc.Unknown) != 1 {
		t.Fatalf("unknown = %+v", doc.Unknown)
	}
	if len(doc.Sections["settings"]) != 3 || len(doc.Sections["unknown"]) != 1 || len(doc.Sections["board"]) != 4 {
		t.Fatalf("sections = %+v", doc.Sections)
	}
	inventory := BuildInventory(doc)
	if inventory.Settings != 3 || inventory.KnownSettings != 2 || inventory.UnknownSettings != 1 {
		t.Fatalf("inventory settings = %+v", inventory)
	}
	if len(inventory.FeaturesEnabled) != 1 || inventory.FeaturesEnabled[0] != "GPS" {
		t.Fatalf("inventory enabled features = %+v", inventory.FeaturesEnabled)
	}
	if len(inventory.FeaturesDisabled) != 1 || inventory.FeaturesDisabled[0] != "AIRMODE" {
		t.Fatalf("inventory disabled features = %+v", inventory.FeaturesDisabled)
	}
	if inventory.SerialPorts != 1 || inventory.AuxModes != 1 || inventory.VTXTableRows != 6 || inventory.UnknownRows != 1 {
		t.Fatalf("inventory counts = %+v", inventory)
	}
	if inventory.SectionCounts["settings"] != 3 || inventory.SectionCounts["board"] != 4 || inventory.SectionCounts["unknown"] != 1 {
		t.Fatalf("inventory sections = %+v", inventory.SectionCounts)
	}
	if len(inventory.NonEmptySections) == 0 || inventory.NonEmptySections[0] != "adjranges" {
		t.Fatalf("inventory non-empty sections = %+v", inventory.NonEmptySections)
	}
}
