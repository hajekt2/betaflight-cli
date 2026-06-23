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

type coverageReport struct {
	Source        string           `json:"source"`
	SchemaVersion string           `json:"schema_version"`
	Summary       coverageSummary  `json:"summary"`
	Domains       []coverageDomain `json:"domains"`
	NextGaps      []coverageGap    `json:"next_gaps"`
}

type coverageSummary struct {
	DomainCount          int  `json:"domain_count"`
	ImplementedCount     int  `json:"implemented_count"`
	PartialCount         int  `json:"partial_count"`
	PlannedCount         int  `json:"planned_count"`
	ReadDomains          int  `json:"read_domains"`
	WriteDomains         int  `json:"write_domains"`
	DangerousDomains     int  `json:"dangerous_domains"`
	BlackboxDomains      int  `json:"blackbox_domains"`
	MaintenanceDomains   int  `json:"maintenance_domains"`
	OfflineOnlyDomains   int  `json:"offline_only_domains"`
	UnsupportedOldFWNote bool `json:"unsupported_old_firmware_note"`
}

type coverageDomain struct {
	Domain            string   `json:"domain"`
	Status            string   `json:"status"`
	ReadCommands      []string `json:"read_commands,omitempty"`
	WriteCommands     []string `json:"write_commands,omitempty"`
	DangerousCommands []string `json:"dangerous_commands,omitempty"`
	OutputRoots       []string `json:"output_roots,omitempty"`
	Notes             []string `json:"notes,omitempty"`
}

type coverageGap struct {
	Domain     string   `json:"domain"`
	Reason     string   `json:"reason"`
	NextSteps  []string `json:"next_steps"`
	SafetyNote string   `json:"safety_note,omitempty"`
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
	cmd.AddCommand(&cobra.Command{
		Use:   "coverage",
		Short: "Print non-graphical Configurator parity coverage by domain",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"coverage": buildCoverageReport(cmd.Root()),
			}))
		},
	})
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

