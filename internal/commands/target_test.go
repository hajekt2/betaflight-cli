package commands

import "testing"

func TestBuildTargetSummary(t *testing.T) {
	firmware := &FirmwareStatus{
		Variant: "BTFL",
		Version: "2025.12.1",
		MSPAPI:  "1.48",
		Support: FirmwareSupport{
			Supported: true,
		},
		Target: FirmwareTarget{
			TargetName:     "STM32F405",
			BoardName:      "FAKEF405",
			ManufacturerID: "FAKE",
		},
		MCU: &MCUInfo{Name: "STM32F405"},
		Identity: FirmwareIdentity{
			ConfigurationStateName: "CONFIGURED",
		},
	}
	system := &SystemStatus{
		Config:    &SystemConfigLine{State: "CONFIGURED"},
		MCU:       &SystemMCULine{Name: "STM32F40X", ClockMHz: intPtr(168), CoreTemperatureC: intPtr(42)},
		Stack:     &SystemStackLine{Bytes: 2048},
		BuildKey:  &BuildKeyLine{Key: "fake-build-key"},
		GyroLine:  "GYRO: fake",
		ACCLine:   "ACC: fake",
		GPSLine:   "GPS: connected",
		OSDLine:   "OSD: MSP",
		FlashLine: "FLASH: present",
		Unparsed:  []string{"future line"},
	}
	resources := &ResourceDiagnostics{
		Resources: []ResourceAssignment{{Kind: "MOTOR", Target: "A00"}},
		Timers:    []TimerAssignment{{Timer: "TIM1"}},
		DMA:       []DMAAssignment{{Controller: "DMA1"}},
		Unparsed:  []string{"future resource line"},
	}
	summary := buildTargetSummary(firmware, system, resources)
	if !summary.Supported || summary.TargetName != "STM32F405" || summary.BoardName != "FAKEF405" || summary.MCUName != "STM32F40X" {
		t.Fatalf("identity summary = %+v", summary)
	}
	if summary.BuildKey != "fake-build-key" || summary.ConfigurationState != "CONFIGURED" || summary.GPSLine != "GPS: connected" {
		t.Fatalf("system summary = %+v", summary)
	}
	if summary.MCUName != "STM32F40X" || summary.MCUClockMHz == nil || *summary.MCUClockMHz != 168 || summary.MCUCoreTemperatureC == nil || *summary.MCUCoreTemperatureC != 42 {
		t.Fatalf("mcu summary = %+v", summary)
	}
	if summary.StackBytes == nil || *summary.StackBytes != 2048 || summary.ACCLine != "ACC: fake" {
		t.Fatalf("stack/acc summary = %+v", summary)
	}
	if summary.ResourceCount != 1 || summary.TimerCount != 1 || summary.DMACount != 1 {
		t.Fatalf("resource summary = %+v", summary)
	}
	if summary.UnparsedSystemLines != 1 || summary.UnparsedResourceLines != 1 {
		t.Fatalf("unparsed counts = %+v", summary)
	}
}

func intPtr(value int) *int {
	return &value
}
