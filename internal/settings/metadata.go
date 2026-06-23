package settings

import (
	"fmt"
	"strconv"
	"strings"
)

type ValueType string

const (
	TypeInt    ValueType = "int"
	TypeUint   ValueType = "uint"
	TypeLookup ValueType = "lookup"
	TypeString ValueType = "string"
)

type Metadata struct {
	Name          string    `json:"name"`
	Type          ValueType `json:"type"`
	Scope         string    `json:"scope,omitempty"`
	Mode          string    `json:"mode,omitempty"`
	Min           *int64    `json:"min,omitempty"`
	Max           *int64    `json:"max,omitempty"`
	MinExpression string    `json:"min_expression,omitempty"`
	MaxExpression string    `json:"max_expression,omitempty"`
	LookupTable   string    `json:"lookup_table,omitempty"`
	Lookup        []string  `json:"lookup,omitempty"`
	BitPosition   *int64    `json:"bit_position,omitempty"`
	MinLength     *int64    `json:"min_length,omitempty"`
	MaxLength     *int64    `json:"max_length,omitempty"`
	PG            string    `json:"pg,omitempty"`
	Line          int       `json:"line,omitempty"`
	Description   string    `json:"description,omitempty"`
	Source        string    `json:"source"`
}

type Registry struct {
	SourceFirmware string     `json:"source_firmware"`
	SourceFiles    []string   `json:"source_files"`
	Generated      bool       `json:"generated"`
	Settings       []Metadata `json:"settings"`
	byName         map[string]Metadata
}

var DefaultRegistry = newRegistry([]Metadata{
	uintSetting("gyro_lpf1_static_hz", "master", 0, 16000, "Gyro low-pass 1 static cutoff."),
	uintSetting("gyro_lpf2_static_hz", "master", 0, 16000, "Gyro low-pass 2 static cutoff."),
	uintSetting("gyro_lpf1_dyn_min_hz", "master", 0, 16000, "Gyro dynamic low-pass 1 minimum cutoff."),
	lookupSetting("dshot_bidir", "master", []string{"OFF", "ON"}, "Bidirectional DShot telemetry."),
	lookupSetting("motor_pwm_protocol", "master", []string{"PWM", "ONESHOT125", "ONESHOT42", "MULTISHOT", "BRUSHED", "DSHOT150", "DSHOT300", "DSHOT600", "PROSHOT1000"}, "Motor output protocol."),
	lookupSetting("failsafe_procedure", "master", []string{"DROP", "LAND", "GPS-RESCUE"}, "Failsafe action."),
	uintSetting("vbat_max_cell_voltage", "master", 100, 500, "Maximum cell voltage in 0.01V units."),
	uintSetting("vbat_min_cell_voltage", "master", 100, 500, "Minimum cell voltage in 0.01V units."),
	uintSetting("small_angle", "master", 0, 180, "Maximum arming angle."),
	lookupSetting("gps_ublox_use_galileo", "master", []string{"OFF", "ON"}, "Enable Galileo for UBLOX GPS."),
	uintSetting("gps_rescue_min_sats", "master", 0, 50, "Minimum satellites for GPS Rescue."),
	uintSetting("deadband", "master", 0, 32, "RC deadband."),
	uintSetting("yaw_deadband", "master", 0, 32, "Yaw deadband."),
	uintSetting("pid_process_denom", "master", 1, 16, "PID process denominator."),
	uintSetting("p_roll", "profile", 0, 200, "Roll P gain."),
	uintSetting("i_roll", "profile", 0, 200, "Roll I gain."),
	uintSetting("d_roll", "profile", 0, 200, "Roll D gain."),
	uintSetting("f_roll", "profile", 0, 250, "Roll feed-forward gain."),
	uintSetting("p_pitch", "profile", 0, 200, "Pitch P gain."),
	uintSetting("i_pitch", "profile", 0, 200, "Pitch I gain."),
	uintSetting("d_pitch", "profile", 0, 200, "Pitch D gain."),
	uintSetting("f_pitch", "profile", 0, 250, "Pitch feed-forward gain."),
	uintSetting("p_yaw", "profile", 0, 200, "Yaw P gain."),
	uintSetting("i_yaw", "profile", 0, 200, "Yaw I gain."),
	uintSetting("f_yaw", "profile", 0, 250, "Yaw feed-forward gain."),
	uintSetting("roll_rc_rate", "rateprofile", 0, 255, "Roll RC rate."),
	uintSetting("pitch_rc_rate", "rateprofile", 0, 255, "Pitch RC rate."),
	uintSetting("yaw_rc_rate", "rateprofile", 0, 255, "Yaw RC rate."),
	uintSetting("roll_expo", "rateprofile", 0, 100, "Roll expo."),
	uintSetting("pitch_expo", "rateprofile", 0, 100, "Pitch expo."),
	uintSetting("yaw_expo", "rateprofile", 0, 100, "Yaw expo."),
	stringSetting("craft_name", "master", "Craft name."),
})

