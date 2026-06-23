package blackbox

import "strings"

var blackboxDisarmReasons = []string{
	"ARMING_DISABLED",
	"FAILSAFE",
	"THROTTLE_TIMEOUT",
	"STICKS",
	"SWITCH",
	"CRASH_PROTECTION",
	"RUNAWAY_TAKEOFF",
	"GPS_RESCUE",
	"SERIAL_IO",
}

var blackboxFlightModeNamesPost45 = []string{
	"ARM",
	"ANGLE",
	"HORIZON",
	"MAG",
	"ALTHOLD",
	"HEADFREE",
	"CHIRP",
	"PASSTHRU",
	"FAILSAFE",
	"POSHOLD",
	"GPSRESCUE",
	"ANTIGRAVITY",
	"HEADADJ",
	"CAMSTAB",
	"BEEPER",
	"LEDLOW",
	"CALIB",
	"OSD",
	"TELEMETRY",
	"SERVO1",
	"SERVO2",
	"SERVO3",
	"BLACKBOX",
	"AIRMODE",
	"3D",
	"FPVANGLEMIX",
	"BLACKBOXERASE",
	"CAMERA1",
	"CAMERA2",
	"CAMERA3",
	"FLIPOVERAFTERCRASH",
	"PREARM",
	"BEEPGPSCOUNT",
	"VTXPITMODE",
	"USER1",
	"USER2",
	"USER3",
	"USER4",
	"PIDAUDIO",
	"ACROTRAINER",
	"VTXCONTROLDISABLE",
	"LAUNCHCONTROL",
}

func disarmReasonName(reason int) string {
	if reason < 0 || reason >= len(blackboxDisarmReasons) {
		return "UNKNOWN"
	}
	return blackboxDisarmReasons[reason]
}

func decodeFlightModeNames(mask int) []string {
	if mask == 0 {
		return nil
	}
	out := []string{}
	for i, name := range blackboxFlightModeNamesPost45 {
		if mask&(1<<i) != 0 {
			out = append(out, name)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func decodeFlightModeDisabledNames(lastFlags, newFlags int) []string {
	disabled := lastFlags &^ newFlags
	return decodeFlightModeNames(disabled)
}

func trimNULString(value string) string {
	return strings.TrimRight(value, "\x00")
}