func buildCoverageReport(root *cobra.Command) coverageReport {
	commandSet := map[string]bool{}
	for _, command := range collectCapabilityCommands(root) {
		if command.Runnable {
			commandSet[command.Command] = true
		}
	}
	domains := coverageDomains(commandSet)
	report := coverageReport{
		Source:        "curated non-graphical Configurator parity map checked against the Cobra command tree",
		SchemaVersion: output.SchemaVersion,
		Domains:       domains,
		NextGaps:      coverageGaps(domains),
	}
	report.Summary.DomainCount = len(domains)
	for _, domain := range domains {
		switch domain.Status {
		case "implemented":
			report.Summary.ImplementedCount++
		case "partial":
			report.Summary.PartialCount++
		case "planned":
			report.Summary.PlannedCount++
		}
		if len(domain.ReadCommands) > 0 {
			report.Summary.ReadDomains++
		}
		if len(domain.WriteCommands) > 0 {
			report.Summary.WriteDomains++
		}
		if len(domain.DangerousCommands) > 0 {
			report.Summary.DangerousDomains++
		}
		if domain.Domain == "blackbox" {
			report.Summary.BlackboxDomains++
		}
		if domain.Domain == "firmware-maintenance" {
			report.Summary.MaintenanceDomains++
		}
		if len(domain.ReadCommands) == 0 && len(domain.WriteCommands) == 0 && len(domain.DangerousCommands) == 0 {
			report.Summary.OfflineOnlyDomains++
		}
	}
	report.Summary.UnsupportedOldFWNote = true
	return report
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

func coverageDomains(commandSet map[string]bool) []coverageDomain {
	domains := []coverageDomain{
		implementedDomain("identity", commandSet, []string{"betaflight-cli info", "betaflight-cli firmware status", "betaflight-cli target status", "betaflight-cli text status"}, nil, nil, []string{"info", "firmware", "target", "text"}, "MSP identity, board, MCU, UID, build, support policy, and text metadata are typed."),
		implementedDomain("connection-diagnostics", commandSet, []string{"betaflight-cli ports list", "betaflight-cli ports diagnose", "betaflight-cli doctor"}, nil, nil, []string{"ports", "diagnostics"}, "USB serial discovery is implemented; non-USB transports remain intentionally out of scope."),
		implementedDomain("configuration-backup", commandSet, []string{"betaflight-cli backup create", "betaflight-cli backup diff", "betaflight-cli configuration snapshot", "betaflight-cli configuration export", "betaflight-cli configuration compare"}, nil, nil, []string{"configuration", "backup"}, "Raw CLI text remains authoritative and parsed inventories are available for agents."),
		implementedDomain("configuration-restore", commandSet, []string{"betaflight-cli configuration validate", "betaflight-cli restore plan", "betaflight-cli presets plan", "betaflight-cli batch plan"}, []string{"betaflight-cli restore apply", "betaflight-cli presets apply", "betaflight-cli batch apply"}, []string{"betaflight-cli save"}, []string{"configuration_validation", "change_plan"}, "Plan/apply/save is implemented with explicit confirmation and defaults safeguards."),
		implementedDomain("runtime-status", commandSet, []string{"betaflight-cli status", "betaflight-cli telemetry snapshot", "betaflight-cli system status", "betaflight-cli tasks status", "betaflight-cli debug status", "betaflight-cli environment status", "betaflight-cli rtc status"}, nil, nil, []string{"status", "telemetry", "system", "tasks", "debug", "environment", "rtc"}, "Core runtime, telemetry, scheduler, debug, environment, and clock reads are typed."),
		implementedDomain("features", commandSet, []string{"betaflight-cli features list", "betaflight-cli features status"}, []string{"betaflight-cli features enable", "betaflight-cli features disable"}, nil, []string{"features", "change_plan"}, "Feature mask reads and CLI-backed feature plans are implemented."),
		implementedDomain("ports-and-modes", commandSet, []string{"betaflight-cli serial list", "betaflight-cli serial status", "betaflight-cli modes list", "betaflight-cli modes active"}, []string{"betaflight-cli serial set", "betaflight-cli modes set"}, nil, []string{"serial", "modes", "change_plan"}, "Serial rows and AUX modes have read, typed status, and plan/apply surfaces."),
		implementedDomain("resources", commandSet, []string{"betaflight-cli resources list", "betaflight-cli resources status"}, []string{"betaflight-cli resources set"}, nil, []string{"resources", "change_plan"}, "Resource, timer, and DMA reads are implemented with resource assignment planning."),
		implementedDomain("profiles", commandSet, []string{"betaflight-cli profiles list", "betaflight-cli profiles status", "betaflight-cli rateprofiles list"}, []string{"betaflight-cli profiles select", "betaflight-cli profiles rate-select", "betaflight-cli profiles battery-select", "betaflight-cli rateprofiles select"}, nil, []string{"profiles", "change_plan"}, "PID, rate, and battery profile selectors are covered."),
		implementedDomain("pid-rates-filters", commandSet, []string{"betaflight-cli pid list", "betaflight-cli pid status", "betaflight-cli rates list", "betaflight-cli rates status", "betaflight-cli filters list", "betaflight-cli filters status"}, []string{"betaflight-cli pid set", "betaflight-cli rates set", "betaflight-cli filters set"}, nil, []string{"pid", "rates", "filters", "change_plan"}, "Core tuning domains have metadata-backed setting lists, typed status, and setting plans."),
		implementedDomain("receiver", commandSet, []string{"betaflight-cli receiver list", "betaflight-cli receiver status", "betaflight-cli rxrange list"}, []string{"betaflight-cli receiver rxfail", "betaflight-cli rxrange set"}, nil, []string{"receiver", "rxrange", "change_plan"}, "Receiver config, RC channels, RX failsafe rows, and channel ranges are covered."),
		implementedDomain("gps", commandSet, []string{"betaflight-cli gps list", "betaflight-cli gps status"}, []string{"betaflight-cli gps set"}, nil, []string{"gps", "change_plan"}, "GPS config, position, rescue settings, PID terms, and satellite info are covered when firmware supplies them."),
		implementedDomain("battery-failsafe", commandSet, []string{"betaflight-cli battery list", "betaflight-cli battery status", "betaflight-cli failsafe list", "betaflight-cli failsafe status"}, []string{"betaflight-cli battery set", "betaflight-cli failsafe set"}, nil, []string{"battery", "failsafe", "change_plan"}, "Battery and failsafe settings plus arming/failsafe status are covered."),
		implementedDomain("vtx-osd-leds", commandSet, []string{"betaflight-cli vtx config", "betaflight-cli vtx list", "betaflight-cli osd list", "betaflight-cli osd status", "betaflight-cli vtxtable list", "betaflight-cli leds list", "betaflight-cli leds status"}, []string{"betaflight-cli vtx set", "betaflight-cli osd set", "betaflight-cli vtxtable set", "betaflight-cli leds set"}, nil, []string{"vtx", "osd", "vtxtable", "leds", "change_plan"}, "VTX, OSD, VTX table, and LED strip surfaces are implemented without graphical layout editing."),
		implementedDomain("motors-servos-mixer", commandSet, []string{"betaflight-cli mixer status", "betaflight-cli motors status", "betaflight-cli motors test-plan", "betaflight-cli servos list", "betaflight-cli servos status", "betaflight-cli adjustments list", "betaflight-cli adjustments status"}, []string{"betaflight-cli servos set", "betaflight-cli servos reverse", "betaflight-cli adjustments set"}, []string{"betaflight-cli motors test-apply"}, []string{"mixer", "motors", "motor_test_plan", "servos", "adjustments", "change_plan"}, "Motor and servo reads are implemented; motor output testing is bounded, dangerous, and confirmation-gated."),
		implementedDomain("storage-blackbox", commandSet, []string{"betaflight-cli storage status", "betaflight-cli blackbox config", "betaflight-cli blackbox inspect"}, nil, nil, []string{"storage", "blackbox", "inspection"}, "Storage summaries, Blackbox configuration, and initial offline log inspection are implemented."),
		implementedDomain("beeper-transponder", commandSet, []string{"betaflight-cli beeper config", "betaflight-cli transponder config"}, nil, nil, []string{"beeper", "transponder"}, "Beeper and transponder configuration reads are covered."),
		implementedDomain("raw-protocol-access", commandSet, []string{"betaflight-cli cli exec"}, nil, []string{"betaflight-cli cli interactive", "betaflight-cli msp request"}, []string{"cli", "msp"}, "Raw CLI and raw MSP access exist for unsupported gaps with safety gates."),
		implementedDomain("firmware-maintenance", commandSet, nil, nil, []string{"betaflight-cli reboot firmware", "betaflight-cli reboot bootloader", "betaflight-cli reboot bootloader-flash", "betaflight-cli reboot msc", "betaflight-cli reboot msc-utc"}, []string{"reboot"}, "Reboot flows are implemented; firmware flashing remains outside this CLI for now."),
	}
	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Domain < domains[j].Domain
	})
	return domains
}

