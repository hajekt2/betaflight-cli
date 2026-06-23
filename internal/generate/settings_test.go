package generate

import (
	"strings"
	"testing"
)

func TestParseSettings(t *testing.T) {
	input := SettingsParseInput{
		ParameterNames: `
#define PARAM_NAME_GYRO_LPF1_STATIC_HZ "gyro_lpf1_static_hz"
#define PARAM_NAME_DSHOT_BIDIR "dshot_bidir"
`,
		SettingsH: `
typedef enum {
    TABLE_OFF_ON = 0,
    TABLE_DEBUG,
    TABLE_MOTOR_PWM_PROTOCOL,
    LOOKUP_TABLE_COUNT
} lookupTableIndex_e;
`,
		SettingsC: `
const char * const lookupTableOffOn[] = {
    "OFF", "ON"
};
static const char * const lookupTablePwmProtocol[] = {
    "PWM", "DSHOT300"
};
const lookupTableEntry_t lookupTables[] = {
    LOOKUP_TABLE_ENTRY(lookupTableOffOn),
    LOOKUP_TABLE_ENTRY(debugModeNames),
    LOOKUP_TABLE_ENTRY(lookupTablePwmProtocol),
};
const clivalue_t valueTable[] = {
    { PARAM_NAME_GYRO_LPF1_STATIC_HZ, VAR_UINT16 | MASTER_VALUE, .config.minmaxUnsigned = { 0, LPF_MAX_HZ }, PG_GYRO_CONFIG, offsetof(gyroConfig_t, gyro_lpf1_static_hz) },
    { PARAM_NAME_DSHOT_BIDIR, VAR_UINT8 | MASTER_VALUE | MODE_LOOKUP, .config.lookup = { TABLE_OFF_ON }, PG_MOTOR_CONFIG, offsetof(motorConfig_t, dev.useDshotTelemetry) },
    { "motor_pwm_protocol", VAR_UINT8 | MASTER_VALUE | MODE_LOOKUP, .config.lookup = { TABLE_MOTOR_PWM_PROTOCOL }, PG_MOTOR_CONFIG, offsetof(motorConfig_t, dev.motorProtocol) },
    { "msp_override_channels_mask", VAR_UINT32 | MASTER_VALUE, .config.u32Max = (1 << MAX_SUPPORTED_RC_CHANNEL_COUNT) - 1, PG_RX_CONFIG, offsetof(rxConfig_t, msp_override_channels_mask) },
    { "craft_name", VAR_UINT8 | MASTER_VALUE | MODE_STRING, .config.string = { 1, 16, STRING_FLAGS_NONE }, PG_PILOT_CONFIG, offsetof(pilotConfig_t, name) },
    { "gyro_1_enabled", VAR_UINT8 | HARDWARE_VALUE | MODE_BITSET, .config.bitpos = 0, PG_GYRO_CONFIG, offsetof(gyroConfig_t, gyro_enabled_bitmask) },
    { "gyro_" STR(N) "_bustype", VAR_UINT8 | HARDWARE_VALUE | MODE_LOOKUP, .config.lookup = { TABLE_OFF_ON }, PG_GYRO_DEVICE_CONFIG, offsetof(gyroConfig_t, dev.busType) },
};
`,
		SourceC: "src/main/cli/settings.c",
	}

	result, err := ParseSettings(input)
	if err != nil {
		t.Fatalf("ParseSettings() error = %v", err)
	}
	if len(result.Settings) != 15 {
		t.Fatalf("len(settings) = %d, warnings = %+v", len(result.Settings), result.Warnings)
	}
	byName := map[string]SettingMetadata{}
	for _, setting := range result.Settings {
		byName[setting.Name] = setting
	}
	gyro := byName["gyro_lpf1_static_hz"]
	if gyro.Min == nil || *gyro.Min != 0 || gyro.MaxExpression != "LPF_MAX_HZ" {
		t.Fatalf("gyro metadata = %+v", gyro)
	}
	dshot := byName["dshot_bidir"]
	if dshot.Type != "lookup" || strings.Join(dshot.Lookup, ",") != "OFF,ON" {
		t.Fatalf("dshot metadata = %+v", dshot)
	}
	motor := byName["motor_pwm_protocol"]
	if strings.Join(motor.Lookup, ",") != "PWM,DSHOT300" {
		t.Fatalf("motor metadata = %+v", motor)
	}
	craft := byName["craft_name"]
	if craft.Type != "string" || craft.MinLength == nil || *craft.MinLength != 1 || craft.MaxLength == nil || *craft.MaxLength != 16 {
		t.Fatalf("craft metadata = %+v", craft)
	}
	mask := byName["msp_override_channels_mask"]
	if mask.MaxExpression != "(1 << MAX_SUPPORTED_RC_CHANNEL_COUNT) - 1" {
		t.Fatalf("mask metadata = %+v", mask)
	}
	bitset := byName["gyro_1_enabled"]
	if bitset.Scope != "hardware" || bitset.BitPosition == nil || *bitset.BitPosition != 0 {
		t.Fatalf("bitset metadata = %+v", bitset)
	}
	if _, ok := byName["gyro_1_bustype"]; !ok {
		t.Fatalf("missing gyro_1_bustype in %+v", byName)
	}
	if _, ok := byName["gyro_2_bustype"]; !ok {
		t.Fatalf("missing gyro_2_bustype in %+v", byName)
	}
	if byName["pos_hold_without_mag"].Type != "lookup" || byName["abs_control_gain"].Scope != "profile" {
		t.Fatalf("supplemental settings missing: %+v / %+v", byName["pos_hold_without_mag"], byName["abs_control_gain"])
	}
}

func TestWriteSettingsRegistry(t *testing.T) {
	min := int64(0)
	max := int64(180)
	out, err := WriteSettingsRegistry([]SettingMetadata{{
		Name:   "small_angle",
		Type:   "uint",
		Scope:  "master",
		Mode:   "direct",
		Min:    &min,
		Max:    &max,
		PG:     "PG_IMU_CONFIG",
		Source: "src/main/cli/settings.c",
		Line:   1110,
	}}, "2025.12.0", []string{"src/main/cli/settings.c"})
	if err != nil {
		t.Fatalf("WriteSettingsRegistry() error = %v", err)
	}
	got := string(out)
	if !strings.Contains(got, "DefaultRegistry = generatedRegistry()") {
		t.Fatalf("generated settings missing init:\n%s", got)
	}
	if !strings.Contains(got, `Name: "small_angle"`) || !strings.Contains(got, "Min: int64Ptr(0)") {
		t.Fatalf("generated settings missing metadata:\n%s", got)
	}
}
