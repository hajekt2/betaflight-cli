package msp

const (
	MSPAPIVersion      uint16 = 1
	MSPFCVariant       uint16 = 2
	MSPFCVersion       uint16 = 3
	MSPBoardInfo       uint16 = 4
	MSPBuildInfo       uint16 = 5
	MSPStatus          uint16 = 101
	MSPRC              uint16 = 105
	MSPAttitude        uint16 = 108
	MSPBatteryState    uint16 = 130
	MSPStatusEx        uint16 = 150
	MSP2CLISetting     uint16 = 0x3010
	MSP2CLISettingInfo uint16 = 0x3011
)

func IsLikelyWriteCode(code uint16) bool {
	if code >= 200 && code <= 252 {
		return true
	}
	switch code {
	case 11, 33, 35, 37, 39, 41, 43, 45, 47, 49, 51, 53, 55, 57, 60, 62, 65, 68, 72, 76, 78, 81, 85, 87, 89, 91, 93, 95, 97, 99:
		return true
	case 0x100a, 0x3002, 0x3003, 0x3007, 0x3009, 0x300f:
		return true
	default:
		return false
	}
}
