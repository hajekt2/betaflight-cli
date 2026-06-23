package commands

import (
	"context"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type TargetStatus struct {
	Source    string               `json:"source"`
	Summary   TargetSummary        `json:"summary"`
	Firmware  *FirmwareStatus      `json:"firmware,omitempty"`
	System    *SystemStatus        `json:"system,omitempty"`
	Resources *ResourceDiagnostics `json:"resources,omitempty"`
	Warnings  []string             `json:"warnings,omitempty"`
}

type TargetSummary struct {
	Variant               string `json:"variant,omitempty"`
	FirmwareVersion       string `json:"firmware_version,omitempty"`
	MSPAPI                string `json:"msp_api,omitempty"`
	Supported             bool   `json:"supported"`
	TargetName            string `json:"target_name,omitempty"`
	BoardName             string `json:"board_name,omitempty"`
	ManufacturerID        string `json:"manufacturer_id,omitempty"`
	MCUName               string `json:"mcu_name,omitempty"`
	ConfigurationState    string `json:"configuration_state,omitempty"`
	BuildKey              string `json:"build_key,omitempty"`
	GyroLine              string `json:"gyro_line,omitempty"`
	GPSLine               string `json:"gps_line,omitempty"`
	OSDLine               string `json:"osd_line,omitempty"`
	FlashLine             string `json:"flash_line,omitempty"`
	ResourceCount         int    `json:"resource_count"`
	TimerCount            int    `json:"timer_count"`
	DMACount              int    `json:"dma_count"`
	UnparsedSystemLines   int    `json:"unparsed_system_lines"`
	UnparsedResourceLines int    `json:"unparsed_resource_lines"`
}

func ReadTargetStatus(ctx context.Context, client *connection.Client) (*TargetStatus, []string) {
	status := &TargetStatus{
		Source:   "MSP identity plus CLI system and resource diagnostics",
		Warnings: []string{},
	}
	firmware, firmwareWarnings := ReadFirmwareStatus(ctx, client)
	status.Firmware = firmware
	status.Warnings = append(status.Warnings, firmwareWarnings...)
	system, err := ReadSystemStatus(ctx, client)
	if err != nil {
		status.Warnings = append(status.Warnings, "system status unavailable: "+err.Error())
	} else {
		status.System = system
	}
	resources, err := ReadResourceDiagnostics(ctx, client)
	if err != nil {
		status.Warnings = append(status.Warnings, "resource diagnostics unavailable: "+err.Error())
	} else {
		status.Resources = resources
	}
	status.Summary = buildTargetSummary(status.Firmware, status.System, status.Resources)
	return status, status.Warnings
}

func buildTargetSummary(firmware *FirmwareStatus, system *SystemStatus, resources *ResourceDiagnostics) TargetSummary {
	var summary TargetSummary
	if firmware != nil {
		summary.Variant = firmware.Variant
		summary.FirmwareVersion = firmware.Version
		summary.MSPAPI = firmware.MSPAPI
		summary.Supported = firmware.Support.Supported
		summary.TargetName = firmware.Target.TargetName
		summary.BoardName = firmware.Target.BoardName
		summary.ManufacturerID = firmware.Target.ManufacturerID
		if firmware.MCU != nil {
			summary.MCUName = firmware.MCU.Name
		}
		summary.ConfigurationState = firmware.Identity.ConfigurationStateName
	}
	if system != nil {
		if system.Config != nil {
			summary.ConfigurationState = system.Config.State
		}
		if system.BuildKey != nil {
			summary.BuildKey = system.BuildKey.Key
		}
		summary.GyroLine = system.GyroLine
		summary.GPSLine = system.GPSLine
		summary.OSDLine = system.OSDLine
		summary.FlashLine = system.FlashLine
		summary.UnparsedSystemLines = len(system.Unparsed)
	}
	if resources != nil {
		summary.ResourceCount = len(resources.Resources)
		summary.TimerCount = len(resources.Timers)
		summary.DMACount = len(resources.DMA)
		summary.UnparsedResourceLines = len(resources.Unparsed)
	}
	return summary
}
