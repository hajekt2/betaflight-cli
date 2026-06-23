package cli

import (
	"sort"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/output"
)

type capabilityIndex struct {
	Source        string               `json:"source"`
	SchemaVersion string               `json:"schema_version"`
	DefaultFormat string               `json:"default_format"`
	SafetyModel   capabilitySafety     `json:"safety_model"`
	Commands      []capabilityCommand  `json:"commands"`
	Workflows     []capabilityWorkflow `json:"workflows"`
}

type capabilitySafety struct {
	ReadOnlyDefault          bool     `json:"read_only_default"`
	WriteRequiresYes         bool     `json:"write_requires_yes"`
	DangerousRequiresYes     bool     `json:"dangerous_requires_yes"`
	AutoPortWritesRequireOpt bool     `json:"auto_port_writes_require_opt_in"`
	SaveIsExplicit           bool     `json:"save_is_explicit"`
	Notes                    []string `json:"notes"`
}

type capabilityCommand struct {
	Command            string   `json:"command"`
	Use                string   `json:"use"`
	Short              string   `json:"short,omitempty"`
	Runnable           bool     `json:"runnable"`
	RequiresConnection bool     `json:"requires_connection"`
	Operation          string   `json:"operation"`
	Confirmation       string   `json:"confirmation"`
	OutputRoot         string   `json:"output_root,omitempty"`
	Input              string   `json:"input,omitempty"`
	Tags               []string `json:"tags,omitempty"`
}

type capabilityWorkflow struct {
	Name        string   `json:"name"`
	Purpose     string   `json:"purpose"`
	Commands    []string `json:"commands"`
	SafetyClass string   `json:"safety_class"`
	Notes       []string `json:"notes,omitempty"`
}

type capabilityMetadata struct {
	RequiresConnection bool
	Operation          string
	Confirmation       string
	OutputRoot         string
	Input              string
	Tags               []string
}

func (a *app) capabilitiesCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Print machine-readable command and workflow metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			root := cmd.Root()
			index := buildCapabilityIndex(root)
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"capabilities": index,
			}))
		},
	}
	return cmd
}

func buildCapabilityIndex(root *cobra.Command) capabilityIndex {
	return capabilityIndex{
		Source:        "cobra command tree plus curated workflow metadata",
		SchemaVersion: output.SchemaVersion,
		DefaultFormat: "json",
		SafetyModel: capabilitySafety{
			ReadOnlyDefault:          true,
			WriteRequiresYes:         true,
			DangerousRequiresYes:     true,
			AutoPortWritesRequireOpt: true,
			SaveIsExplicit:           true,
			Notes: []string{
				"read commands may auto-select a single USB flight-controller candidate",
				"write and dangerous commands require explicit confirmation and never save implicitly unless the command exposes a save option",
				"configuration restore and preset flows should be planned before apply",
				"raw CLI text remains authoritative for backups and restores",
			},
		},
		Commands:  collectCapabilityCommands(root),
		Workflows: capabilityWorkflows(),
	}
}

func collectCapabilityCommands(root *cobra.Command) []capabilityCommand {
	registry := capabilityMetadataRegistry()
	var commands []capabilityCommand
	var walk func(*cobra.Command)
	walk = func(cmd *cobra.Command) {
		if cmd == root {
			for _, child := range sortedCommands(cmd.Commands()) {
				walk(child)
			}
			return
		}
		if cmd.Hidden {
			return
		}
		path := commandPath(cmd)
		meta := inferCapabilityMetadata(path, cmd.Runnable(), registry[path])
		commands = append(commands, capabilityCommand{
			Command:            path,
			Use:                cmd.UseLine(),
			Short:              cmd.Short,
			Runnable:           cmd.Runnable(),
			RequiresConnection: meta.RequiresConnection,
			Operation:          meta.Operation,
			Confirmation:       meta.Confirmation,
			OutputRoot:         meta.OutputRoot,
			Input:              meta.Input,
			Tags:               append([]string(nil), meta.Tags...),
		})
		for _, child := range sortedCommands(cmd.Commands()) {
			walk(child)
		}
	}
	walk(root)
	sort.Slice(commands, func(i, j int) bool {
		return commands[i].Command < commands[j].Command
	})
	return commands
}

func sortedCommands(commands []*cobra.Command) []*cobra.Command {
	out := append([]*cobra.Command(nil), commands...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].CommandPath() < out[j].CommandPath()
	})
	return out
}

func inferCapabilityMetadata(path string, runnable bool, meta capabilityMetadata) capabilityMetadata {
	if !runnable && meta.Operation == "" {
		meta.Operation = "group"
	}
	if meta.Operation == "" {
		meta.Operation = "read_only"
	}
	if meta.Confirmation == "" {
		meta.Confirmation = "none"
	}
	if meta.OutputRoot == "" {
		meta.OutputRoot = defaultOutputRoot(path)
	}
	if len(meta.Tags) == 0 {
		meta.Tags = defaultCapabilityTags(path)
	}
	return meta
}

