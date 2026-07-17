package msp

// ForwardMSPSourceVersion identifies the exact upstream prerelease that adds
// optional MSP commands newer than the stable compiled metadata baseline.
const ForwardMSPSourceVersion = "2026.6.0-rc2"

const (
	MSPAttitudeQuaternion uint16 = 0x00a7
	MSP2BatteryProfile    uint16 = 0x300e
	MSP2SetBatteryProfile uint16 = 0x300f
	MSP2CLISetting        uint16 = 0x3010
	MSP2CLISettingInfo    uint16 = 0x3011
)

func init() {
	RegisterCommands([]CommandMeta{
		{Name: "MSP_ATTITUDE_QUATERNION", Code: MSPAttitudeQuaternion, Protocol: 1, Direction: DirectionRead, SourceVersion: ForwardMSPSourceVersion, Source: "src/main/msp/msp_protocol.h", Line: 221},
		{Name: "MSP2_BATTERY_PROFILE", Code: MSP2BatteryProfile, Protocol: 2, Direction: DirectionRead, SourceVersion: ForwardMSPSourceVersion, Source: "src/main/msp/msp_protocol_v2_betaflight.h", Line: 35},
		{Name: "MSP2_SET_BATTERY_PROFILE", Code: MSP2SetBatteryProfile, Protocol: 2, Direction: DirectionWrite, SourceVersion: ForwardMSPSourceVersion, Source: "src/main/msp/msp_protocol_v2_betaflight.h", Line: 36},
		{Name: "MSP2_CLI_SETTING", Code: MSP2CLISetting, Protocol: 2, Direction: DirectionRead, SourceVersion: ForwardMSPSourceVersion, Source: "src/main/msp/msp_protocol_v2_betaflight.h", Line: 37},
		{Name: "MSP2_CLI_SETTING_INFO", Code: MSP2CLISettingInfo, Protocol: 2, Direction: DirectionRead, SourceVersion: ForwardMSPSourceVersion, Source: "src/main/msp/msp_protocol_v2_betaflight.h", Line: 38},
	})
}
