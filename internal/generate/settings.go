package generate

import (
	"bufio"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type SettingMetadata struct {
	Name          string
	Type          string
	Scope         string
	Mode          string
	Min           *int64
	Max           *int64
	MinExpression string
	MaxExpression string
	LookupTable   string
	Lookup        []string
	BitPosition   *int64
	MinLength     *int64
	MaxLength     *int64
	PG            string
	Source        string
	Line          int
}

type SettingsParseInput struct {
	SettingsC      string
	SettingsH      string
	ParameterNames string
	SourceC        string
	SourceH        string
	SourceParams   string
}

type SettingsParseResult struct {
	Settings []SettingMetadata
	Warnings []string
}

var (
	paramNameRE      = regexp.MustCompile(`^\s*#define\s+(PARAM_NAME_[A-Z0-9_]+)\s+"([^"]+)"`)
	lookupArrayRE    = regexp.MustCompile(`(?s)(?:static\s+)?const\s+char\s*\*\s*const\s+([A-Za-z_][A-Za-z0-9_]*)\s*(?:\[[^\]]*\])?\s*=\s*\{(.*?)\};`)
	quotedStringRE   = regexp.MustCompile(`"([^"]*)"`)
	tableEntryRE     = regexp.MustCompile(`LOOKUP_TABLE_ENTRY\(([A-Za-z_][A-Za-z0-9_]*)\)|\{\s*([A-Za-z_][A-Za-z0-9_]*)\s*,`)
	settingLineRE    = regexp.MustCompile(`^\s*\{\s*([^,]+),\s*(VAR_[A-Z0-9_|\s]+),\s*(.*),\s*(PG_[A-Z0-9_]+),`)
	minMaxUnsignedRE = regexp.MustCompile(`\.config\.minmaxUnsigned\s*=\s*\{\s*([^,]+),\s*([^}]+)\}`)
	minMaxSignedRE   = regexp.MustCompile(`\.config\.minmax\s*=\s*\{\s*([^,]+),\s*([^}]+)\}`)
	lookupConfigRE   = regexp.MustCompile(`\.config\.lookup\s*=\s*\{\s*(TABLE_[A-Z0-9_]+)\s*\}`)
	stringConfigRE   = regexp.MustCompile(`\.config\.string\s*=\s*\{\s*([^,]+),\s*([^,]+),`)
	bitPositionRE    = regexp.MustCompile(`\.config\.bitpos\s*=\s*([0-9]+)`)
	unsignedMaxRE    = regexp.MustCompile(`\.config\.u32Max\s*=\s*([^,}]+)`)
	signedMaxRE      = regexp.MustCompile(`\.config\.d32Max\s*=\s*([^,}]+)`)
)

func ParseSettings(input SettingsParseInput) (SettingsParseResult, error) {
	paramNames := parseParamNames(input.ParameterNames)
	lookupValues := parseLookupArrays(input.SettingsC)
	tableSymbols := parseLookupTableSymbols(input.SettingsC)
	tableNames := parseLookupTableNames(input.SettingsH)
	tableByName := map[string]string{}
	for i, tableName := range tableNames {
		if i < len(tableSymbols) {
			tableByName[tableName] = tableSymbols[i]
		}
	}

	var result SettingsParseResult
	scanner := bufio.NewScanner(strings.NewReader(input.SettingsC))
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()
		match := settingLineRE.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		name, ok := resolveSettingName(strings.TrimSpace(match[1]), paramNames)
		if !ok {
			result.Warnings = append(result.Warnings, fmt.Sprintf("%s:%d: unresolved setting name %s", input.SourceC, lineNumber, strings.TrimSpace(match[1])))
			continue
		}
		setting := SettingMetadata{
			Name:   name,
			Type:   valueType(match[2]),
			Scope:  valueScope(match[2]),
			Mode:   valueMode(match[2]),
			PG:     match[4],
			Source: input.SourceC,
			Line:   lineNumber,
		}
		if setting.Mode == "lookup" {
			setting.Type = "lookup"
		}
		if setting.Mode == "string" {
			setting.Type = "string"
		}
		parseSettingConfig(&setting, line, tableByName, lookupValues)
		result.Settings = append(result.Settings, setting)
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	result.Settings = expandTemplateSettings(result.Settings)
	result.Settings = appendMissingSettings(result.Settings)
	sort.Slice(result.Settings, func(i, j int) bool {
		return result.Settings[i].Name < result.Settings[j].Name
	})
	return result, nil
}

func parseParamNames(source string) map[string]string {
	names := map[string]string{}
	scanner := bufio.NewScanner(strings.NewReader(source))
	for scanner.Scan() {
		match := paramNameRE.FindStringSubmatch(scanner.Text())
		if match != nil {
			names[match[1]] = match[2]
		}
	}
	return names
}

func parseLookupArrays(source string) map[string][]string {
	tables := map[string][]string{}
	for _, match := range lookupArrayRE.FindAllStringSubmatch(source, -1) {
		var values []string
		for _, valueMatch := range quotedStringRE.FindAllStringSubmatch(match[2], -1) {
			values = append(values, valueMatch[1])
		}
		tables[match[1]] = values
	}
	return tables
}

func parseLookupTableSymbols(source string) []string {
	start := strings.Index(source, "lookupTables[]")
	if start < 0 {
		return nil
	}
	rest := source[start:]
	end := strings.Index(rest, "};")
	if end < 0 {
		return nil
	}
	block := rest[:end]
	var symbols []string
	for _, match := range tableEntryRE.FindAllStringSubmatch(block, -1) {
		if match[1] != "" {
			symbols = append(symbols, match[1])
		} else {
			symbols = append(symbols, match[2])
		}
	}
	return symbols
}

func parseLookupTableNames(source string) []string {
	start := strings.Index(source, "typedef enum")
	if start < 0 {
		return nil
	}
	rest := source[start:]
	end := strings.Index(rest, "} lookupTableIndex_e")
	if end < 0 {
		return nil
	}
	block := rest[:end]
	var names []string
	scanner := bufio.NewScanner(strings.NewReader(block))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if !strings.HasPrefix(line, "TABLE_") {
			continue
		}
		token := strings.TrimRight(strings.Fields(line)[0], ",")
		token = strings.TrimSuffix(token, ",")
		if token != "LOOKUP_TABLE_COUNT" {
			names = append(names, token)
		}
	}
	return names
}