func newRegistry(settings []Metadata) Registry {
	return newRegistryWithSource("2025.12.x", []string{
		"src/main/cli/settings.c",
		"src/main/cli/settings.h",
	}, false, settings)
}

func newRegistryWithSource(sourceFirmware string, sourceFiles []string, generated bool, settings []Metadata) Registry {
	reg := Registry{
		SourceFirmware: sourceFirmware,
		SourceFiles:    sourceFiles,
		Generated:      generated,
		Settings:       settings,
		byName:         map[string]Metadata{},
	}
	for _, setting := range settings {
		reg.byName[strings.ToLower(setting.Name)] = setting
	}
	return reg
}

func int64Ptr(value int64) *int64 {
	return &value
}

func (r Registry) Lookup(name string) (Metadata, bool) {
	setting, ok := r.byName[strings.ToLower(name)]
	return setting, ok
}

func (r Registry) Filter(match func(Metadata) bool) []Metadata {
	var out []Metadata
	for _, setting := range r.Settings {
		if match(setting) {
			out = append(out, setting)
		}
	}
	return out
}

func (m Metadata) Validate(value string) error {
	if m.Mode == "array" {
		return nil
	}
	if m.Mode == "bitset" && (strings.EqualFold(value, "ON") || strings.EqualFold(value, "OFF")) {
		return nil
	}
	switch m.Type {
	case TypeString:
		if value == "" {
			return fmt.Errorf("%s cannot be empty", m.Name)
		}
		length := int64(len(value))
		if m.MinLength != nil && length < *m.MinLength {
			return fmt.Errorf("%s length must be >= %d", m.Name, *m.MinLength)
		}
		if m.MaxLength != nil && length > *m.MaxLength {
			return fmt.Errorf("%s length must be <= %d", m.Name, *m.MaxLength)
		}
		return nil
	case TypeLookup:
		if len(m.Lookup) == 0 {
			return nil
		}
		for _, allowed := range m.Lookup {
			if strings.EqualFold(value, allowed) {
				return nil
			}
		}
		return fmt.Errorf("%s must be one of %s", m.Name, strings.Join(m.Lookup, ", "))
	case TypeInt, TypeUint:
		n, err := strconv.ParseInt(value, 10, 64)
		if err != nil {
			return fmt.Errorf("%s must be an integer: %w", m.Name, err)
		}
		if m.Type == TypeUint && n < 0 {
			return fmt.Errorf("%s must be unsigned", m.Name)
		}
		if m.Min != nil && n < *m.Min {
			return fmt.Errorf("%s must be >= %d", m.Name, *m.Min)
		}
		if m.Max != nil && n > *m.Max {
			return fmt.Errorf("%s must be <= %d", m.Name, *m.Max)
		}
		return nil
	default:
		return nil
	}
}

func uintSetting(name, scope string, min, max int64, description string) Metadata {
	return numericSetting(name, scope, TypeUint, min, max, description)
}

func numericSetting(name, scope string, typ ValueType, min, max int64, description string) Metadata {
	return Metadata{
		Name:        name,
		Type:        typ,
		Scope:       scope,
		Min:         &min,
		Max:         &max,
		Description: description,
		Source:      "compiled seed metadata; replace with generated settings.c metadata",
	}
}

func lookupSetting(name, scope string, values []string, description string) Metadata {
	return Metadata{
		Name:        name,
		Type:        TypeLookup,
		Scope:       scope,
		Lookup:      values,
		Description: description,
		Source:      "compiled seed metadata; replace with generated settings.c metadata",
	}
}

func stringSetting(name, scope, description string) Metadata {
	return Metadata{
		Name:        name,
		Type:        TypeString,
		Scope:       scope,
		Description: description,
		Source:      "compiled seed metadata; replace with generated settings.c metadata",
	}
}
