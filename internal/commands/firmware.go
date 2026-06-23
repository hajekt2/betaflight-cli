package commands

import (
	"context"
	"strconv"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/support"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

const supportedFirmwarePolicy = support.SupportedFirmwarePolicy

type FirmwareStatus struct {
	Source       string               `json:"source"`
	Variant      string               `json:"variant"`
	Version      string               `json:"version"`
	MSPAPI       string               `json:"msp_api"`
	MSPProtocol  uint8                `json:"msp_protocol"`
	Support      FirmwareSupport      `json:"support"`
	Target       FirmwareTarget       `json:"target"`
	Build        *BuildInfo           `json:"build,omitempty"`
	MCU          *MCUInfo             `json:"mcu,omitempty"`
	UID          *DeviceUID           `json:"uid,omitempty"`
	Metadata     FirmwareMetadata     `json:"metadata"`
	Identity     FirmwareIdentity     `json:"identity"`
	Capabilities FirmwareCapabilities `json:"capabilities"`
}

type FirmwareSupport struct {
	Supported            bool     `json:"supported"`
	Policy               string   `json:"policy"`
	Reason               string   `json:"reason"`
	AllowUnsupportedFlag string   `json:"allow_unsupported_flag"`
	Warnings             []string `json:"warnings,omitempty"`
}

type FirmwareTarget struct {
	Identifier         string `json:"identifier,omitempty"`
	TargetName         string `json:"target_name,omitempty"`
	BoardName          string `json:"board_name,omitempty"`
	ManufacturerID     string `json:"manufacturer_id,omitempty"`
	HardwareRevision   uint16 `json:"hardware_revision,omitempty"`
	BoardType          uint8  `json:"board_type,omitempty"`
	TargetCapabilities uint8  `json:"target_capabilities,omitempty"`
	SignatureHex       string `json:"signature_hex,omitempty"`
}

type FirmwareMetadata struct {
	SettingsSourceFirmware string   `json:"settings_source_firmware"`
	SettingsGenerated      bool     `json:"settings_generated"`
	SettingsCount          int      `json:"settings_count"`
	SettingsSourceFiles    []string `json:"settings_source_files"`
}

type FirmwareIdentity struct {
	LegacyName              string `json:"legacy_name,omitempty"`
	ConfiguratorUID         string `json:"configurator_uid,omitempty"`
	ConfigurationStateName  string `json:"configuration_state_name,omitempty"`
	ConfigurationProblemCSV string `json:"configuration_problem_csv,omitempty"`
}

type FirmwareCapabilities struct {
	MCUTypeID             *uint8                 `json:"mcu_type_id,omitempty"`
	ConfigurationState    *uint8                 `json:"configuration_state,omitempty"`
	SampleRateHz          *uint16                `json:"sample_rate_hz,omitempty"`
	ConfigurationProblems *ConfigurationProblems `json:"configuration_problems,omitempty"`
	SPIDeviceCount        *uint8                 `json:"spi_device_count,omitempty"`
	I2CDeviceCount        *uint8                 `json:"i2c_device_count,omitempty"`
}

func ReadFirmwareStatus(ctx context.Context, client *connection.Client) (*FirmwareStatus, []string) {
	info, warnings := ReadInfo(ctx, client)
	if info.Variant == "" {
		if variant, err := client.FCVariant(ctx); err == nil {
			info.Variant = variant
		} else {
			warnings = append(warnings, "MSP_FC_VARIANT unavailable: "+err.Error())
		}
	}
	if info.FirmwareVersion == "" {
		if version, err := client.FCVersion(ctx); err == nil {
			info.FirmwareVersion = version
		} else {
			warnings = append(warnings, "MSP_FC_VERSION unavailable: "+err.Error())
		}
	}
	if info.MSPAPIVersion == "" {
		if api, err := client.APIVersion(ctx); err == nil {
			info.MSPProtocol = api.MSPProtocol
			info.MSPAPIVersion = strconv.Itoa(int(api.Major)) + "." + strconv.Itoa(int(api.Minor))
		} else {
			warnings = append(warnings, "MSP_API_VERSION unavailable: "+err.Error())
		}
	}
	registry := settings.DefaultRegistry
	status := &FirmwareStatus{
		Source:      "MSP identity and compiled metadata",
		Variant:     info.Variant,
		Version:     info.FirmwareVersion,
		MSPAPI:      info.MSPAPIVersion,
		MSPProtocol: info.MSPProtocol,
		Support:     EvaluateFirmwareSupport(info.Variant, info.FirmwareVersion, info.MSPAPIVersion),
		Metadata: FirmwareMetadata{
			SettingsSourceFirmware: registry.SourceFirmware,
			SettingsGenerated:      registry.Generated,
			SettingsCount:          len(registry.Settings),
			SettingsSourceFiles:    append([]string(nil), registry.SourceFiles...),
		},
		Identity: FirmwareIdentity{
			LegacyName: info.LegacyName,
		},
		Build: info.Build,
		MCU:   info.MCU,
		UID:   info.UID,
	}
	if info.UID != nil {
		status.Identity.ConfiguratorUID = info.UID.ConfiguratorIdentifier
	}
	if info.Board != nil {
		status.Target = FirmwareTarget{
			Identifier:         info.Board.Identifier,
			TargetName:         info.Board.TargetName,
			BoardName:          info.Board.BoardName,
			ManufacturerID:     info.Board.ManufacturerID,
			HardwareRevision:   info.Board.HardwareRevision,
			BoardType:          info.Board.BoardType,
			TargetCapabilities: info.Board.TargetCapabilities,
			SignatureHex:       info.Board.SignatureHex,
		}
		status.Capabilities = FirmwareCapabilities{
			MCUTypeID:             info.Board.MCUTypeID,
			ConfigurationState:    info.Board.ConfigurationState,
			SampleRateHz:          info.Board.SampleRateHz,
			ConfigurationProblems: info.Board.ConfigurationProblems,
			SPIDeviceCount:        info.Board.SPIDeviceCount,
			I2CDeviceCount:        info.Board.I2CDeviceCount,
		}
		status.Identity.ConfigurationStateName = info.Board.ConfigurationStateName
		if info.Board.ConfigurationProblems != nil {
			status.Identity.ConfigurationProblemCSV = strings.Join(info.Board.ConfigurationProblems.Names, ",")
		}
	}
	status.Support.Warnings = append(status.Support.Warnings, warnings...)
	return status, warnings
}

func EvaluateFirmwareSupport(variant, firmwareVersion, apiVersion string) FirmwareSupport {
	support := FirmwareSupport{
		Policy:               supportedFirmwarePolicy,
		AllowUnsupportedFlag: "--allow-unsupported",
	}
	switch {
	case variant != "" && variant != "BTFL":
		support.Reason = "non-Betaflight firmware variant"
	case apiVersion == "":
		support.Reason = "missing MSP API version"
	case !strings.HasPrefix(apiVersion, "1."):
		support.Reason = "unsupported MSP API major version"
	case support.IsSupportedFirmwareVersion(firmwareVersion):
		support.Supported = true
		support.Reason = "firmware is inside the supported metadata range"
	case firmwareVersion == "":
		support.Reason = "missing firmware version"
	default:
		support.Reason = "firmware is outside the supported metadata range"
	}
	return support
}