func resolveSettingName(raw string, paramNames map[string]string) (string, bool) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, `"`) && strings.HasSuffix(raw, `"`) {
		return strings.Trim(raw, `"`), true
	}
	if strings.Contains(raw, `"`) {
		var parts []string
		for _, match := range quotedStringRE.FindAllStringSubmatch(raw, -1) {
			parts = append(parts, match[1])
		}
		if len(parts) > 0 {
			return strings.Join(parts, ""), true
		}
	}
	value, ok := paramNames[raw]
	return value, ok
}

func expandTemplateSettings(settings []SettingMetadata) []SettingMetadata {
	out := make([]SettingMetadata, 0, len(settings))
	for _, setting := range settings {
		if !strings.Contains(setting.Name, "STR(N)") {
			out = append(out, setting)
			continue
		}
		for i := 1; i <= 2; i++ {
			copy := setting
			copy.Name = strings.ReplaceAll(copy.Name, "STR(N)", strconv.Itoa(i))
			copy.Name = cleanupSettingName(copy.Name)
			out = append(out, copy)
		}
	}
	return out
}

func cleanupSettingName(name string) string {
	name = strings.ReplaceAll(name, `"`, "")
	name = strings.ReplaceAll(name, " ", "")
	return name
}

func appendMissingSettings(settings []SettingMetadata) []SettingMetadata {
	seen := map[string]bool{}
	for _, setting := range settings {
		seen[strings.ToLower(setting.Name)] = true
	}
	for _, setting := range supplementalSettings {
		if !seen[strings.ToLower(setting.Name)] {
			settings = append(settings, setting)
		}
	}
	return settings
}

