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
	AutoPortWritesDefault    bool     `json:"auto_port_writes_default"`
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
			AutoPortWritesDefault:    true,
			AutoPortWritesRequireOpt: false,
			SaveIsExplicit:           true,
			Notes: []string{
				"read and write commands may auto-select a single USB flight-controller candidate",
				"write and dangerous commands do not require explicit --auto-port by default",
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
		if strings.Contains(domain.Domain, "blackbox") || containsString(domain.OutputRoots, "blackbox") {
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

func containsString(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
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
	if strings.HasPrefix(path, "betaflight-cli completion ") && meta.Operation == "" {
		meta.Operation = "text_output"
		meta.RequiresConnection = false
		meta.Confirmation = "none"
		meta.Tags = []string{"completion", "shell"}
	}
	if path == "betaflight-cli help" && meta.Operation == "" {
		meta.Operation = "text_output"
		meta.RequiresConnection = false
		meta.Confirmation = "none"
		meta.Tags = []string{"help", "text"}
	}
	if !runnable && meta.Operation == "" {
		meta.Operation = "group"
	}
	if meta.Operation == "" {
		meta.Operation = "read_only"
	}
	if meta.Confirmation == "" {
		meta.Confirmation = "none"
	}
	if meta.OutputRoot == "" && meta.Operation != "group" && meta.Operation != "text_output" {
		meta.OutputRoot = defaultOutputRoot(path)
	}
	if len(meta.Tags) == 0 {
		meta.Tags = defaultCapabilityTags(path)
	}
	return meta
}

func coverageDomains(commandSet map[string]bool) []coverageDomain {
	domains := []coverageDomain{
		implementedDomain("agent-discovery", commandSet, []string{"betaflight-cli version", "betaflight-cli schema", "betaflight-cli capabilities", "betaflight-cli capabilities coverage", "betaflight-cli msp list", "betaflight-cli msp metadata", "betaflight-cli settings metadata"}, nil, nil, []string{"version", "schema", "capabilities", "coverage", "msp", "settings"}, "Machine-readable discovery, schema, capability, MSP registry, and generated settings metadata commands are implemented for agents."),
		implementedDomain("firmware-flashing", commandSet, []string{"betaflight-cli firmware flash"}, nil, []string{"betaflight-cli firmware flash"}, []string{"firmware_flash"}, "External firmware flashing is supported with explicit confirmation and a preflight plan."),
		implementedDomain("identity", commandSet, []string{"betaflight-cli info", "betaflight-cli firmware status", "betaflight-cli target status", "betaflight-cli text status"}, []string{"betaflight-cli text set", "betaflight-cli text set-json"}, nil, []string{"info", "firmware", "target", "text"}, "MSP identity, board, MCU, UID, build, support policy, and text metadata are typed; writable text fields can be changed through confirmed MSP2 with positional or JSON input."),
		implementedDomain("connection-diagnostics", commandSet, []string{"betaflight-cli ports list", "betaflight-cli ports diagnose", "betaflight-cli doctor"}, nil, nil, []string{"ports", "diagnostics"}, "USB serial discovery is implemented; non-USB transports remain intentionally out of scope."),
		implementedDomain("configuration-backup", commandSet, []string{
			"betaflight-cli backup create",
			"betaflight-cli backup diff",
			"betaflight-cli configuration snapshot",
			"betaflight-cli configuration export",
			"betaflight-cli configuration compare",
			"betaflight-cli configuration diff",
			"betaflight-cli configuration status",
			"betaflight-cli configuration plan",
			"betaflight-cli cli diff",
			"betaflight-cli cli dump",
		}, nil, nil, []string{"configuration", "backup"}, "Raw CLI text remains authoritative and parsed inventories are available for agents."),
		implementedDomain("configuration-restore", commandSet, []string{"betaflight-cli configuration validate", "betaflight-cli restore plan", "betaflight-cli presets plan", "betaflight-cli batch plan", "betaflight-cli configuration plan"}, []string{"betaflight-cli restore apply", "betaflight-cli presets apply", "betaflight-cli presets fetch", "betaflight-cli batch apply", "betaflight-cli configuration apply"}, []string{"betaflight-cli save"}, []string{"configuration_validation", "change_plan"}, "Plan/apply/save is implemented with explicit confirmation and defaults safeguards."),
		implementedDomain("runtime-status", commandSet, []string{"betaflight-cli status", "betaflight-cli telemetry", "betaflight-cli telemetry snapshot", "betaflight-cli system status", "betaflight-cli tasks status", "betaflight-cli debug status", "betaflight-cli environment status", "betaflight-cli rtc status"}, []string{"betaflight-cli rtc set", "betaflight-cli rtc set-json", "betaflight-cli debug set-accelerometer-trim", "betaflight-cli debug set-accelerometer-trim-json"}, nil, []string{"status", "telemetry", "system", "tasks", "debug", "environment", "rtc"}, "Core runtime, telemetry, scheduler, debug, environment, and clock reads are typed; RTC time sync and accelerometer trim writes are exposed as confirmed MSP writes with JSON forms for agents."),
		implementedDomain("features", commandSet, []string{"betaflight-cli features list", "betaflight-cli features status"}, []string{"betaflight-cli features enable", "betaflight-cli features disable", "betaflight-cli features set-json", "betaflight-cli features set-mask"}, nil, []string{"features", "feature_mask", "change_plan"}, "Feature mask reads, CLI-backed feature plans, JSON feature operation plans, and complete typed MSP feature mask writes are implemented."),
		implementedDomain("ports-and-modes", commandSet, []string{"betaflight-cli serial list", "betaflight-cli serial status", "betaflight-cli modes list", "betaflight-cli modes active"}, []string{"betaflight-cli serial set", "betaflight-cli serial set-json", "betaflight-cli serial apply-config-json", "betaflight-cli modes set", "betaflight-cli modes set-json", "betaflight-cli modes set-range"}, nil, []string{"serial", "serial_config", "modes", "mode_ranges", "mode_range", "change_plan"}, "Serial rows and AUX modes have read, typed status, and plan/apply surfaces; serial row JSON, full-table serial config, AUX mode range JSON, and AUX mode range writes have confirmed native CLI-backed or typed MSP paths."),
		implementedDomain("resources", commandSet, []string{"betaflight-cli resources list", "betaflight-cli resources status"}, []string{"betaflight-cli resources set", "betaflight-cli resources set-json"}, nil, []string{"resources", "change_plan"}, "Resource, timer, and DMA reads are implemented with resource assignment planning, including JSON input for agent workflows."),
		implementedDomain("profiles", commandSet, []string{"betaflight-cli profiles list", "betaflight-cli profiles status", "betaflight-cli rateprofiles list"}, []string{"betaflight-cli profiles select", "betaflight-cli profiles rate-select", "betaflight-cli profiles battery-select", "betaflight-cli profiles select-json", "betaflight-cli profiles copy", "betaflight-cli profiles copy-json", "betaflight-cli rateprofiles select"}, nil, []string{"profiles", "change_plan", "profile_copy"}, "PID, rate, and battery profile selectors are covered with positional and JSON plan/apply forms; PID/rate profile copy is available through confirmed MSP with positional and JSON input."),
		implementedDomain("pid-rates-filters", commandSet, []string{"betaflight-cli pid list", "betaflight-cli pid status", "betaflight-cli pid preview-simplified-json", "betaflight-cli pid validate-simplified", "betaflight-cli rates list", "betaflight-cli rates status", "betaflight-cli filters list", "betaflight-cli filters status"}, []string{"betaflight-cli pid set", "betaflight-cli pid set-gains-json", "betaflight-cli pid set-advanced-json", "betaflight-cli pid set-simplified-json", "betaflight-cli rates set", "betaflight-cli rates set-profile-json", "betaflight-cli filters set", "betaflight-cli filters set-advanced-json", "betaflight-cli filters set-filter-json"}, nil, []string{"pid", "pid_gains", "pid_advanced", "simplified_tuning", "simplified_tuning_preview", "simplified_tuning_validation", "rates", "rate_profile", "filters", "advanced_config", "filter_config", "change_plan"}, "Core tuning domains have metadata-backed setting lists, typed status, preview, validation, and setting plans; PID gain, PID advanced profile, simplified tuning, rate profile, advanced loop/motor config, and filter config writes have confirmed typed MSP paths."),
		implementedDomain("sensor-maintenance", commandSet, []string{"betaflight-cli sensors status"}, []string{"betaflight-cli sensors set-config", "betaflight-cli sensors set-config-json", "betaflight-cli sensors set-alignment", "betaflight-cli sensors set-alignment-json", "betaflight-cli sensors set-compass-declination", "betaflight-cli sensors set-compass-json"}, []string{"betaflight-cli sensors calibrate-accelerometer", "betaflight-cli sensors calibrate-magnetometer"}, []string{"sensors", "sensor_calibration", "sensor_config", "sensor_alignment", "compass_config"}, "Sensor status, hardware selection, alignment, and compass declination writes are exposed through typed MSP commands with JSON forms for agents; accelerometer/magnetometer calibration actions remain dangerous and confirmation-gated."),
		implementedDomain("receiver", commandSet, []string{"betaflight-cli receiver list", "betaflight-cli receiver status", "betaflight-cli rxrange list"}, []string{"betaflight-cli receiver rxfail", "betaflight-cli receiver set", "betaflight-cli receiver set-config-json", "betaflight-cli receiver set-rxfail", "betaflight-cli receiver set-rxfail-json", "betaflight-cli receiver set-rssi-channel", "betaflight-cli receiver set-rssi-channel-json", "betaflight-cli receiver set-map", "betaflight-cli receiver set-map-json", "betaflight-cli receiver set-deadband", "betaflight-cli receiver set-deadband-json", "betaflight-cli rxrange set", "betaflight-cli rxrange set-json"}, nil, []string{"receiver", "receiver_config", "rxrange", "change_plan", "rx_fail", "rx_fail_table", "rssi_channel", "rc_map", "rc_deadband"}, "Receiver config, RC channels, RX failsafe rows, RSSI channel, RC map, RC deadband, and channel ranges are covered; receiver config, RX failsafe, RSSI channel, RC map, and RC deadband writes have confirmed typed MSP paths with JSON forms, and rxrange rows support native CLI plan/apply with JSON input."),
		implementedDomain("settings", commandSet, []string{"betaflight-cli settings list", "betaflight-cli settings get", "betaflight-cli settings firmware-get", "betaflight-cli settings firmware-info", "betaflight-cli settings diff"}, []string{"betaflight-cli settings set", "betaflight-cli settings set-json"}, nil, []string{"settings", "firmware_setting", "firmware_setting_info", "change_plan"}, "Core setting inventory, targeted CLI reads, MSP2 firmware setting reads, MSP2 firmware setting-info reads, and single or multi-setting plan-aware changes are implemented with dry-run diffs before write."),
		implementedDomain("gps", commandSet, []string{"betaflight-cli gps list", "betaflight-cli gps status"}, []string{"betaflight-cli gps set", "betaflight-cli gps set-config", "betaflight-cli gps set-config-json", "betaflight-cli gps set-rescue", "betaflight-cli gps set-rescue-json", "betaflight-cli gps set-rescue-pids", "betaflight-cli gps set-rescue-pids-json"}, nil, []string{"gps", "change_plan", "gps_config", "gps_rescue", "gps_rescue_pids"}, "GPS config, position, rescue settings, PID terms, and satellite info are covered when firmware supplies them; provider, GPS Rescue, and GPS Rescue PID writes have confirmed typed MSP paths with JSON forms for agents."),
		implementedDomain("battery-failsafe", commandSet, []string{"betaflight-cli battery list", "betaflight-cli battery status", "betaflight-cli failsafe list", "betaflight-cli failsafe status"}, []string{"betaflight-cli battery set", "betaflight-cli battery set-config-json", "betaflight-cli battery set-profile-json", "betaflight-cli battery set-voltage-meter", "betaflight-cli battery set-voltage-meter-json", "betaflight-cli battery set-current-meter", "betaflight-cli battery set-current-meter-json", "betaflight-cli failsafe set", "betaflight-cli failsafe set-arming-json", "betaflight-cli failsafe set-config-json", "betaflight-cli failsafe set-board-alignment", "betaflight-cli failsafe set-board-alignment-json"}, nil, []string{"battery", "battery_config", "battery_profile", "failsafe", "arming_config", "failsafe_config", "change_plan", "board_alignment", "voltage_meter_config", "current_meter_config"}, "Battery and failsafe settings plus arming/failsafe status are covered; battery config, battery profile, voltage/current meter calibration, arming config, failsafe config, and board alignment have confirmed typed MSP writes with JSON forms for agent workflows."),
		implementedDomain("vtx-osd-leds", commandSet, []string{"betaflight-cli vtx config", "betaflight-cli vtx device-status", "betaflight-cli vtx list", "betaflight-cli osd list", "betaflight-cli osd status", "betaflight-cli vtxtable list", "betaflight-cli vtxtable status", "betaflight-cli leds list", "betaflight-cli leds status"}, []string{"betaflight-cli vtx set", "betaflight-cli vtx set-config", "betaflight-cli vtx set-config-json", "betaflight-cli osd set", "betaflight-cli osd set-canvas", "betaflight-cli osd set-general-json", "betaflight-cli osd set-video-system", "betaflight-cli osd set-video-system-json", "betaflight-cli osd set-position", "betaflight-cli osd set-position-json", "betaflight-cli osd set-stat", "betaflight-cli osd set-stat-json", "betaflight-cli osd set-timer", "betaflight-cli osd set-timer-json", "betaflight-cli vtxtable set", "betaflight-cli vtxtable set-json", "betaflight-cli vtxtable set-band", "betaflight-cli vtxtable set-band-json", "betaflight-cli vtxtable set-power", "betaflight-cli vtxtable set-power-json", "betaflight-cli leds set", "betaflight-cli leds set-json", "betaflight-cli leds set-colors-json", "betaflight-cli leds set-mode-color", "betaflight-cli leds set-mode-color-json", "betaflight-cli leds set-values", "betaflight-cli leds set-values-json"}, nil, []string{"vtx", "vtx_device", "vtx_config", "osd", "osd_canvas", "osd_general_config", "osd_video_system", "osd_position", "osd_stat", "osd_timer", "vtxtable", "vtxtable_band", "vtxtable_power", "leds", "change_plan", "led_colors", "led_mode_color", "led_values"}, "VTX, OSD, VTX table, and LED strip surfaces are implemented without graphical layout editing; VTX config, VTX device status, and VTX table status have typed MSP reads, VTX config has positional and JSON typed MSP writes, OSD canvas/general/video/item/stat/timer writes have JSON forms for agents, and VTX table rows, full VTX table JSON writes, LED strip rows, LED color tables, LED mode colors, and LED value tuning have confirmed native CLI or typed MSP writes with JSON forms."),
		implementedDomain("motors-servos-mixer", commandSet, []string{"betaflight-cli mixer status", "betaflight-cli motors status", "betaflight-cli motors test-plan", "betaflight-cli servos list", "betaflight-cli servos status", "betaflight-cli adjustments list", "betaflight-cli adjustments status"}, []string{"betaflight-cli mixer set-config-json", "betaflight-cli motors set-config", "betaflight-cli motors set-config-json", "betaflight-cli motors set-3d-config", "betaflight-cli motors set-3d-config-json", "betaflight-cli servos set", "betaflight-cli servos reverse", "betaflight-cli servos reverse-json", "betaflight-cli servos set-json", "betaflight-cli servos set-config", "betaflight-cli servos set-config-json", "betaflight-cli servos set-mix-rule", "betaflight-cli servos set-mix-rule-json", "betaflight-cli adjustments set", "betaflight-cli adjustments set-json", "betaflight-cli adjustments set-range", "betaflight-cli adjustments set-range-json"}, []string{"betaflight-cli motors test-apply"}, []string{"mixer", "mixer_config", "motors", "motor_config", "motor_3d_config", "motor_test_plan", "servos", "servo_table", "servo_config", "servo_mix_rule", "adjustments", "adjustment_table", "adjustment_range", "change_plan"}, "Mixer, motor, and servo reads are implemented; mixer configuration, motor configuration, 3D motor configuration, servo table JSON, servo row, servo reverse row JSON, servo mix rule, adjustment range JSON, and adjustment range changes have confirmed typed MSP or native CLI-backed writes with JSON forms for motor, servo, and adjustment configuration; motor output testing is bounded, dangerous, and confirmation-gated."),
		implementedDomain("storage-blackbox", commandSet, []string{"betaflight-cli storage status", "betaflight-cli storage export", "betaflight-cli blackbox config", "betaflight-cli blackbox inspect", "betaflight-cli blackbox list", "betaflight-cli blackbox export"}, []string{"betaflight-cli blackbox set-config-json"}, []string{"betaflight-cli storage erase"}, []string{"storage", "dataflash_export", "storage_erase", "blackbox", "blackbox_config", "blackbox_logs", "blackbox_export", "inspection"}, "Storage summaries, Dataflash export, Dataflash erase, Blackbox configuration, Blackbox config writes, Blackbox log listing, Blackbox export, and offline log inspection are implemented."),
		implementedDomain("beeper-transponder", commandSet, []string{"betaflight-cli beeper config", "betaflight-cli transponder config"}, []string{"betaflight-cli beeper enable", "betaflight-cli beeper disable", "betaflight-cli beeper set-json", "betaflight-cli beeper set-config", "betaflight-cli beeper set-config-json", "betaflight-cli transponder set-provider", "betaflight-cli transponder set-data", "betaflight-cli transponder set-json", "betaflight-cli transponder set-config", "betaflight-cli transponder set-config-json"}, nil, []string{"beeper", "beeper_config", "transponder", "transponder_config", "change_plan"}, "Beeper and transponder configuration reads are covered; beeper and transponder full-config writes have positional and JSON typed MSP paths, and convenience beeper/transponder writes have positional and JSON CLI-backed plan/apply forms."),
		implementedDomain("raw-protocol-access", commandSet, []string{"betaflight-cli cli exec", "betaflight-cli msp batch"}, nil, []string{"betaflight-cli cli interactive", "betaflight-cli msp request"}, []string{"cli", "msp"}, "Raw CLI and raw MSP access exist for unsupported gaps with safety gates."),
		implementedDomain("firmware-maintenance", commandSet, nil, nil, []string{"betaflight-cli reboot firmware", "betaflight-cli reboot bootloader", "betaflight-cli reboot bootloader-flash", "betaflight-cli reboot msc", "betaflight-cli reboot msc-utc"}, []string{"reboot"}, "Maintenance includes reboot flows and dangerous firmware flashing."),
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
	gaps := []coverageGap{}
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
	writePlan := capabilityMetadata{RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "change_plan", Input: "plain CLI text or JSON plan", Tags: []string{"plan", "offline"}}
	writeApply := capabilityMetadata{RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "change_plan", Input: "plain CLI text or JSON plan", Tags: []string{"write", "apply"}}
	dangerous := capabilityMetadata{RequiresConnection: true, Operation: "dangerous", Confirmation: "--yes", Tags: []string{"dangerous"}}
	registry := map[string]capabilityMetadata{
		"betaflight-cli capabilities":                    {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "capabilities", Tags: []string{"offline", "discovery"}},
		"betaflight-cli capabilities coverage":           {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "coverage", Tags: []string{"offline", "discovery"}},
		"betaflight-cli schema":                          {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "schema", Tags: []string{"offline", "metadata"}},
		"betaflight-cli version":                         {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "version", Tags: []string{"offline", "metadata"}},
		"betaflight-cli ports":                           {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "ports", Tags: []string{"offline", "ports"}},
		"betaflight-cli ports list":                      {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "ports", Tags: []string{"offline", "ports"}},
		"betaflight-cli ports diagnose":                  {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "diagnostics", Tags: []string{"offline", "ports"}},
		"betaflight-cli doctor":                          {RequiresConnection: false, Operation: "offline_or_read_only_probe", Confirmation: "none", OutputRoot: "ports", Tags: []string{"offline", "diagnostics"}},
		"betaflight-cli blackbox inspect":                {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "inspection", Input: "Blackbox log file or stdin", Tags: []string{"blackbox", "offline"}},
		"betaflight-cli blackbox list":                   {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "blackbox_logs", Tags: []string{"blackbox", "read", "list"}},
		"betaflight-cli blackbox export":                 {RequiresConnection: true, Operation: "read_only", Confirmation: "--force only for existing local files", OutputRoot: "blackbox_export", Input: "local output file path", Tags: []string{"blackbox", "read", "export"}},
		"betaflight-cli blackbox set-config-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "blackbox_config", Input: "JSON object or object with blackbox/blackbox_config", Tags: []string{"blackbox", "write"}},
		"betaflight-cli configuration validate":          {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "configuration_validation", Input: "raw CLI text or JSON lines/raw", Tags: []string{"configuration", "validate", "offline"}},
		"betaflight-cli features set-json":               {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply", OutputRoot: "change_plan", Input: "JSON object with enable/enabled and disable/disabled feature name arrays", Tags: []string{"features", "write", "plan"}},
		"betaflight-cli features set-mask":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "feature_mask", Tags: []string{"features", "write"}},
		"betaflight-cli serial set-json":                 {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON object with port/identifier/id, function_mask, and msp/gps/telemetry/blackbox baud values, optionally nested under serial, port, row, or set", Tags: []string{"serial", "write", "plan"}},
		"betaflight-cli beeper set-config":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "beeper_config", Tags: []string{"beeper", "write"}},
		"betaflight-cli beeper set-json":                 {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON object with enable/enabled and disable/disabled beeper modes, optionally nested under beeper, changes, or set", Tags: []string{"beeper", "write", "plan"}},
		"betaflight-cli beeper set-config-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "beeper_config", Input: "JSON object or object with beeper_config/beeper/config", Tags: []string{"beeper", "write"}},
		"betaflight-cli transponder set-config":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "transponder_config", Tags: []string{"transponder", "write"}},
		"betaflight-cli transponder set-json":            {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON object with provider/name/provider_name and data/data_bytes/bytes/data_hex, optionally nested under transponder, changes, or set", Tags: []string{"transponder", "write", "plan"}},
		"betaflight-cli transponder set-config-json":     {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "transponder_config", Input: "JSON object or object with transponder_config/transponder/config; data or data_hex", Tags: []string{"transponder", "write"}},
		"betaflight-cli vtx set-config":                  {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "vtx_config", Tags: []string{"vtx", "write"}},
		"betaflight-cli vtx set-config-json":             {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "vtx_config", Input: "JSON object or object with vtx_config/vtx/config", Tags: []string{"vtx", "write"}},
		"betaflight-cli vtx device-status":               {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "vtx_device", Tags: []string{"vtx", "read"}},
		"betaflight-cli vtxtable status":                 {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "vtxtable", Tags: []string{"vtxtable", "read"}},
		"betaflight-cli osd set-canvas":                  {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_canvas", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-general-json":            {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_general_config", Input: "JSON object or object with osd_general_config/osd/config", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-video-system":            {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_video_system", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-video-system-json":       {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_video_system", Input: "JSON object or object with osd/osd_video_system/config/value", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-position":                {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_position", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-position-json":           {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_position", Input: "JSON object or object with osd_position/position/config", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-stat":                    {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_stat", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-stat-json":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_stat", Input: "JSON object or object with osd_stat/stat/config", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-timer":                   {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_timer", Tags: []string{"osd", "write"}},
		"betaflight-cli osd set-timer-json":              {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "osd_timer", Input: "JSON object or object with osd_timer/timer/config", Tags: []string{"osd", "write"}},
		"betaflight-cli vtxtable set-json":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "vtxtable", Input: "JSON object or object with vtxtable/vtx_table", Tags: []string{"vtxtable", "write"}},
		"betaflight-cli vtxtable set-band":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "vtxtable_band", Tags: []string{"vtxtable", "write"}},
		"betaflight-cli vtxtable set-band-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "vtxtable_band", Input: "JSON object or object with vtxtable_band/band/config/vtxtable.band", Tags: []string{"vtxtable", "write"}},
		"betaflight-cli vtxtable set-power":              {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "vtxtable_power", Tags: []string{"vtxtable", "write"}},
		"betaflight-cli vtxtable set-power-json":         {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "vtxtable_power", Input: "JSON object or object with vtxtable_power/power/config/vtxtable.power", Tags: []string{"vtxtable", "write"}},
		"betaflight-cli serial apply-config-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "serial_config", Input: "JSON array or object with ports", Tags: []string{"serial", "write"}},
		"betaflight-cli pid set-gains-json":              {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "pid_gains", Input: "JSON array or object with gains", Tags: []string{"pid", "write"}},
		"betaflight-cli pid set-advanced-json":           {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "pid_advanced", Input: "JSON object or object with pid_advanced", Tags: []string{"pid", "write"}},
		"betaflight-cli pid preview-simplified-json":     {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "simplified_tuning_preview", Input: "JSON object or object with simplified_tuning", Tags: []string{"pid", "simplified_tuning", "preview", "read"}},
		"betaflight-cli pid validate-simplified":         {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "simplified_tuning_validation", Tags: []string{"pid", "simplified_tuning", "validate", "read"}},
		"betaflight-cli pid set-simplified-json":         {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "simplified_tuning", Input: "JSON object or object with simplified_tuning", Tags: []string{"pid", "simplified_tuning", "write"}},
		"betaflight-cli rates set-profile-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rate_profile", Input: "JSON object or object with rate_profile", Tags: []string{"rates", "write"}},
		"betaflight-cli filters set-advanced-json":       {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "advanced_config", Input: "JSON object or object with advanced_config", Tags: []string{"filters", "write"}},
		"betaflight-cli filters set-filter-json":         {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "filter_config", Input: "JSON object or object with filter_config", Tags: []string{"filters", "write"}},
		"betaflight-cli mixer set-config-json":           {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "mixer_config", Input: "JSON object or object with mixer_config", Tags: []string{"mixer", "write"}},
		"betaflight-cli motors test-plan":                {RequiresConnection: false, Operation: "offline_dangerous_plan", Confirmation: "none", OutputRoot: "motor_test_plan", Tags: []string{"motors", "dangerous", "plan", "offline"}},
		"betaflight-cli motors test-apply":               {RequiresConnection: true, Operation: "dangerous", Confirmation: "--yes --props-off --battery-aware", OutputRoot: "motor_test_plan", Tags: []string{"motors", "dangerous", "apply"}},
		"betaflight-cli motors set-config":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "motor_config", Tags: []string{"motors", "write"}},
		"betaflight-cli motors set-config-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "motor_config", Input: "JSON object or object with motor_config/motors/config", Tags: []string{"motors", "write"}},
		"betaflight-cli motors set-3d-config":            {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "motor_3d_config", Tags: []string{"motors", "write"}},
		"betaflight-cli motors set-3d-config-json":       {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "motor_3d_config", Input: "JSON object or object with motor_3d_config/motors/config", Tags: []string{"motors", "write"}},
		"betaflight-cli modes set-json":                  {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "mode_ranges", Input: "JSON array or object with ranges/mode_ranges/modes", Tags: []string{"modes", "write"}},
		"betaflight-cli modes set-range":                 {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "mode_range", Tags: []string{"modes", "write"}},
		"betaflight-cli failsafe set-arming-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "arming_config", Input: "JSON object or object with arming_config", Tags: []string{"failsafe", "arming", "write"}},
		"betaflight-cli failsafe set-config-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "failsafe_config", Input: "JSON object or object with failsafe_config", Tags: []string{"failsafe", "write"}},
		"betaflight-cli battery set-profile-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "battery_profile", Input: "JSON object or object with battery_profile", Tags: []string{"battery", "profile", "write"}},
		"betaflight-cli servos set-json":                 {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "servo_table", Input: "JSON object or object with servo_table/servos", Tags: []string{"servos", "write"}},
		"betaflight-cli servos set-config":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "servo_config", Tags: []string{"servos", "write"}},
		"betaflight-cli servos set-config-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "servo_config", Input: "JSON object or object with servo_config/servo/config", Tags: []string{"servos", "write"}},
		"betaflight-cli servos set-mix-rule":             {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "servo_mix_rule", Tags: []string{"servos", "write"}},
		"betaflight-cli servos set-mix-rule-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "servo_mix_rule", Input: "JSON object or object with servo_mix_rule/mix_rule/rule", Tags: []string{"servos", "write"}},
		"betaflight-cli adjustments set-json":            {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "adjustment_table", Input: "JSON array or object with ranges/adjustment_table/adjustments", Tags: []string{"adjustments", "write"}},
		"betaflight-cli adjustments set-range":           {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "adjustment_range", Tags: []string{"adjustments", "write"}},
		"betaflight-cli adjustments set-range-json":      {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "adjustment_range", Input: "JSON object or object with adjustment_range/range/config", Tags: []string{"adjustments", "write"}},
		"betaflight-cli storage export":                  {RequiresConnection: true, Operation: "read_only", Confirmation: "--force only for existing local files", OutputRoot: "dataflash_export", Input: "local output file path", Tags: []string{"storage", "read", "export"}},
		"betaflight-cli storage erase":                   {RequiresConnection: true, Operation: "dangerous", Confirmation: "--yes", OutputRoot: "storage_erase", Tags: []string{"storage", "dangerous", "erase"}},
		"betaflight-cli sensors calibrate-accelerometer": {RequiresConnection: true, Operation: "dangerous", Confirmation: "--yes", OutputRoot: "sensor_calibration", Tags: []string{"sensors", "calibration", "dangerous"}},
		"betaflight-cli sensors calibrate-magnetometer":  {RequiresConnection: true, Operation: "dangerous", Confirmation: "--yes", OutputRoot: "sensor_calibration", Tags: []string{"sensors", "calibration", "dangerous"}},
		"betaflight-cli configuration plan":              {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "change_plan", Input: "raw CLI text or JSON lines/raw", Tags: []string{"configuration", "plan", "offline"}},
		"betaflight-cli settings firmware-get":           {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "firmware_setting", Tags: []string{"settings", "read", "firmware"}},
		"betaflight-cli settings firmware-info":          {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "firmware_setting_info", Tags: []string{"settings", "read", "firmware"}},
		"betaflight-cli profiles select-json":            {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON object with profile/pid_profile, rate_profile/rateprofile, and battery_profile indexes", Tags: []string{"profiles", "write", "plan"}},
		"betaflight-cli batch plan":                      writePlan,
		"betaflight-cli restore plan":                    writePlan,
		"betaflight-cli presets plan":                    writePlan,
		"betaflight-cli presets fetch":                   {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "change_plan", Input: "remote preset URL", Tags: []string{"presets", "plan", "offline"}},
		"betaflight-cli batch apply":                     writeApply,
		"betaflight-cli restore apply":                   writeApply,
		"betaflight-cli presets apply":                   writeApply,
		"betaflight-cli configuration apply":             writeApply,
		"betaflight-cli cli diff":                        readCLI,
		"betaflight-cli cli dump":                        readCLI,
		"betaflight-cli cli exec":                        {RequiresConnection: true, Operation: "read_only_or_write_or_dangerous", Confirmation: "--yes for writes and dangerous CLI lines", OutputRoot: "cli", Input: "one Betaflight CLI command line", Tags: []string{"cli", "passthrough"}},
		"betaflight-cli cli interactive":                 dangerous,
		"betaflight-cli firmware flash":                  {RequiresConnection: false, Operation: "plan_or_dangerous_execute", Confirmation: "--yes and --execute to run external flash tool; --reboot-first also connects to the Flight Controller", OutputRoot: "firmware_flash", Tags: []string{"firmware", "maintenance", "dangerous"}},
		"betaflight-cli msp list":                        {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "msp", Tags: []string{"offline", "diagnostic"}},
		"betaflight-cli msp metadata":                    {RequiresConnection: false, Operation: "offline", Confirmation: "none", OutputRoot: "msp", Tags: []string{"offline", "diagnostic"}},
		"betaflight-cli msp batch":                       {RequiresConnection: true, Operation: "read_only", Confirmation: "none", OutputRoot: "msp", Tags: []string{"msp", "raw", "batch"}},
		"betaflight-cli msp request":                     {RequiresConnection: true, Operation: "read_only_or_write_or_dangerous", Confirmation: "--yes for generated write-like commands and numeric commands without compiled metadata", Tags: []string{"msp", "raw"}},
		"betaflight-cli settings set": {
			RequiresConnection: true,
			Operation:          "write_when_apply_is_set",
			Confirmation:       "--yes with --apply",
			OutputRoot:         "change_plan",
			Tags:               []string{"settings", "write"},
		},
		"betaflight-cli settings set-json": {
			RequiresConnection: true,
			Operation:          "write_when_apply_is_set",
			Confirmation:       "--yes with --apply",
			OutputRoot:         "change_plan",
			Input:              "JSON object map or array of {name,value} rows, optionally under settings or changes",
			Tags:               []string{"settings", "write", "plan"},
		},
		"betaflight-cli rtc set":                           {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rtc", Tags: []string{"rtc", "write"}},
		"betaflight-cli rtc set-json":                      {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rtc", Input: "JSON object with timestamp/timestamp_utc/iso_utc or now, optionally nested under rtc", Tags: []string{"rtc", "write"}},
		"betaflight-cli debug set-accelerometer-trim":      {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "accelerometer_trim", Tags: []string{"debug", "write"}},
		"betaflight-cli debug set-accelerometer-trim-json": {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "accelerometer_trim", Input: "JSON object or object with accelerometer_trim/trim/config", Tags: []string{"debug", "write"}},
		"betaflight-cli sensors set-config":                {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "sensor_config", Tags: []string{"sensors", "write"}},
		"betaflight-cli sensors set-config-json":           {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "sensor_config", Input: "JSON object or object with sensor_config/hardware_config/config/sensors.hardware_config", Tags: []string{"sensors", "write"}},
		"betaflight-cli sensors set-alignment":             {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "sensor_alignment", Tags: []string{"sensors", "alignment", "write"}},
		"betaflight-cli sensors set-alignment-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "sensor_alignment", Input: "JSON object or object with sensor_alignment/alignment/config/sensors.alignment", Tags: []string{"sensors", "alignment", "write"}},
		"betaflight-cli sensors set-compass-declination":   {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "compass_config", Tags: []string{"sensors", "compass", "write"}},
		"betaflight-cli sensors set-compass-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "compass_config", Input: "JSON object or object with compass_config/compass/config/sensors.compass", Tags: []string{"sensors", "compass", "write"}},
		"betaflight-cli profiles copy":                     {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "profile_copy", Tags: []string{"profiles", "write"}},
		"betaflight-cli profiles copy-json":                {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "profile_copy", Input: "JSON object with kind, source, and destination, optionally nested under profile_copy, copy, or request", Tags: []string{"profiles", "write"}},
		"betaflight-cli servos reverse-json":               {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON object with servo, source, and mode/reverse/reversed, optionally nested under servo_reverse, reverse_row, reverse, or smix", Tags: []string{"servos", "write", "plan"}},
		"betaflight-cli text set":                          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "text", Tags: []string{"text", "write"}},
		"betaflight-cli text set-json":                     {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "text", Input: "JSON object with field/key and value, or object with text/set/request", Tags: []string{"text", "write"}},
		"betaflight-cli failsafe set-board-alignment":      {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "board_alignment", Tags: []string{"failsafe", "board_alignment", "write"}},
		"betaflight-cli failsafe set-board-alignment-json": {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "board_alignment", Input: "JSON object or object with board_alignment/alignment/config/failsafe.board_alignment", Tags: []string{"failsafe", "board_alignment", "write"}},
		"betaflight-cli receiver set-config-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "receiver_config", Input: "JSON object or object with receiver_config", Tags: []string{"receiver", "write"}},
		"betaflight-cli receiver set-rssi-channel":         {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rssi_channel", Tags: []string{"receiver", "rssi", "write"}},
		"betaflight-cli receiver set-rssi-channel-json":    {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rssi_channel", Input: "JSON object with channel/rssi_channel/value or receiver.rssi_channel", Tags: []string{"receiver", "rssi", "write"}},
		"betaflight-cli receiver set-rxfail":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rx_fail", Tags: []string{"receiver", "failsafe", "write"}},
		"betaflight-cli receiver set-rxfail-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rx_fail_table", Input: "JSON array or object with rx_fail_table/channels/rx_fail/failsafe", Tags: []string{"receiver", "failsafe", "write"}},
		"betaflight-cli receiver set-map":                  {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rc_map", Tags: []string{"receiver", "map", "write"}},
		"betaflight-cli receiver set-map-json":             {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rc_map", Input: "JSON array or object with rc_map/map/receiver.rc_map", Tags: []string{"receiver", "map", "write"}},
		"betaflight-cli receiver set-deadband":             {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rc_deadband", Tags: []string{"receiver", "deadband", "write"}},
		"betaflight-cli receiver set-deadband-json":        {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "rc_deadband", Input: "JSON object or object with rc_deadband/deadband/config/receiver.deadband", Tags: []string{"receiver", "deadband", "write"}},
		"betaflight-cli rxrange set-json":                  {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON array or object with rxranges/ranges/rxrange/range", Tags: []string{"receiver", "rxrange", "write"}},
		"betaflight-cli resources set-json":                {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON object, array, or object with resource/resources/row/rows", Tags: []string{"resources", "write"}},
		"betaflight-cli gps set-config":                    {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "gps_config", Tags: []string{"gps", "write"}},
		"betaflight-cli gps set-config-json":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "gps_config", Input: "JSON object or object with gps_config/config/gps.config", Tags: []string{"gps", "write"}},
		"betaflight-cli gps set-rescue":                    {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "gps_rescue", Tags: []string{"gps", "rescue", "write"}},
		"betaflight-cli gps set-rescue-json":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "gps_rescue", Input: "JSON object or object with gps_rescue/rescue/config/gps.rescue", Tags: []string{"gps", "rescue", "write"}},
		"betaflight-cli gps set-rescue-pids":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "gps_rescue_pids", Tags: []string{"gps", "rescue", "write"}},
		"betaflight-cli gps set-rescue-pids-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "gps_rescue_pids", Input: "JSON object or object with gps_rescue_pids/gps_rescue_pid/rescue_pid/config/gps.rescue_pid", Tags: []string{"gps", "rescue", "write"}},
		"betaflight-cli leds set-colors-json":              {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "led_colors", Input: "JSON array or object with colors", Tags: []string{"leds", "colors", "write"}},
		"betaflight-cli leds set-json":                     {RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Input: "JSON object, array, or object with led/leds/row/rows", Tags: []string{"leds", "write"}},
		"betaflight-cli leds set-mode-color":               {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "led_mode_color", Tags: []string{"leds", "mode_color", "write"}},
		"betaflight-cli leds set-mode-color-json":          {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "led_mode_color", Input: "JSON object or object with led_mode_color/mode_color/config", Tags: []string{"leds", "mode_color", "write"}},
		"betaflight-cli leds set-values":                   {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "led_values", Tags: []string{"leds", "write"}},
		"betaflight-cli leds set-values-json":              {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "led_values", Input: "JSON object or object with led_values/values/config", Tags: []string{"leds", "write"}},
		"betaflight-cli battery set-config-json":           {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "battery_config", Input: "JSON object or object with battery_config", Tags: []string{"battery", "write"}},
		"betaflight-cli battery set-voltage-meter":         {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "voltage_meter_config", Tags: []string{"battery", "voltage", "write"}},
		"betaflight-cli battery set-voltage-meter-json":    {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "voltage_meter_config", Input: "JSON object or object with voltage_meter_config/config/voltage_meter_configs/battery.voltage_meter_configs", Tags: []string{"battery", "voltage", "write"}},
		"betaflight-cli battery set-current-meter":         {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "current_meter_config", Tags: []string{"battery", "current", "write"}},
		"betaflight-cli battery set-current-meter-json":    {RequiresConnection: true, Operation: "write", Confirmation: "--yes", OutputRoot: "current_meter_config", Input: "JSON object or object with current_meter_config/config/current_meter_configs/battery.current_meter_configs", Tags: []string{"battery", "current", "write"}},
		"betaflight-cli save":                              dangerous,
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
		"betaflight-cli configuration diff",
		"betaflight-cli configuration export",
		"betaflight-cli configuration plan",
		"betaflight-cli configuration apply",
		"betaflight-cli system status",
		"betaflight-cli tasks status",
		"betaflight-cli telemetry",
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
		"betaflight-cli vtx list",
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
		if _, exists := registry[path]; !exists {
			registry[path] = readMSP
			if strings.Contains(path, " list") ||
				strings.Contains(path, "backup ") ||
				strings.Contains(path, "configuration export") ||
				strings.Contains(path, "configuration snapshot") ||
				strings.Contains(path, "configuration diff") {
				meta := readCLI
				registry[path] = meta
			}
		}
	}
	for _, path := range []string{
		"betaflight-cli features enable",
		"betaflight-cli features disable",
		"betaflight-cli beeper enable",
		"betaflight-cli beeper disable",
		"betaflight-cli beeper set-json",
		"betaflight-cli transponder set-provider",
		"betaflight-cli transponder set-data",
		"betaflight-cli transponder set-json",
		"betaflight-cli serial set",
		"betaflight-cli serial set-json",
		"betaflight-cli modes set",
		"betaflight-cli resources set",
		"betaflight-cli resources set-json",
		"betaflight-cli profiles select",
		"betaflight-cli profiles rate-select",
		"betaflight-cli profiles battery-select",
		"betaflight-cli profiles select-json",
		"betaflight-cli rateprofiles select",
		"betaflight-cli vtxtable set",
		"betaflight-cli leds set",
		"betaflight-cli leds set-json",
		"betaflight-cli servos set",
		"betaflight-cli servos reverse",
		"betaflight-cli servos reverse-json",
		"betaflight-cli adjustments set",
		"betaflight-cli rxrange set",
		"betaflight-cli rxrange set-json",
		"betaflight-cli receiver rxfail",
		"betaflight-cli pid set",
		"betaflight-cli rates set",
		"betaflight-cli filters set",
		"betaflight-cli receiver set",
		"betaflight-cli vtx set",
		"betaflight-cli osd set",
		"betaflight-cli gps set",
		"betaflight-cli battery set",
		"betaflight-cli failsafe set",
	} {
		if _, exists := registry[path]; !exists {
			registry[path] = capabilityMetadata{RequiresConnection: true, Operation: "plan_or_write", Confirmation: "--yes with --apply or --save", OutputRoot: "change_plan", Tags: []string{"configuration", "write"}}
		}
	}
	for _, path := range []string{
		"betaflight-cli reboot firmware",
		"betaflight-cli reboot bootloader",
		"betaflight-cli reboot bootloader-flash",
		"betaflight-cli reboot msc",
		"betaflight-cli reboot msc-utc",
		"betaflight-cli msp request",
	} {
		if _, exists := registry[path]; !exists {
			registry[path] = dangerous
		}
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