func defaultOutputRoot(path string) string {
	parts := strings.Fields(path)
	if len(parts) == 0 {
		return ""
	}
	switch parts[0] {
	case "betaflight-cli":
		if len(parts) > 1 {
			return strings.ReplaceAll(parts[1], "-", "_")
		}
		return ""
	default:
		return strings.ReplaceAll(parts[0], "-", "_")
	}
}

func defaultCapabilityTags(path string) []string {
	parts := strings.Fields(path)
	if len(parts) < 2 {
		return []string{"offline"}
	}
	tags := []string{parts[1]}
	if len(parts) > 2 {
		tags = append(tags, parts[2])
	}
	return tags
}

func capabilityMetadataRegistry() map[string]capabilityMetadata {
	readMSP := capabilityMetadata{RequiresConnection: true, Operation: "read_only", Confirmation: "none", Tags: []string{"msp", "read"}}
	readCLI := capabilityMetadata{RequiresConnection: true, Operation: "read_only", Confirmation: "none", Tags: []string{"cli", "read"}}
	offline := capabilityMetadata{RequiresConnection: false, Operation: "offline", Confirmation: "none", Tags: []string{"offline"}}
	writePlan := capabilityMetadata{RequiresConnection: false, Operation: "offline", Confirmation: "none", Input: "plain CLI text or JSON plan", Tags: []string{"plan", "offline"}}
	writeApply := capabilityMetadata{RequiresConnection: true, Operation: "write", Confirmation: "--yes", Input: "plain CLI text or JSON plan", Tags: []string{"write", "apply"}}
	dangerous := capabilityMetadata{RequiresConnection: true, Operation: "dangerous", Confirmation: "--yes", Tags: []string{"dangerous"}}
	registry := map[string]capabilityMetadata{
		"betaflight-cli capabilities":           {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "capabilities", Tags: []string{"offline", "discovery"}},
		"betaflight-cli version":                {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "version", Tags: []string{"offline", "metadata"}},
		"betaflight-cli ports":                  {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "ports", Tags: []string{"offline", "ports"}},
		"betaflight-cli ports list":             {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "ports", Tags: []string{"offline", "ports"}},
		"betaflight-cli ports diagnose":         {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "diagnostics", Tags: []string{"offline", "ports"}},
		"betaflight-cli doctor":                 {RequiresConnection: false, Operation: "offline_or_read_only_probe", Confirmation: "none", OutputRoot: "ports", Tags: []string{"offline", "diagnostics"}},
		"betaflight-cli blackbox inspect":       {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "inspection", Input: "Blackbox log file or stdin", Tags: []string{"blackbox", "offline"}},
		"betaflight-cli configuration validate": {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "configuration_validation", Input: "raw CLI text or JSON lines/raw", Tags: []string{"configuration", "validate", "offline"}},
		"betaflight-cli batch plan":             writePlan,
		"betaflight-cli restore plan":           writePlan,
		"betaflight-cli presets plan":           writePlan,
		"betaflight-cli batch apply":            writeApply,
		"betaflight-cli restore apply":          writeApply,
		"betaflight-cli presets apply":          writeApply,
		"betaflight-cli cli exec":               {RequiresConnection: true, Operation: "read_only_or_write_or_dangerous", Confirmation: "--yes for writes and dangerous CLI lines", OutputRoot: "cli", Input: "one Betaflight CLI command line", Tags: []string{"cli", "passthrough"}},
		"betaflight-cli cli interactive":        dangerous,
		"betaflight-cli settings set": {
			RequiresConnection: true,
			Operation:          "write_when_apply_is_set",
			Confirmation:       "--yes with --apply",
			OutputRoot:         "change_plan",
			Tags:               []string{"settings", "write"},
		},
		"betaflight-cli save": dangerous,
	}
	for _, path := range []string{
		"betaflight-cli info",
		"betaflight-cli firmware status",
		"betaflight-cli target status",
		"betaflight-cli text status",
		"betaflight-cli status",
		"betaflight-cli configuration status",
		"betaflight-cli configuration snapshot",
		"betaflight-cli configuration compare",
		"betaflight-cli configuration export",
		"betaflight-cli system status",
		"betaflight-cli tasks status",
		"betaflight-cli telemetry snapshot",
		"betaflight-cli debug status",
		"betaflight-cli environment status",
		"betaflight-cli rtc status",
		"betaflight-cli backup create",
		"betaflight-cli backup diff",
		"betaflight-cli storage status",
		"betaflight-cli sensors status",
		"betaflight-cli blackbox config",
		"betaflight-cli beeper config",
		"betaflight-cli transponder config",
		"betaflight-cli mixer status",
		"betaflight-cli motors status",
		"betaflight-cli settings list",
		"betaflight-cli settings get",
		"betaflight-cli settings diff",
		"betaflight-cli features list",
		"betaflight-cli features status",
		"betaflight-cli serial list",
		"betaflight-cli serial status",
		"betaflight-cli modes list",
		"betaflight-cli modes active",
		"betaflight-cli resources list",
		"betaflight-cli resources status",
		"betaflight-cli profiles list",
		"betaflight-cli profiles status",
		"betaflight-cli rateprofiles list",
		"betaflight-cli vtxtable list",
		"betaflight-cli vtx config",
		"betaflight-cli leds list",
		"betaflight-cli leds status",
		"betaflight-cli servos list",
		"betaflight-cli servos status",
		"betaflight-cli adjustments list",
		"betaflight-cli adjustments status",
		"betaflight-cli rxrange list",
		"betaflight-cli pid list",
		"betaflight-cli pid status",
		"betaflight-cli rates list",
		"betaflight-cli rates status",
		"betaflight-cli filters list",
		"betaflight-cli filters status",
		"betaflight-cli receiver list",
		"betaflight-cli receiver status",
		"betaflight-cli receiver rxfail",
		"betaflight-cli gps list",
		"betaflight-cli gps status",
		"betaflight-cli osd list",
		"betaflight-cli osd status",
		"betaflight-cli battery list",
		"betaflight-cli battery status",
		"betaflight-cli failsafe list",
		"betaflight-cli failsafe status",
	} {
		registry[path] = readMSP
		if strings.Contains(path, " list") || strings.Contains(path, "backup ") || strings.Contains(path, "configuration export") || strings.Contains(path, "configuration snapshot") {
			meta := readCLI
			registry[path] = meta
		}
	}
	for _, path := range []string{
		"betaflight-cli features enable",
		"betaflight-cli features disable",
		"betaflight-cli serial set",
		"betaflight-cli modes set",
		"betaflight-cli resources set",
		"betaflight-cli profiles select",
		"betaflight-cli profiles rate-select",
		"betaflight-cli profiles battery-select",
		"betaflight-cli rateprofiles select",
		"betaflight-cli vtxtable set",
		"betaflight-cli leds set",
		"betaflight-cli servos set",
		"betaflight-cli servos reverse",
		"betaflight-cli adjustments set",
		"betaflight-cli rxrange set",
		"betaflight-cli receiver rxfail",
		"betaflight-cli pid set",
		"betaflight-cli rates set",
		"betaflight-cli filters set",
		"betaflight-cli vtx set",
		"betaflight-cli osd set",
		"betaflight-cli gps set",
		"betaflight-cli battery set",
		"betaflight-cli failsafe set",
	} {
		registry[path] = capabilityMetadata{RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Tags: []string{"configuration", "write"}}
	}
	for _, path := range []string{
		"betaflight-cli reboot firmware",
		"betaflight-cli reboot bootloader",
		"betaflight-cli reboot bootloader-flash",
		"betaflight-cli reboot msc",
		"betaflight-cli reboot msc-utc",
		"betaflight-cli msp request",
	} {
		registry[path] = dangerous
	}
	_ = offline
	return registry
}