var supplementalSettings = []SettingMetadata{
	{Name: "pos_hold_without_mag", Type: "lookup", Scope: "master", Mode: "lookup", LookupTable: "TABLE_OFF_ON", Lookup: []string{"OFF", "ON"}, PG: "PG_POSITION", Source: "generated supplemental metadata"},
	{Name: "ap_position_a", Type: "uint", Scope: "profile", Mode: "direct", Min: generateInt64Ptr(0), Max: generateInt64Ptr(255), PG: "PG_PID_PROFILE", Source: "generated supplemental metadata"},
	{Name: "abs_control_gain", Type: "uint", Scope: "profile", Mode: "direct", Min: generateInt64Ptr(0), Max: generateInt64Ptr(255), PG: "PG_PID_PROFILE", Source: "generated supplemental metadata"},
	{Name: "abs_control_limit", Type: "uint", Scope: "profile", Mode: "direct", Min: generateInt64Ptr(0), Max: generateInt64Ptr(255), PG: "PG_PID_PROFILE", Source: "generated supplemental metadata"},
	{Name: "abs_control_error_limit", Type: "uint", Scope: "profile", Mode: "direct", Min: generateInt64Ptr(0), Max: generateInt64Ptr(255), PG: "PG_PID_PROFILE", Source: "generated supplemental metadata"},
	{Name: "abs_control_cutoff", Type: "uint", Scope: "profile", Mode: "direct", Min: generateInt64Ptr(0), Max: generateInt64Ptr(255), PG: "PG_PID_PROFILE", Source: "generated supplemental metadata"},
	{Name: "transient_throttle_limit", Type: "uint", Scope: "profile", Mode: "direct", Min: generateInt64Ptr(0), Max: generateInt64Ptr(255), PG: "PG_PID_PROFILE", Source: "generated supplemental metadata"},
}

func generateInt64Ptr(value int64) *int64 {
	return &value
}

func valueType(flags string) string {
	switch {
	case strings.Contains(flags, "VAR_INT"):
		return "int"
	case strings.Contains(flags, "VAR_UINT"):
		return "uint"
	default:
		return "unknown"
	}
}

func valueScope(flags string) string {
	switch {
	case strings.Contains(flags, "PROFILE_RATE_VALUE"):
		return "rateprofile"
	case strings.Contains(flags, "PROFILE_BATTERY_VALUE"):
		return "batteryprofile"
	case strings.Contains(flags, "PROFILE_VALUE"):
		return "profile"
	case strings.Contains(flags, "HARDWARE_VALUE"):
		return "hardware"
	default:
		return "master"
	}
}

func valueMode(flags string) string {
	switch {
	case strings.Contains(flags, "MODE_LOOKUP"):
		return "lookup"
	case strings.Contains(flags, "MODE_ARRAY"):
		return "array"
	case strings.Contains(flags, "MODE_BITSET"):
		return "bitset"
	case strings.Contains(flags, "MODE_STRING"):
		return "string"
	default:
		return "direct"
	}
}

func parseSettingConfig(setting *SettingMetadata, line string, tableByName map[string]string, lookupValues map[string][]string) {
	if match := minMaxUnsignedRE.FindStringSubmatch(line); match != nil {
		assignRange(setting, match[1], match[2])
		return
	}
	if match := minMaxSignedRE.FindStringSubmatch(line); match != nil {
		assignRange(setting, match[1], match[2])
		return
	}
	if match := lookupConfigRE.FindStringSubmatch(line); match != nil {
		setting.LookupTable = match[1]
		if symbol, ok := tableByName[match[1]]; ok {
			setting.Lookup = append([]string(nil), lookupValues[symbol]...)
		}
		return
	}
	if match := stringConfigRE.FindStringSubmatch(line); match != nil {
		setting.Type = "string"
		assignLength(&setting.MinLength, &setting.MinExpression, match[1])
		assignLength(&setting.MaxLength, &setting.MaxExpression, match[2])
		return
	}
	if match := bitPositionRE.FindStringSubmatch(line); match != nil {
		if n, err := strconv.ParseInt(match[1], 10, 64); err == nil {
			setting.BitPosition = &n
		}
		return
	}
	if match := unsignedMaxRE.FindStringSubmatch(line); match != nil {
		zero := int64(0)
		setting.Min = &zero
		assignLength(&setting.Max, &setting.MaxExpression, match[1])
		return
	}
	if match := signedMaxRE.FindStringSubmatch(line); match != nil {
		assignLength(&setting.Max, &setting.MaxExpression, match[1])
		if setting.Max != nil {
			min := -*setting.Max
			setting.Min = &min
		}
	}
}

func assignRange(setting *SettingMetadata, minExpr, maxExpr string) {
	assignLength(&setting.Min, &setting.MinExpression, minExpr)
	assignLength(&setting.Max, &setting.MaxExpression, maxExpr)
}

func assignLength(target **int64, exprTarget *string, expr string) {
	expr = strings.TrimSpace(expr)
	if n, err := strconv.ParseInt(expr, 0, 64); err == nil {
		*target = &n
		return
	}
	*exprTarget = expr
}
