package commands

import "testing"

func TestParseResourceDiagnostics(t *testing.T) {
	status := ParseResourceDiagnostics([]string{
		"# resources",
		"resource MOTOR 1 A00",
		"resource SERIAL_TX 1 A09",
		"timer A00 AF1",
		"timer A09 NONE",
		"dma pin A00 1",
		"dma SPI_TX 1 0",
		"unknown diagnostic line",
	})
	if len(status.Resources) != 2 {
		t.Fatalf("resources = %+v", status.Resources)
	}
	if status.Resources[0].Kind != "MOTOR" || status.Resources[0].Index != "1" || status.Resources[0].Target != "A00" {
		t.Fatalf("first resource = %+v", status.Resources[0])
	}
	if len(status.Timers) != 2 {
		t.Fatalf("timers = %+v", status.Timers)
	}
	if status.Timers[1].Pin != "A09" || !status.Timers[1].None {
		t.Fatalf("none timer = %+v", status.Timers[1])
	}
	if len(status.DMA) != 2 {
		t.Fatalf("dma = %+v", status.DMA)
	}
	if status.DMA[0].Scope != "pin" || status.DMA[0].Device != "A00" || status.DMA[0].Option != "1" {
		t.Fatalf("pin dma = %+v", status.DMA[0])
	}
	if status.DMA[1].Scope != "SPI_TX" || status.DMA[1].Device != "1" || status.DMA[1].Option != "0" {
		t.Fatalf("device dma = %+v", status.DMA[1])
	}
	if len(status.Comments) != 1 || len(status.Unparsed) != 1 {
		t.Fatalf("comments = %+v unparsed = %+v", status.Comments, status.Unparsed)
	}
}

func TestParseResourceDiagnosticsActiveTables(t *testing.T) {
	status := ParseResourceDiagnostics([]string{
		"Currently active IO resource assignments:",
		"(reboot to update)",
		"--------------------",
		"A00: SERIAL_TX 4",
		"A04: FREE",
		"Currently active Timers:",
		"TIM8:",
		"    CH1 : DSHOT_BITBANG 1",
		"TIM9: FREE",
		"Currently active DMA:",
		"DMA1 Stream 0: SPI_SDI 3",
		"DMA1 Stream 1: FREE",
	})
	if len(status.Resources) != 2 {
		t.Fatalf("resources = %+v", status.Resources)
	}
	if status.Resources[0].Target != "A00" || status.Resources[0].Kind != "SERIAL_TX" || status.Resources[0].Index != "4" {
		t.Fatalf("active resource = %+v", status.Resources[0])
	}
	if !status.Resources[1].Free {
		t.Fatalf("free resource = %+v", status.Resources[1])
	}
	if len(status.Timers) != 3 {
		t.Fatalf("timers = %+v", status.Timers)
	}
	if status.Timers[1].Timer != "TIM8" || status.Timers[1].Channel != "CH1" || status.Timers[1].Function != "DSHOT_BITBANG" || status.Timers[1].Index != "1" {
		t.Fatalf("timer channel = %+v", status.Timers[1])
	}
	if !status.Timers[2].Free {
		t.Fatalf("free timer = %+v", status.Timers[2])
	}
	if len(status.DMA) != 2 {
		t.Fatalf("dma = %+v", status.DMA)
	}
	if status.DMA[0].Controller != "DMA1" || status.DMA[0].Stream != "0" || status.DMA[0].Function != "SPI_SDI" || status.DMA[0].FunctionIndex != "3" {
		t.Fatalf("active dma = %+v", status.DMA[0])
	}
	if !status.DMA[1].Free {
		t.Fatalf("free dma = %+v", status.DMA[1])
	}
	if len(status.Unparsed) != 0 {
		t.Fatalf("unparsed = %+v", status.Unparsed)
	}
}
