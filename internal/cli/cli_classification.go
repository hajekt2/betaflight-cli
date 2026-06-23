package cli

import "strings"

type cliClass int

const (
	cliReadOnly cliClass = iota
	cliWrite
	cliDangerous
)

type cliCommandPolicy struct {
	class        cliClass
	batchAllowed bool
}

var cliCommandPolicies = map[string]cliCommandPolicy{
	"diff":            {class: cliReadOnly},
	"dump":            {class: cliReadOnly},
	"get":             {class: cliReadOnly},
	"version":         {class: cliReadOnly},
	"status":          {class: cliReadOnly},
	"help":            {class: cliReadOnly},
	"tasks":           {class: cliReadOnly},
	"set":             {class: cliWrite, batchAllowed: true},
	"feature":         {class: cliWrite, batchAllowed: true},
	"serial":          {class: cliWrite, batchAllowed: true},
	"aux":             {class: cliWrite, batchAllowed: true},
	"profile":         {class: cliWrite, batchAllowed: true},
	"rateprofile":     {class: cliWrite, batchAllowed: true},
	"battery_profile": {class: cliWrite, batchAllowed: true},
	"vtxtable":        {class: cliWrite, batchAllowed: true},
	"mode_color":      {class: cliWrite, batchAllowed: true},
	"color":           {class: cliWrite, batchAllowed: true},
	"led":             {class: cliWrite, batchAllowed: true},
	"servo":           {class: cliWrite, batchAllowed: true},
	"smix":            {class: cliWrite, batchAllowed: true},
	"adjrange":        {class: cliWrite, batchAllowed: true},
	"rxrange":         {class: cliWrite, batchAllowed: true},
	"rxfail":          {class: cliWrite, batchAllowed: true},
	"save":            {class: cliDangerous},
	"defaults":        {class: cliDangerous},
	"motor":           {class: cliDangerous},
	"motors":          {class: cliDangerous},
	"dshotprog":       {class: cliDangerous},
	"bl":              {class: cliDangerous},
	"dfu":             {class: cliDangerous},
	"msc":             {class: cliDangerous},
	"exit":            {class: cliDangerous},
	"reboot":          {class: cliDangerous},
	"erase":           {class: cliDangerous},
	"beeper":          {class: cliDangerous, batchAllowed: true},
	"beacon":          {class: cliWrite, batchAllowed: true},
	"mixer":           {class: cliWrite, batchAllowed: true},
	"mmix":            {class: cliWrite, batchAllowed: true},
	"map":             {class: cliWrite, batchAllowed: true},
	"timer":           {class: cliWrite, batchAllowed: true},
	"dma":             {class: cliWrite, batchAllowed: true},
}

var cliBatchResourceAllowed = true

func classifyCLI(command string) cliClass {
	fields := tokenizeCLI(command)
	if len(fields) == 0 {
		return cliReadOnly
	}
	first := fields[0]
	if first == "resource" {
		return classifyResourceCommand(fields)
	}
	if policy, ok := cliCommandPolicies[first]; ok {
		return policy.class
	}
	return cliWrite
}

func isKnownCLICommand(command string) bool {
	fields := tokenizeCLI(command)
	if len(fields) == 0 {
		return false
	}
	first := fields[0]
	if first == "resource" {
		return true
	}
	_, ok := cliCommandPolicies[first]
	return ok
}

func isBatchAllowed(line string) bool {
	fields := tokenizeCLI(line)
	if len(fields) == 0 {
		return false
	}
	first := fields[0]
	if first == "resource" {
		return cliBatchResourceAllowed
	}
	policy, ok := cliCommandPolicies[first]
	if !ok {
		return false
	}
	return policy.batchAllowed
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

func classifyResourceCommand(fields []string) cliClass {
	if len(fields) == 0 || fields[0] != "resource" {
		return cliWrite
	}
	if len(fields) == 1 {
		return cliReadOnly
	}
	subcommand := fields[1]
	if subcommand == "show" || subcommand == "list" {
		return cliReadOnly
	}
	return cliWrite
}
