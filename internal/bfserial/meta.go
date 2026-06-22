package bfserial

import "strconv"

var serialFunctions = []struct {
	bit  uint32
	name string
}{
	{1 << 0, "MSP"},
	{1 << 1, "GPS"},
	{1 << 2, "TELEMETRY_FRSKY_HUB"},
	{1 << 3, "TELEMETRY_HOTT"},
	{1 << 4, "TELEMETRY_LTM"},
	{1 << 5, "TELEMETRY_SMARTPORT"},
	{1 << 6, "RX_SERIAL"},
	{1 << 7, "BLACKBOX"},
	{1 << 9, "TELEMETRY_MAVLINK"},
	{1 << 10, "ESC_SENSOR"},
	{1 << 11, "VTX_SMARTAUDIO"},
	{1 << 12, "TELEMETRY_IBUS"},
	{1 << 13, "VTX_TRAMP"},
	{1 << 14, "RCDEVICE"},
	{1 << 15, "LIDAR_TF"},
	{1 << 16, "FRSKY_OSD"},
	{1 << 17, "VTX_MSP"},
	{1 << 18, "GIMBAL"},
	{1 << 19, "LIDAR_NL"},
	{1 << 20, "OSD_CUSTOM_TEXT"},
}

var baudRateNames = []string{
	"AUTO",
	"9600",
	"19200",
	"38400",
	"57600",
	"115200",
	"230400",
	"250000",
	"400000",
	"460800",
	"500000",
	"921600",
	"1000000",
	"1500000",
	"2000000",
	"2470000",
}

func FunctionNames(mask uint32) []string {
	out := []string{}
	for _, def := range serialFunctions {
		if mask&def.bit != 0 {
			out = append(out, def.name)
		}
	}
	if len(out) == 0 && mask == 0 {
		return []string{"NONE"}
	}
	return out
}

func BaudRateName(index uint8) string {
	if int(index) >= len(baudRateNames) {
		return ""
	}
	return baudRateNames[index]
}

func PortIdentifierName(identifier uint8) string {
	switch {
	case identifier == 20:
		return "USB_VCP"
	case identifier >= 30 && identifier <= 39:
		return indexedName("SOFTSERIAL", int(identifier-29))
	case identifier >= 40 && identifier <= 49:
		return indexedName("LPUART", int(identifier-39))
	case identifier >= 50 && identifier <= 69:
		return indexedName("UART", int(identifier-50))
	case identifier >= 70 && identifier <= 99:
		return indexedName("PIOUART", int(identifier-69))
	default:
		return ""
	}
}

func indexedName(prefix string, index int) string {
	return prefix + strconv.Itoa(index)
}
