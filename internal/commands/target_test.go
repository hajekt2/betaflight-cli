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
		BuildKey:  &BuildKeyLine{Key: "fake-build-key"},
		GyroLine:  "GYRO: fake",
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
	if !summary.Supported || summary.TargetName != "STM32F405" || summary.BoardName != "FAKEF405" || summary.MCUName != "STM32F405" {
		t.Fatalf("identity summary = %+v", summary)
	}
	if summary.BuildKey != "fake-build-key" || summary.ConfigurationState != "CONFIGURED" || summary.GPSLine != "GPS: connected" {
		t.Fatalf("system summary = %+v", summary)
	}
	if summary.ResourceCount != 1 || summary.TimerCount != 1 || summary.DMACount != 1 {
		t.Fatalf("resource summary = %+v", summary)
	}
	if summary.UnparsedSystemLines != 1 || summary.UnparsedResourceLines != 1 {
		t.Fatalf("unparsed counts = %+v", summary)
	}
}
