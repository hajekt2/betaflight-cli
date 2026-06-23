package cli

import (
	"strings"
)

type cliClass int

const (
	cliReadOnly cliClass = iota
	cliWrite
	cliDangerous
)

type cliClassifyRule struct {
	matchFn func(fields []string) bool
	class   cliClass
}

var cliClassificationRules = []cliClassifyRule{
	{matchFn: matchLiteral("diff", "dump", "get", "version", "status", "help", "tasks"), class: cliReadOnly},
	{matchFn: matchLiteral("set", "feature", "serial", "aux", "profile", "rateprofile", "battery_profile", "vtxtable", "mode_color", "color", "led", "servo", "smix", "adjrange", "rxrange", "rxfail"), class: cliWrite},
	{matchFn: matchResourceCommand, class: cliWrite},
	{matchFn: matchLiteral("save", "defaults", "motor", "motors", "dshotprog", "bl", "dfu", "msc", "exit", "reboot", "erase", "beeper"), class: cliDangerous},
}

var batchAllowRules = []string{"set", "feature", "serial", "aux", "resource", "timer", "dma", "profile", "rateprofile", "battery_profile", "vtxtable", "mode_color", "color", "led", "servo", "smix", "adjrange", "rxrange", "rxfail", "beeper", "beacon", "mixer", "mmix", "map"}

func classifyCLI(command string) cliClass {
	fields := tokenizeCLI(command)
	if len(fields) == 0 {
		return cliReadOnly
	}
	for _, rule := range cliClassificationRules {
		if rule.matchFn(fields) {
			return rule.class
		}
	}
	return cliWrite
}

func isBatchAllowed(line string) bool {
	fields := tokenizeCLI(line)
	if len(fields) == 0 {
		return false
	}
	for _, keyword := range batchAllowRules {
		if fields[0] == keyword {
			return true
		}
	}
	return false
}

func isConfigurationRead(command string) bool {
	fields := tokenizeCLI(command)
	if len(fields) == 0 {
		return false
	}
	first := fields[0]
	return first == "dump" || first == "diff"
}

func tokenizeCLI(command string) []string {
	return strings.Fields(strings.ToLower(strings.TrimSpace(command)))
}

func matchLiteral(commands ...string) func(fields []string) bool {
	set := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		set[command] = struct{}{}
	}
	return func(fields []string) bool {
		if len(fields) == 0 {
			return false
		}
		_, ok := set[fields[0]]
		return ok
	}
}

func matchResourceCommand(fields []string) bool {
	if len(fields) == 0 || fields[0] != "resource" {
		return false
	}
	if len(fields) == 1 {
		return true
	}
	subcommand := fields[1]
	return subcommand != "show" && subcommand != "list"
}