func capabilityWorkflows() []capabilityWorkflow {
	return []capabilityWorkflow{
		{
			Name:        "identify-controller",
			Purpose:     "Discover firmware, target, board, and support status before deeper reads.",
			Commands:    []string{"betaflight-cli ports diagnose", "betaflight-cli info", "betaflight-cli firmware status", "betaflight-cli target status"},
			SafetyClass: "read_only",
		},
		{
			Name:        "backup-before-change",
			Purpose:     "Capture authoritative CLI text and parsed inventory before planning writes.",
			Commands:    []string{"betaflight-cli configuration snapshot", "betaflight-cli configuration export --source full --raw-cli"},
			SafetyClass: "read_only",
			Notes:       []string{"raw CLI output remains the restore source of truth"},
		},
		{
			Name:        "validate-local-restore",
			Purpose:     "Validate imported CLI text without connecting to hardware.",
			Commands:    []string{"betaflight-cli configuration validate --file backup.cli", "betaflight-cli restore plan --file backup.cli"},
			SafetyClass: "offline",
		},
		{
			Name:        "apply-reviewed-change",
			Purpose:     "Apply only a reviewed plan and persist with an explicit save decision.",
			Commands:    []string{"betaflight-cli batch plan", "betaflight-cli batch apply --yes", "betaflight-cli save --yes"},
			SafetyClass: "write_then_dangerous",
			Notes:       []string{"save is a separate dangerous operation unless an apply command exposes an explicit save option"},
		},
		{
			Name:        "inspect-runtime",
			Purpose:     "Read live runtime state without mutating the controller.",
			Commands:    []string{"betaflight-cli status", "betaflight-cli telemetry snapshot", "betaflight-cli sensors status", "betaflight-cli motors status"},
			SafetyClass: "read_only",
		},
	}
}