func implementedDomain(domain string, commandSet map[string]bool, readCommands, writeCommands, dangerousCommands, outputRoots []string, notes ...string) coverageDomain {
	out := coverageDomain{
		Domain:            domain,
		Status:            "implemented",
		ReadCommands:      existingCommands(commandSet, readCommands),
		WriteCommands:     existingCommands(commandSet, writeCommands),
		DangerousCommands: existingCommands(commandSet, dangerousCommands),
		OutputRoots:       append([]string(nil), outputRoots...),
		Notes:             append([]string(nil), notes...),
	}
	missing := missingCommands(commandSet, append(append(append([]string{}, readCommands...), writeCommands...), dangerousCommands...))
	if len(missing) > 0 {
		out.Status = "partial"
		out.Notes = append(out.Notes, "missing runnable commands: "+strings.Join(missing, ", "))
	}
	sort.Strings(out.OutputRoots)
	return out
}

func existingCommands(commandSet map[string]bool, commands []string) []string {
	var out []string
	for _, command := range commands {
		if commandSet[command] {
			out = append(out, command)
		}
	}
	sort.Strings(out)
	return out
}

func missingCommands(commandSet map[string]bool, commands []string) []string {
	var out []string
	for _, command := range commands {
		if !commandSet[command] {
			out = append(out, command)
		}
	}
	sort.Strings(out)
	return out
}

func coverageGaps(domains []coverageDomain) []coverageGap {
	gaps := []coverageGap{
		{
			Domain:     "motor-testing",
			Reason:     "Configurator exposes richer live motor test workflows; this CLI currently provides a bounded single-motor test apply path.",
			NextSteps:  []string{"support controlled all-motor idle tests only with stronger confirmation", "capture timing and elapsed-clock evidence", "consider optional pre- and post-test diff helpers for repeated runs"},
			SafetyNote: "This must be dangerous by default because motors can spin.",
		},
		{
			Domain:     "blackbox-decoding",
			Reason:     "Offline Blackbox inspection currently decodes simple variable-byte samples, not full Blackbox frame streams.",
			NextSteps:  []string{"implement binary frame decoding", "add typed gyro, motor, RC, PID, and event streams", "add JSON summaries suitable for agents"},
			SafetyNote: "Offline only.",
		},
		{
			Domain:     "firmware-flashing",
			Reason:     "Reboot-to-bootloader flows exist, but firmware flashing is not implemented.",
			NextSteps:  []string{"decide whether flashing belongs in this CLI", "if accepted, add target validation and image provenance checks", "require explicit dangerous confirmation"},
			SafetyNote: "Firmware flashing can brick hardware when misused.",
		},
	}
	for _, domain := range domains {
		if domain.Status == "partial" {
			gaps = append(gaps, coverageGap{
				Domain:     domain.Domain,
				Reason:     "The curated parity map references commands that are not currently runnable.",
				NextSteps:  []string{"inspect the domain notes", "add missing commands or update coverage metadata"},
				SafetyNote: "Do not claim implemented parity for this domain until the command map is complete.",
			})
		}
	}
	sort.Slice(gaps, func(i, j int) bool {
		return gaps[i].Domain < gaps[j].Domain
	})
	return gaps
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
		"betaflight-cli motors test-plan":       {RequiresConnection: false, Operation: "offline_dangerous_plan", Confirmation: "none", OutputRoot: "motor_test_plan", Tags: []string{"motors", "dangerous", "plan", "offline"}},
		"betaflight-cli motors test-apply":      {RequiresConnection: true, Operation: "dangerous", Confirmation: "--yes --props-off --battery-aware", OutputRoot: "motor_test_plan", Tags: []string{"motors", "dangerous", "apply"}},
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
		"betaflight-cli motors test-plan",
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
