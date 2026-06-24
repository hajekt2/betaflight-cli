package cli

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/batch"
	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type BuildInfo struct {
	Version string
	Commit  string
	Date    string
}

type app struct {
	build   BuildInfo
	opts    options
	out     io.Writer
	in      io.Reader
	connect connectFunc
}

type connectFunc func(context.Context, connection.Config, connection.OperationClass) (*connection.Client, connection.TargetInfo, error)

type options struct {
	port             string
	autoPort         bool
	allowUnsupported bool
	baud             int
	timeout          time.Duration
	format           string
	verbose          bool
	yes              bool
}

type exitError int

func (e exitError) Error() string {
	return fmt.Sprintf("exit %d", int(e))
}

func Execute(build BuildInfo) int {
	a := &app{build: build, out: os.Stdout, in: os.Stdin, connect: connection.Connect}
	root := a.rootCommand()
	if err := root.Execute(); err != nil {
		var ee exitError
		if errors.As(err, &ee) {
			return int(ee)
		}
		env := output.Failure(commandPath(root), nil, "usage_error", err.Error())
		_ = output.Render(os.Stdout, a.opts.format, env)
		return 1
	}
	return 0
}

func (a *app) rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:           "betaflight-cli",
		Short:         "AI-agent-first CLI for Betaflight flight controllers",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
	})
	root.PersistentFlags().StringVar(&a.opts.port, "port", "", "USB serial port, such as COM3 or /dev/tty.usbmodem01")
	root.PersistentFlags().BoolVar(&a.opts.autoPort, "auto-port", true, "auto-select a single compatible USB serial port when --port is omitted")
	root.PersistentFlags().BoolVar(&a.opts.allowUnsupported, "allow-unsupported", false, "allow domain commands outside supported firmware metadata")
	root.PersistentFlags().IntVar(&a.opts.baud, "baud", 115200, "serial baud rate")
	root.PersistentFlags().DurationVar(&a.opts.timeout, "timeout", 2*time.Second, "serial read timeout")
	root.PersistentFlags().StringVar(&a.opts.format, "format", "json", "output format: json or text")
	root.PersistentFlags().BoolVar(&a.opts.verbose, "verbose", false, "enable verbose diagnostics")
	root.PersistentFlags().BoolVar(&a.opts.yes, "yes", false, "confirm non-interactive write or dangerous action")

	root.AddCommand(a.versionCommand())
	root.AddCommand(a.capabilitiesCommand())
	root.AddCommand(a.portsCommand())
	root.AddCommand(a.doctorCommand())
	root.AddCommand(a.infoCommand())
	root.AddCommand(a.firmwareCommand())
	root.AddCommand(a.targetCommand())
	root.AddCommand(a.textCommand())
	root.AddCommand(a.cameraCommand())
	root.AddCommand(a.statusCommand())
	root.AddCommand(a.configurationCommand())
	root.AddCommand(a.systemCommand())
	root.AddCommand(a.tasksCommand())
	root.AddCommand(a.telemetryCommand())
	root.AddCommand(a.debugCommand())
	root.AddCommand(a.environmentCommand())
	root.AddCommand(a.rtcCommand())
	root.AddCommand(a.cliCommand())
	root.AddCommand(a.backupCommand())
	root.AddCommand(a.restoreCommand())
	root.AddCommand(a.presetsCommand())
	root.AddCommand(a.blackboxCommand())
	root.AddCommand(a.storageCommand())
	root.AddCommand(a.sensorsCommand())
	root.AddCommand(a.beeperCommand())
	root.AddCommand(a.transponderCommand())
	root.AddCommand(a.mixerCommand())
	root.AddCommand(a.motorsCommand())
	root.AddCommand(a.schemaCommand())
	root.AddCommand(a.settingsCommand())
	root.AddCommand(a.featuresCommand())
	root.AddCommand(a.serialCommand())
	root.AddCommand(a.modesCommand())
	root.AddCommand(a.resourcesCommand())
	root.AddCommand(a.profilesCommand())
	root.AddCommand(a.rateprofilesCommand())
	root.AddCommand(a.vtxTableCommand())
	root.AddCommand(a.ledsCommand())
	root.AddCommand(a.servosCommand())
	root.AddCommand(a.adjustmentsCommand())
	root.AddCommand(a.rxRangeCommand())
	root.AddCommand(a.settingDomainCommand(pidDomain()))
	root.AddCommand(a.settingDomainCommand(ratesDomain()))
	root.AddCommand(a.settingDomainCommand(filtersDomain()))
	root.AddCommand(a.settingDomainCommand(receiverDomain()))
	root.AddCommand(a.settingDomainCommand(vtxDomain()))
	root.AddCommand(a.settingDomainCommand(osdDomain()))
	root.AddCommand(a.settingDomainCommand(gpsDomain()))
	root.AddCommand(a.settingDomainCommand(batteryDomain()))
	root.AddCommand(a.settingDomainCommand(failsafeDomain()))
	root.AddCommand(a.batchCommand())
	root.AddCommand(a.saveCommand())
	root.AddCommand(a.rebootCommand())
	root.AddCommand(a.mspCommand())
	return root
}

func (a *app) schemaCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "schema",
		Short: "Show the stable JSON contract metadata for AI integrations",
		RunE: func(cmd *cobra.Command, _ []string) error {
			capabilities := collectCapabilityCommands(cmd.Root())
			commandCount := len(capabilities)
			operationCounts := make(map[string]int)
			outputRootSet := map[string]struct{}{}
			runnableCount := 0
			requiresConnectionCount := 0
			for _, capability := range capabilities {
				operationCounts[capability.Operation]++
				if capability.OutputRoot != "" {
					outputRootSet[capability.OutputRoot] = struct{}{}
				}
				if capability.Runnable {
					runnableCount++
				}
				if capability.RequiresConnection {
					requiresConnectionCount++
				}
			}
			operations := make([]string, 0, len(operationCounts))
			for operation := range operationCounts {
				operations = append(operations, operation)
			}
			sort.Strings(operations)
			outputRoots := make([]string, 0, len(outputRootSet))
			for root := range outputRootSet {
				outputRoots = append(outputRoots, root)
			}
			sort.Strings(outputRoots)

			coverage := buildCoverageReport(cmd.Root())
			envelopeFields := map[string]any{
				"schema_version": map[string]any{
					"type":        "string",
					"description": "envelope schema version",
				},
				"ok": map[string]any{
					"type":        "boolean",
					"description": "command success status",
				},
				"command": map[string]any{
					"type":        "string",
					"description": "canonical command that produced this envelope",
				},
				"target": map[string]any{
					"type":        "object",
					"description": "connected target metadata when a serial transport was involved",
				},
				"data": map[string]any{
					"type":        "object",
					"description": "command-specific payload",
				},
				"warnings": map[string]any{
					"type":        "array",
					"description": "non-blocking warnings",
				},
				"errors": map[string]any{
					"type":        "array",
					"description": "structured errors when ok is false",
				},
				"side_effects": map[string]any{
					"type":        "array",
					"description": "externally visible effects such as writes, reboots, or network calls",
				},
			}
			envelopeJSONSchema := map[string]any{
				"$schema":              "https://json-schema.org/draft/2020-12/schema",
				"$id":                  "https://hajekt2.github.io/betaflight-cli/schema/envelope-1.0.json",
				"title":                "betaflight-cli response envelope",
				"type":                 "object",
				"additionalProperties": false,
				"required": []string{
					"schema_version",
					"ok",
					"command",
					"data",
					"warnings",
					"errors",
					"side_effects",
				},
				"properties": map[string]any{
					"schema_version": map[string]any{
						"type":  "string",
						"const": output.SchemaVersion,
					},
					"ok": map[string]any{
						"type": "boolean",
					},
					"command": map[string]any{
						"type":      "string",
						"minLength": 1,
					},
					"target": map[string]any{
						"type":                 "object",
						"additionalProperties": false,
						"properties": map[string]any{
							"port":             map[string]any{"type": "string"},
							"auto_detected":    map[string]any{"type": "boolean"},
							"selection_reason": map[string]any{"type": "string"},
							"variant":          map[string]any{"type": "string"},
							"firmware_version": map[string]any{"type": "string"},
							"msp_api_version":  map[string]any{"type": "string"},
							"msp_protocol_version": map[string]any{
								"type":    "integer",
								"minimum": 0,
								"maximum": 255,
							},
						},
					},
					"data": map[string]any{
						"type": "object",
					},
					"warnings": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"code", "message"},
							"properties": map[string]any{
								"code":    map[string]any{"type": "string"},
								"message": map[string]any{"type": "string"},
							},
						},
					},
					"errors": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"code", "message"},
							"properties": map[string]any{
								"code":    map[string]any{"type": "string"},
								"message": map[string]any{"type": "string"},
							},
						},
					},
					"side_effects": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type":                 "object",
							"additionalProperties": false,
							"required":             []string{"type"},
							"properties": map[string]any{
								"type":    map[string]any{"type": "string"},
								"detail":  map[string]any{"type": "string"},
								"command": map[string]any{"type": "string"},
							},
						},
					},
				},
			}
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"schema": map[string]any{
					"command": "schema",
					"schema_version": map[string]any{
						"envelope":         output.SchemaVersion,
						"inversion_level":  "command-output-schema-first",
						"description":      "stable JSON envelope contract currently used for all machine-facing output",
						"supported_minors": []string{"stable"},
					},
					"envelope":             envelopeFields,
					"envelope_json_schema": envelopeJSONSchema,
					"command_contracts": map[string]any{
						"total_commands":               commandCount,
						"runnable_commands":            runnableCount,
						"requires_connection_commands": requiresConnectionCount,
						"operation_counts":             operationCounts,
						"operations":                   operations,
						"requires_connection_default":  "when command touches transport",
						"commands":                     capabilities,
					},
					"capabilities": map[string]any{
						"coverage": map[string]any{
							"implemented_domains": coverage.Summary.ImplementedCount,
							"partial_domains":     coverage.Summary.PartialCount,
							"domain_count":        coverage.Summary.DomainCount,
							"next_gaps":           coverage.NextGaps,
						},
					},
					"output_roots":  outputRoots,
					"command_count": commandCount,
				},
			}))
		},
	}
}

func (a *app) vtxConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Read VTX configuration over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				config, err := bfcommands.ReadVTXConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"vtx": config,
				})
			})
		},
	}
}

func (a *app) vtxDeviceStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "device-status",
		Short: "Read VTX device runtime status over MSP2",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				status, err := bfcommands.ReadVTXDeviceStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"vtx_device": status})
				if !status.Supported {
					env.Warnings = append(env.Warnings, output.Warning{Code: "unsupported_msp", Message: status.UnsupportedReason})
				}
				return env
			})
		},
	}
}

func (a *app) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			registry := settings.DefaultRegistry
			env := output.Success(commandPath(cmd), nil, map[string]any{
				"version":                  a.build.Version,
				"commit":                   a.build.Commit,
				"date":                     a.build.Date,
				"schema_version":           output.SchemaVersion,
				"go_target":                runtime.Version(),
				"msp_source_firmware":      msp.GeneratedMSPSourceVersion,
				"settings_source_firmware": registry.SourceFirmware,
				"settings_generated":       registry.Generated,
				"settings_count":           len(registry.Settings),
				"settings_source_files":    append([]string(nil), registry.SourceFiles...),
			})
			return a.render(env)
		},
	}
}

func (a *app) portsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "ports", Short: "Inspect serial ports"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List serial ports without opening them",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ports, err := connection.ListPorts()
			if err != nil {
				return a.render(a.failure(commandPath(cmd), nil, err))
			}
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{"ports": ports}))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "diagnose",
		Short: "Rank USB serial candidates without opening ports",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ports, err := connection.ListPorts()
			if err != nil {
				return a.render(a.failure(commandPath(cmd), nil, err))
			}
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"diagnostics": diagnosePorts(ports),
			}))
		},
	})
	return cmd
}

type portDiagnostics struct {
	Source            string                `json:"source"`
	PlatformHint      string                `json:"platform_hint"`
	Ports             []connection.PortInfo `json:"ports"`
	Candidates        []connection.PortInfo `json:"candidates"`
	CandidateCount    int                   `json:"candidate_count"`
	SingleCandidate   bool                  `json:"single_candidate"`
	RecommendedPort   string                `json:"recommended_port,omitempty"`
	RecommendedAction string                `json:"recommended_action"`
	Warnings          []string              `json:"warnings,omitempty"`
}

func diagnosePorts(ports []connection.PortInfo) portDiagnostics {
	diagnostics := portDiagnostics{
		Source:       "local serial port inventory",
		PlatformHint: platformHint(),
		Ports:        append([]connection.PortInfo(nil), ports...),
		Candidates:   []connection.PortInfo{},
		Warnings:     []string{},
	}
	for _, port := range ports {
		if port.Candidate {
			diagnostics.Candidates = append(diagnostics.Candidates, port)
		}
	}
	diagnostics.CandidateCount = len(diagnostics.Candidates)
	switch diagnostics.CandidateCount {
	case 0:
		diagnostics.RecommendedAction = "connect a Betaflight flight controller over USB, then run doctor --probe"
		diagnostics.Warnings = append(diagnostics.Warnings, "no typical Betaflight USB serial candidates were found")
	case 1:
		diagnostics.SingleCandidate = true
		diagnostics.RecommendedPort = diagnostics.Candidates[0].Name
		diagnostics.RecommendedAction = "run doctor --probe or use this port for read-only commands"
	default:
		diagnostics.RecommendedAction = "run doctor --probe or pass --port explicitly after selecting the intended flight controller"
		diagnostics.Warnings = append(diagnostics.Warnings, "multiple USB serial candidates were found")
	}
	return diagnostics
}

func (a *app) doctorCommand() *cobra.Command {
	var probe bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Read-only environment and connection diagnostics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ports, err := connection.ListPorts()
			if err != nil {
				return a.render(a.failure(commandPath(cmd), nil, err))
			}
			data := map[string]any{
				"ports":         ports,
				"probe":         probe,
				"platform_hint": platformHint(),
			}
			if probe {
				data["probe_results"] = a.probePorts(cmd.Context(), ports)
			}
			return a.render(output.Success(commandPath(cmd), nil, data))
		},
	}
	cmd.Flags().BoolVar(&probe, "probe", false, "open candidate ports and test MSP_API_VERSION")
	return cmd
}

func (a *app) infoCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "info",
		Short: "Read flight controller identity and board information",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				info, warnings := bfcommands.ReadInfo(cmd.Context(), client)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"info": info,
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	}
}

func (a *app) firmwareCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "firmware", Short: "Inspect firmware, target, and metadata support"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read firmware identity, target metadata, and support policy",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				firmware, warnings := bfcommands.ReadFirmwareStatus(cmd.Context(), client)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"firmware": firmware,
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	})
	cmd.AddCommand(a.firmwareFlashCommand())
	return cmd
}

func (a *app) firmwareFlashCommand() *cobra.Command {
	var imagePath string
	var tool string
	var toolArgs []string
	var rebootFirst bool
	var execute bool
	cmd := &cobra.Command{
		Use:   "flash",
		Short: "Flash a firmware image using an external tool (dangerous)",
		Args:  cobra.ExactArgs(0),
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := bfcommands.PlanFirmwareFlash(bfcommands.FirmwareFlashOptions{
				ImagePath:          imagePath,
				Tool:               tool,
				ToolArgs:           toolArgs,
				RebootToBootloader: rebootFirst,
			})
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			if !execute {
				return a.render(output.Success(commandPath(cmd), nil, map[string]any{
					"firmware_flash": map[string]any{
						"action":   "plan",
						"plan":     plan,
						"executed": false,
						"requires": "--yes and --execute to run",
						"note":     "this command does not touch hardware in plan mode",
					},
				}))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "firmware flash is dangerous; pass --yes"))
			}
			run := func(cmd *cobra.Command, target *output.Target, preflight *bfcommands.RebootResult) output.Envelope {
				if preflight != nil {
					result, err := bfcommands.ExecuteFirmwareFlash(cmd.Context(), *plan)
					if err != nil {
						env := output.Failure(commandPath(cmd), target, "firmware_flash_failed", err.Error())
						env.SideEffects = append(env.SideEffects, output.SideEffect{
							Type:    "firmware_reboot",
							Command: commandPath(cmd),
							Detail:  "bootloader reboot requested before flashing",
						})
						env.Data = map[string]any{
							"firmware_flash": map[string]any{
								"plan":       plan,
								"reboot":     preflight,
								"executed":   true,
								"successful": false,
							},
						}
						return env
					}
					env := output.Success(commandPath(cmd), target, map[string]any{
						"firmware_flash": map[string]any{
							"plan":       plan,
							"reboot":     preflight,
							"result":     result,
							"executed":   true,
							"successful": true,
						},
					})
					env.SideEffects = append(env.SideEffects, output.SideEffect{
						Type:    "firmware_reboot",
						Command: commandPath(cmd),
						Detail:  "bootloader reboot requested before flashing",
					})
					env.SideEffects = append(env.SideEffects, output.SideEffect{
						Type:    "firmware_flash",
						Command: strings.Join(result.Plan.EstimatedCommand, " "),
						Detail:  "external flash tool executed",
					})
					return env
				}
				result, err := bfcommands.ExecuteFirmwareFlash(cmd.Context(), *plan)
				if err != nil {
					env := output.Failure(commandPath(cmd), target, "firmware_flash_failed", err.Error())
					env.Data = map[string]any{
						"firmware_flash": map[string]any{
							"plan":       plan,
							"executed":   true,
							"successful": false,
						},
					}
					return env
				}
				env := output.Success(commandPath(cmd), target, map[string]any{
					"firmware_flash": map[string]any{
						"plan":       plan,
						"result":     result,
						"executed":   true,
						"successful": true,
					},
				})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "firmware_flash",
					Command: strings.Join(result.Plan.EstimatedCommand, " "),
					Detail:  "external flash tool executed",
				})
				return env
			}
			if rebootFirst {
				return a.withClient(cmd.Context(), commandPath(cmd), connection.Dangerous, func(client *connection.Client, target output.Target) output.Envelope {
					reboot, err := bfcommands.SendReboot(cmd.Context(), client, bfcommands.RebootBootloaderFlash)
					if err != nil {
						return a.failure(commandPath(cmd), &target, err)
					}
					return run(cmd, &target, reboot)
				})
			}
			return a.render(run(cmd, nil, nil))
		},
	}
	cmd.Flags().StringVar(&imagePath, "image", "", "firmware image path (.hex, .bin, .img)")
	cmd.Flags().StringVar(&tool, "tool", "dfu-util", "external flash tool executable")
	cmd.Flags().StringSliceVar(&toolArgs, "tool-arg", nil, "repeated args for flash tool (repeatable)")
	cmd.Flags().BoolVar(&rebootFirst, "reboot-first", false, "reboot into bootloader before executing flash tool")
	cmd.Flags().BoolVar(&execute, "execute", false, "run flash tool immediately after planning")
	return cmd
}

func (a *app) targetCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "target", Short: "Inspect target hardware and runtime inventory"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read firmware, board, system, and resource inventory",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				status, warnings := bfcommands.ReadTargetStatus(cmd.Context(), client)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"target": status,
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	})
	return cmd
}

func (a *app) textCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "text", Short: "Read and update Betaflight text metadata"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read pilot, craft, profile, build, and release text over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				text, err := bfcommands.ReadTextStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"text": text,
				})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set FIELD VALUE",
		Short: "Set writable pilot, craft, or profile text over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			field, ok := bfcommands.TextFieldByKey(args[0])
			if !ok {
				return validationFailureMessage(a, cmd, fmt.Sprintf("field must be one of: %s", writableTextFieldKeys()))
			}
			request := bfcommands.TextSetRequest{TextField: field, Value: args[1]}
			if _, err := bfcommands.EncodeTextSet(request); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "text set changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetText(cmd.Context(), client, request)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"text": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "text_set",
					Command: "MSP2_SET_TEXT",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-json FILE",
		Short: "Set writable pilot, craft, or profile text from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			request, err := parseTextSetJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if _, err := bfcommands.EncodeTextSet(request); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "text set changes configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetText(cmd.Context(), client, request)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"text": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "text_set",
					Command: "MSP2_SET_TEXT",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	})
	return cmd
}

func writableTextFieldKeys() string {
	fields := bfcommands.WritableTextFields()
	keys := make([]string, 0, len(fields))
	for _, field := range fields {
		keys = append(keys, field.Key)
	}
	return strings.Join(keys, ", ")
}

func parseTextSetJSON(data []byte) (bfcommands.TextSetRequest, error) {
	var raw struct {
		Field   *string                    `json:"field"`
		Key     *string                    `json:"key"`
		Value   *string                    `json:"value"`
		Text    *bfcommands.TextSetRequest `json:"text"`
		Set     *bfcommands.TextSetRequest `json:"set"`
		Request *bfcommands.TextSetRequest `json:"request"`
		Result  *struct {
			Request *bfcommands.TextSetRequest `json:"request"`
		} `json:"text_set"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		return bfcommands.TextSetRequest{}, err
	}
	switch {
	case raw.Text != nil:
		return completeTextSetRequest(*raw.Text)
	case raw.Set != nil:
		return completeTextSetRequest(*raw.Set)
	case raw.Request != nil:
		return completeTextSetRequest(*raw.Request)
	case raw.Result != nil && raw.Result.Request != nil:
		return completeTextSetRequest(*raw.Result.Request)
	case raw.Value != nil && raw.Field != nil:
		return textSetRequestFromKey(*raw.Field, *raw.Value)
	case raw.Value != nil && raw.Key != nil:
		return textSetRequestFromKey(*raw.Key, *raw.Value)
	}
	var request bfcommands.TextSetRequest
	if err := json.Unmarshal(data, &request); err != nil {
		return bfcommands.TextSetRequest{}, err
	}
	return completeTextSetRequest(request)
}

func completeTextSetRequest(request bfcommands.TextSetRequest) (bfcommands.TextSetRequest, error) {
	if request.Key == "" {
		return bfcommands.TextSetRequest{}, fmt.Errorf("text field key is required")
	}
	field, ok := bfcommands.TextFieldByKey(request.Key)
	if !ok {
		return bfcommands.TextSetRequest{}, fmt.Errorf("field must be one of: %s", writableTextFieldKeys())
	}
	request.TextField = field
	return request, nil
}

func textSetRequestFromKey(key string, value string) (bfcommands.TextSetRequest, error) {
	request := bfcommands.TextSetRequest{TextField: bfcommands.TextField{Key: key}, Value: value}
	return completeTextSetRequest(request)
}

func (a *app) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read compact runtime status over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				status, err := bfcommands.ReadRuntimeStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"status": status,
				})
			})
		},
	}
}

func (a *app) configurationCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "configuration", Short: "Inspect configuration state and write readiness"}
	cmd.AddCommand(a.configurationValidateCommand())
	cmd.AddCommand(a.configurationPlanCommand())
	cmd.AddCommand(a.configurationApplyCommand())
	cmd.AddCommand(a.configurationCompareCommand())
	cmd.AddCommand(a.configurationExportCommand())
	cmd.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Read and parse configuration diff output",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), "diff all")
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				doc := bfconfig.Parse(lines, settings.DefaultRegistry)
				return output.Success(commandPath(cmd), &target, map[string]any{
					"command":           "diff all",
					"lines":             lines,
					"raw":               strings.Join(lines, "\n"),
					"sections":          doc.Sections,
					"configuration":     doc,
					"inventory":         bfconfig.BuildInventory(doc),
					"raw_authoritative": true,
				})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read configuration state, profiles, arming blockers, and write guidance",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				status, warnings := bfcommands.ReadConfigurationStatus(cmd.Context(), client)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"configuration": status,
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "snapshot",
		Short: "Read dump and diff configuration snapshots for review",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				snapshot, err := bfcommands.ReadConfigurationSnapshot(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"configuration_snapshot": snapshot,
				})
			})
		},
	})
	return cmd
}

func (a *app) configurationPlanCommand() *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Validate and print a configuration change plan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			imported, err := a.readImportPlan(opts, "configuration", "configuration_text")
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: opts.includeDefaults})
			if !ok {
				return a.render(env)
			}
			return a.render(output.Success(commandPath(cmd), nil, importPlanData(imported, opts, false)))
		},
	}
	addImportFlags(cmd, &opts, false)
	return cmd
}

func (a *app) configurationApplyCommand() *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply a validated configuration plan",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if opts.includeDefaults && !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--include-defaults requires --yes because defaults nosave resets configuration before applying lines"))
			}
			imported, err := a.readImportPlan(opts, "configuration", "configuration_text")
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: opts.includeDefaults})
			if !ok {
				return a.render(env)
			}
			op := connection.Write
			if opts.includeDefaults {
				op = connection.Dangerous
			}
			return a.applyImportPlan(cmd, imported, opts, op)
		},
	}
	addImportFlags(cmd, &opts, true)
	return cmd
}

func (a *app) configurationExportCommand() *cobra.Command {
	var source string
	var redact bool
	var rawCLI bool
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Export current Betaflight configuration as JSON or raw CLI text",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cliLine := ""
			switch strings.ToLower(source) {
			case "full", "dump", "dump-all":
				cliLine = "dump all"
			case "diff", "diff-all":
				cliLine = "diff all"
			default:
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "--source must be one of full or diff"))
			}
			return a.runBackupCommand(cmd, cliLine, redact, rawCLI)
		},
	}
	cmd.Flags().StringVar(&source, "source", "full", "configuration source: full or diff")
	cmd.Flags().BoolVar(&redact, "redact", false, "redact sensitive values for sharing")
	cmd.Flags().BoolVar(&rawCLI, "raw-cli", false, "emit raw CLI text as the command data")
	return cmd
}

func (a *app) configurationCompareCommand() *cobra.Command {
	var file string
	cmd := &cobra.Command{
		Use:   "compare",
		Short: "Compare local Betaflight CLI configuration text with the current controller dump",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := a.readInput(file)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_error", err.Error()))
			}
			referenceLines, _ := configurationLinesFromInput(data)
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				compare, err := bfcommands.ReadConfigurationCompare(cmd.Context(), client, referenceLines)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"configuration_compare": compare,
				})
			})
		},
	}
	cmd.Flags().StringVar(&file, "file", "-", "reference configuration text file path, or - for stdin")
	return cmd
}

func (a *app) configurationValidateCommand() *cobra.Command {
	var file string
	var includeDefaults bool
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Validate Betaflight CLI configuration text without connecting",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := a.readInput(file)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_error", err.Error()))
			}
			lines, sourceFormat := configurationLinesFromInput(data)
			doc := bfconfig.Parse(lines, settings.DefaultRegistry)
			importData := []byte(strings.Join(lines, "\n"))
			result := map[string]any{
				"source_format":     sourceFormat,
				"line_count":        countNonEmptyInputLines(lines),
				"section_counts":    countConfigurationSections(doc.Sections),
				"inventory":         bfconfig.BuildInventory(doc),
				"unknown_count":     len(doc.Unknown),
				"unknown_settings":  countUnknownConfigurationSettings(doc.Settings),
				"document":          doc,
				"include_defaults":  includeDefaults,
				"raw_authoritative": true,
			}
			validation := map[string]any{
				"valid":  true,
				"errors": []output.Error{},
			}
			imported, err := batch.ImportCLI(importData, batch.ImportOptions{
				Kind:            "configuration_validation",
				SourceFormat:    sourceFormat,
				IncludeDefaults: includeDefaults,
			})
			if err != nil {
				validation["valid"] = false
				validation["errors"] = []output.Error{{Code: "validation_error", Message: err.Error()}}
			} else {
				result["plan"] = imported.Plan
				result["skipped_lines"] = imported.Skipped
				if env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: includeDefaults}); !ok {
					validation["valid"] = false
					validation["errors"] = env.Errors
				}
			}
			if len(doc.Unknown) > 0 || countUnknownConfigurationSettings(doc.Settings) > 0 {
				validation["review_required"] = true
			} else {
				validation["review_required"] = validation["valid"] == false
			}
			result["validation"] = validation
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"configuration_validation": result,
			}))
		},
	}
	cmd.Flags().StringVar(&file, "file", "-", "configuration text file path, or - for stdin")
	cmd.Flags().BoolVar(&includeDefaults, "include-defaults", false, "include exact defaults nosave lines in validation")
	return cmd
}

func countNonEmptyInputLines(lines []string) int {
	count := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			count++
		}
	}
	return count
}

func configurationLinesFromInput(data []byte) ([]string, string) {
	trimmed := strings.TrimSpace(string(data))
	if strings.HasPrefix(trimmed, "{") {
		var payload map[string]any
		if err := json.Unmarshal([]byte(trimmed), &payload); err == nil {
			if lines, ok := configurationLinesFromJSONMap(payload); ok {
				return lines, "json"
			}
			if nested, ok := payload["data"].(map[string]any); ok {
				if lines, ok := configurationLinesFromJSONMap(nested); ok {
					return lines, "json"
				}
			}
		}
	}
	return strings.Split(string(data), "\n"), "cli_text"
}

func configurationLinesFromJSONMap(payload map[string]any) ([]string, bool) {
	if lines, ok := payload["lines"].([]any); ok {
		out := make([]string, 0, len(lines))
		for _, line := range lines {
			text, ok := line.(string)
			if !ok {
				return nil, false
			}
			out = append(out, text)
		}
		return out, true
	}
	if raw, ok := payload["raw"].(string); ok {
		return strings.Split(raw, "\n"), true
	}
	return nil, false
}

func countConfigurationSections(sections map[string][]string) map[string]int {
	counts := map[string]int{}
	for name, lines := range sections {
		counts[name] = len(lines)
	}
	return counts
}

func countUnknownConfigurationSettings(settings []bfconfig.Setting) int {
	count := 0
	for _, setting := range settings {
		if !setting.Known {
			count++
		}
	}
	return count
}

func (a *app) tasksCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "tasks", Short: "Inspect scheduler task diagnostics"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read scheduler task statistics through Betaflight CLI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				tasks, err := bfcommands.ReadTaskStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"tasks": tasks,
				})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "task_stats_reset", Command: "tasks", Detail: "Betaflight resets max task execution statistics after printing tasks"})
				return env
			})
		},
	})
	return cmd
}

func (a *app) systemCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "system", Short: "Inspect CLI-backed system diagnostics"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read Betaflight CLI status output as structured diagnostics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				system, err := bfcommands.ReadSystemStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"system": system,
				})
			})
		},
	})
	return cmd
}

func (a *app) telemetryCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "telemetry", Short: "Read telemetry"}
	readSnapshot := func(cmd *cobra.Command, _ []string) error {
		return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
			telemetry, warnings := bfcommands.ReadTelemetry(cmd.Context(), client)
			env := output.Success(commandPath(cmd), &target, map[string]any{
				"telemetry": telemetry,
			})
			addStringWarnings(&env, warnings)
			return env
		})
	}
	cmd.RunE = readSnapshot
	cmd.AddCommand(&cobra.Command{
		Use:   "snapshot",
		Short: "Read one telemetry snapshot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return readSnapshot(cmd, nil)
		},
	})
	return cmd
}

func (a *app) debugCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "debug", Short: "Inspect live debug channels and trim values"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read MSP debug values and accelerometer trims",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				debug, warnings, err := bfcommands.ReadDebugStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"debug":    debug,
					"warnings": warnings,
				})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-accelerometer-trim PITCH ROLL",
		Short: "Set accelerometer trim over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pitch, err := parseInt16Arg("PITCH", args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			roll, err := parseInt16Arg("ROLL", args[1])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "accelerometer trim changes calibration state; pass --yes"))
			}
			trim := bfcommands.AccelerometerTrim{Pitch: pitch, Roll: roll}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetAccelerometerTrim(cmd.Context(), client, trim)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"accelerometer_trim": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "accelerometer_trim",
					Command: result.MSPName,
					Detail:  "accelerometer trim updated",
				})
				return env
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-accelerometer-trim-json FILE",
		Short: "Set accelerometer trim from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			trim, err := parseAccelerometerTrimJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "accelerometer trim changes calibration state; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetAccelerometerTrim(cmd.Context(), client, trim)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"accelerometer_trim": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "accelerometer_trim",
					Command: result.MSPName,
					Detail:  "accelerometer trim updated",
				})
				return env
			})
		},
	})
	return cmd
}

func parseAccelerometerTrimJSON(data []byte) (bfcommands.AccelerometerTrim, error) {
	var wrapped struct {
		AccelerometerTrim *bfcommands.AccelerometerTrim `json:"accelerometer_trim"`
		Trim              *bfcommands.AccelerometerTrim `json:"trim"`
		Config            *bfcommands.AccelerometerTrim `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.AccelerometerTrim{}, err
	}
	switch {
	case wrapped.AccelerometerTrim != nil:
		return *wrapped.AccelerometerTrim, nil
	case wrapped.Trim != nil:
		return *wrapped.Trim, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var trim bfcommands.AccelerometerTrim
	if err := json.Unmarshal(data, &trim); err != nil {
		return bfcommands.AccelerometerTrim{}, err
	}
	return trim, nil
}

func (a *app) environmentCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "environment", Short: "Inspect live altitude, rangefinder, and analog readings"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read altitude, rangefinder, and analog readings over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				environment, warnings, err := bfcommands.ReadEnvironmentStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"environment": environment,
					"warnings":    warnings,
				})
			})
		},
	})
	return cmd
}

func (a *app) rtcCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "rtc", Short: "Inspect flight controller real-time clock"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read RTC datetime over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				rtc, warnings, err := bfcommands.ReadRTCStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"rtc":      rtc,
					"warnings": warnings,
				})
			})
		},
	})
	var timestamp string
	var now bool
	set := &cobra.Command{
		Use:   "set",
		Short: "Set RTC datetime over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (timestamp == "") == !now {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "use exactly one of --timestamp or --now"))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "rtc set changes flight controller time; pass --yes"))
			}
			value := time.Now().UTC()
			if timestamp != "" {
				parsed, err := time.Parse(time.RFC3339Nano, timestamp)
				if err != nil {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", fmt.Sprintf("invalid --timestamp: %v", err)))
				}
				value = parsed
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRTC(cmd.Context(), client, value)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rtc": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rtc_set",
					Command: result.MSPName,
					Detail:  "flight controller RTC updated",
				})
				return env
			})
		},
	}
	set.Flags().StringVar(&timestamp, "timestamp", "", "UTC timestamp to set, formatted as RFC3339/RFC3339Nano")
	set.Flags().BoolVar(&now, "now", false, "set RTC to the local machine's current UTC time")
	cmd.AddCommand(set)
	cmd.AddCommand(&cobra.Command{
		Use:   "set-json FILE",
		Short: "Set RTC datetime from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			value, err := parseRTCSetJSON(data, time.Now)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "rtc set changes flight controller time; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRTC(cmd.Context(), client, value)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rtc": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rtc_set",
					Command: result.MSPName,
					Detail:  "flight controller RTC updated",
				})
				return env
			})
		},
	})
	return cmd
}

func parseRTCSetJSON(data []byte, now func() time.Time) (time.Time, error) {
	var wrapped struct {
		Timestamp    string `json:"timestamp"`
		TimestampUTC string `json:"timestamp_utc"`
		ISOUTC       string `json:"iso_utc"`
		Now          *bool  `json:"now"`
		RTC          *struct {
			Timestamp    string `json:"timestamp"`
			TimestampUTC string `json:"timestamp_utc"`
			ISOUTC       string `json:"iso_utc"`
			Now          *bool  `json:"now"`
		} `json:"rtc"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return time.Time{}, err
	}
	timestamp := firstNonEmpty(wrapped.Timestamp, wrapped.TimestampUTC, wrapped.ISOUTC)
	useNow := wrapped.Now != nil && *wrapped.Now
	if wrapped.RTC != nil {
		timestamp = firstNonEmpty(wrapped.RTC.Timestamp, wrapped.RTC.TimestampUTC, wrapped.RTC.ISOUTC, timestamp)
		useNow = useNow || wrapped.RTC.Now != nil && *wrapped.RTC.Now
	}
	if (timestamp == "") == !useNow {
		return time.Time{}, fmt.Errorf("use exactly one of timestamp/timestamp_utc/iso_utc or now")
	}
	if useNow {
		return now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	return parsed, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (a *app) cliCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "cli", Short: "Run Betaflight CLI commands"}
	runCLIRaw := func(cmd *cobra.Command, raw string) error {
		class := classifyCLISequence(raw)
		if class != cliReadOnly && !a.opts.yes {
			reason := fmt.Sprintf("%q is classified as writable; pass --yes or use safer domain-specific commands", raw)
			if class == cliDangerous {
				reason = fmt.Sprintf("%q is classified as dangerous; pass --yes or use safer domain-specific commands", raw)
			}
			env := output.Failure(commandPath(cmd), nil, "confirmation_required", reason)
			return a.render(env)
		}
		op := connection.ReadOnly
		if class == cliWrite {
			op = connection.Write
		}
		if class == cliDangerous {
			op = connection.Dangerous
		}
		return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
			lines, err := client.ExecCLI(cmd.Context(), raw)
			if err != nil {
				return a.failure(commandPath(cmd), &target, err)
			}
			envData := map[string]any{
				"command": raw,
				"lines":   lines,
				"raw":     strings.Join(lines, "\n"),
			}
			if isConfigurationRead(raw) {
				doc := bfconfig.Parse(lines, settings.DefaultRegistry)
				envData["configuration"] = doc
				envData["sections"] = doc.Sections
				envData["raw_authoritative"] = true
			}
			env := output.Success(commandPath(cmd), &target, envData)
			if class != cliReadOnly {
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: raw})
			}
			return env
		})
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "exec COMMAND",
		Short: "Run a framed non-interactive Betaflight CLI command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCLIRaw(cmd, args[0])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Run non-interactive `diff all` and parse response",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCLIRaw(cmd, "diff all")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "dump",
		Short: "Run non-interactive `dump all` and parse response",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCLIRaw(cmd, "dump all")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "interactive",
		Short: "Enter interactive Betaflight CLI mode using #",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "interactive CLI mode allows writes and requires --yes"))
			}
			client, _, err := connection.Connect(cmd.Context(), a.connectionConfig(), connection.Dangerous)
			if err != nil {
				return a.render(a.failure(commandPath(cmd), nil, err))
			}
			defer client.Close()
			return client.RunInteractive(os.Stdin, os.Stdout)
		},
	})
	return cmd
}

func (a *app) backupCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "backup", Short: "Create backups and diffs"}
	cmd.AddCommand(a.backupCreateCommand())
	cmd.AddCommand(a.backupDiffCommand())
	return cmd
}

func (a *app) backupCreateCommand() *cobra.Command {
	var redact bool
	var rawCLI bool
	returnCmd := &cobra.Command{
		Use:   "create",
		Short: "Create restore-oriented backup",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runBackupCommand(cmd, "dump all", redact, rawCLI)
		},
	}
	returnCmd.Flags().BoolVar(&redact, "redact", false, "redact sensitive values for sharing")
	returnCmd.Flags().BoolVar(&rawCLI, "raw-cli", false, "emit raw CLI text as the command data")
	return returnCmd
}

func (a *app) backupDiffCommand() *cobra.Command {
	var redact bool
	var rawCLI bool
	returnCmd := &cobra.Command{
		Use:   "diff",
		Short: "Create compact diff output",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runBackupCommand(cmd, "diff all", redact, rawCLI)
		},
	}
	returnCmd.Flags().BoolVar(&redact, "redact", false, "redact sensitive values for sharing")
	returnCmd.Flags().BoolVar(&rawCLI, "raw-cli", false, "emit raw CLI text as the command data")
	return returnCmd
}

func (a *app) settingsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "settings", Short: "Plan and apply setting changes"}
	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List compiled setting metadata",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.render(output.Success(commandPath(cmd), nil, settings.DefaultRegistry))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "metadata NAME",
		Short: "Show compiled metadata for one setting",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			setting, ok := settings.DefaultRegistry.Lookup(args[0])
			if !ok {
				return a.render(output.Failure(commandPath(cmd), nil, "unknown_setting", fmt.Sprintf("no compiled metadata for %q", args[0])))
			}
			return a.render(output.Success(commandPath(cmd), nil, setting))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "get NAME",
		Short: "Read a setting through Betaflight CLI get",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), "get "+name)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				data := map[string]any{
					"setting": name,
					"lines":   lines,
					"raw":     strings.Join(lines, "\n"),
				}
				if setting, ok := settings.DefaultRegistry.Lookup(name); ok {
					data["metadata"] = setting
				}
				return output.Success(commandPath(cmd), &target, data)
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "firmware-get NAME",
		Short: "Read a setting through MSP2_CLI_SETTING",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				setting, err := bfcommands.ReadFirmwareSetting(cmd.Context(), client, name)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				data := map[string]any{"firmware_setting": setting}
				if compiled, ok := settings.DefaultRegistry.Lookup(name); ok {
					data["compiled_metadata"] = compiled
				}
				env := output.Success(commandPath(cmd), &target, data)
				if !setting.Supported {
					env.Warnings = append(env.Warnings, output.Warning{Code: "unsupported_msp", Message: setting.UnsupportedReason})
				}
				return env
			})
		},
	})
	var infoOffset uint16
	infoCmd := &cobra.Command{
		Use:   "firmware-info NAME",
		Short: "Read firmware setting metadata text through MSP2_CLI_SETTING_INFO",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name := args[0]
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				info, err := bfcommands.ReadFirmwareSettingInfo(cmd.Context(), client, name, infoOffset)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				data := map[string]any{"firmware_setting_info": info}
				if compiled, ok := settings.DefaultRegistry.Lookup(name); ok {
					data["compiled_metadata"] = compiled
				}
				env := output.Success(commandPath(cmd), &target, data)
				if !info.Supported {
					env.Warnings = append(env.Warnings, output.Warning{Code: "unsupported_msp", Message: info.UnsupportedReason})
				}
				return env
			})
		},
	}
	infoCmd.Flags().Uint16Var(&infoOffset, "offset", 0, "byte offset into firmware setting info text")
	cmd.AddCommand(infoCmd)
	cmd.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Run non-interactive `diff all` and parse response",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), "diff all")
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				doc := bfconfig.Parse(lines, settings.DefaultRegistry)
				return output.Success(commandPath(cmd), &target, map[string]any{
					"command":           "diff all",
					"lines":             lines,
					"raw":               strings.Join(lines, "\n"),
					"sections":          doc.Sections,
					"configuration":     doc,
					"inventory":         bfconfig.BuildInventory(doc),
					"raw_authoritative": true,
				})
			})
		},
	})
	var apply bool
	var save bool
	set := &cobra.Command{
		Use:   "set NAME VALUE",
		Short: "Plan or apply a CLI-backed setting change",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, value := args[0], args[1]
			setting, known := settings.DefaultRegistry.Lookup(name)
			if known {
				if err := setting.Validate(value); err != nil {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
				}
			}
			line := fmt.Sprintf("set %s = %s", name, value)
			plan := map[string]any{
				"setting":   name,
				"value":     value,
				"cli_lines": []string{line},
				"applied":   false,
				"saved":     false,
			}
			if known {
				plan["metadata"] = setting
			}
			if !apply {
				env := output.Success(commandPath(cmd), nil, withChangePlanRoot(plan))
				if !known {
					env.Warnings = append(env.Warnings, output.Warning{Code: "metadata_missing", Message: fmt.Sprintf("no compiled metadata for %q; validation skipped", name)})
				}
				return a.render(env)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "apply requires --yes"))
			}
			if save && !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "--save requires --yes because it persists and usually reboots the flight controller"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), line)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				plan["applied"] = true
				plan["response_lines"] = lines
				env := output.Success(commandPath(cmd), &target, withChangePlanRoot(plan))
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: line, Detail: "configuration change applied but not saved"})
				if save {
					saveLines, err := client.ExecCLI(cmd.Context(), "save")
					if err != nil {
						addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
					}
					plan["saved"] = true
					plan["save_response_lines"] = saveLines
					env.Data = withChangePlanRoot(plan)
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
				}
				return env
			})
		},
	}
	set.Flags().BoolVar(&apply, "apply", false, "send the CLI-backed setting change")
	set.Flags().BoolVar(&save, "save", false, "persist after applying; requires --yes")
	cmd.AddCommand(set, a.settingsSetJSONCommand())
	return cmd
}

func (a *app) saveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "save",
		Short: "Persist configuration with Betaflight save",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "save persists configuration and usually reboots; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Dangerous, func(client *connection.Client, target output.Target) output.Envelope {
				lines, err := client.ExecCLI(cmd.Context(), "save")
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"save": map[string]any{
						"lines":        lines,
						"acknowledged": err == nil,
					},
				})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
				if err != nil {
					addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
				}
				return env
			})
		},
	}
}

func (a *app) rebootCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "reboot", Short: "Reboot or switch the flight controller boot mode"}
	cmd.AddCommand(a.rebootModeCommand("firmware", "Reboot into firmware", bfcommands.RebootFirmware, "firmware_reboot"))
	cmd.AddCommand(a.rebootModeCommand("bootloader", "Reboot into ROM bootloader", bfcommands.RebootBootloaderROM, "bootloader_reboot"))
	cmd.AddCommand(a.rebootModeCommand("bootloader-flash", "Reboot into flash bootloader", bfcommands.RebootBootloaderFlash, "bootloader_reboot"))
	cmd.AddCommand(a.rebootModeCommand("msc", "Reboot into USB mass-storage mode", bfcommands.RebootMSC, "msc_reboot"))
	cmd.AddCommand(a.rebootModeCommand("msc-utc", "Reboot into USB mass-storage mode using UTC", bfcommands.RebootMSCUTC, "msc_reboot"))
	return cmd
}

func (a *app) rebootModeCommand(use, short string, mode bfcommands.RebootMode, sideEffectType string) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", fmt.Sprintf("%s is a high-risk reboot action; pass --yes", commandPath(cmd))))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Dangerous, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SendReboot(cmd.Context(), client, mode)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"reboot": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: sideEffectType, Command: commandPath(cmd), Detail: "flight controller may reboot, disconnect, or change USB mode"})
				return env
			})
		},
	}
}

func (a *app) mspCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "msp", Short: "Raw MSP diagnostics"}
	var payloadHex string
	var decodePayload bool
	var directionFilter, sourceFilter, protocolFilter, nameFilter string
	listCmd := &cobra.Command{
		Use:   "list",
		Short: "List known MSP commands from the compiled registry",
		RunE: func(cmd *cobra.Command, _ []string) error {
			direction := msp.CommandDirection(strings.ToLower(strings.TrimSpace(directionFilter)))
			if direction != "" && direction != msp.DirectionRead && direction != msp.DirectionWrite && direction != msp.DirectionBoth && direction != msp.DirectionUnknown {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "invalid --direction; use read, write, both, or unknown"))
			}
			protocolFilter = strings.TrimSpace(protocolFilter)
			protocolValue := 0
			if protocolFilter != "" {
				p, err := strconv.Atoi(protocolFilter)
				if err != nil {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "invalid --protocol; expected integer"))
				}
				if p < 0 || p > 255 {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "invalid --protocol; expected 0-255"))
				}
				protocolValue = p
			}
			commands := msp.ListCommands()
			var filtered []map[string]any
			for _, command := range commands {
				if nameFilter != "" && !strings.Contains(strings.ToLower(command.Name), strings.ToLower(nameFilter)) {
					continue
				}
				if direction != "" && command.Direction != direction {
					continue
				}
				if protocolFilter != "" && command.Protocol != uint8(protocolValue) {
					continue
				}
				if sourceFilter != "" && !strings.Contains(strings.ToLower(command.Source), strings.ToLower(sourceFilter)) {
					continue
				}
				filtered = append(filtered, map[string]any{
					"name":      command.Name,
					"code":      command.Code,
					"protocol":  command.Protocol,
					"direction": command.Direction,
					"source":    command.Source,
					"line":      command.Line,
				})
			}
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"msp": map[string]any{
					"registry_version": msp.GeneratedMSPSourceVersion,
					"count":            len(filtered),
					"commands":         filtered,
				},
			}))
		},
	}
	listCmd.Flags().StringVar(&directionFilter, "direction", "", "filter by command direction: read, write, both, unknown")
	listCmd.Flags().StringVar(&protocolFilter, "protocol", "", "filter by MSP protocol version")
	listCmd.Flags().StringVar(&sourceFilter, "source", "", "filter by source file substring")
	listCmd.Flags().StringVar(&nameFilter, "name", "", "filter by command name substring")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "metadata [CODE]",
		Short: "Show compiled MSP registry metadata",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				commands := msp.ListCommands()
				directions := map[string]int{}
				protocols := map[string]int{}
				sources := map[string]int{}
				for _, command := range commands {
					directions[string(command.Direction)]++
					protocols[strconv.Itoa(int(command.Protocol))]++
					if command.Source != "" {
						sources[command.Source]++
					}
				}
				return a.render(output.Success(commandPath(cmd), nil, map[string]any{
					"msp": map[string]any{
						"registry_version": msp.GeneratedMSPSourceVersion,
						"count":            len(commands),
						"directions":       directions,
						"protocols":        protocols,
						"sources":          sources,
					},
				}))
			}
			code, meta, err := parseMSPCode(args[0])
			if err != nil {
				return err
			}
			env := output.Success(commandPath(cmd), nil, map[string]any{
				"msp": map[string]any{
					"registry_version": msp.GeneratedMSPSourceVersion,
					"code":             code,
					"code_name":        meta.Name,
					"protocol":         meta.Protocol,
					"direction":        meta.Direction,
					"source":           meta.Source,
					"line":             meta.Line,
				},
			})
			if meta.Source == "" {
				env.Warnings = append(env.Warnings, output.Warning{Code: "unknown_command", Message: "command has no known metadata and may be unsupported"})
			}
			return a.render(env)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "batch CODE...",
		Short: "Send an MSP_MULTIPLE_MSP read-only batch request",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			requests, payload, err := buildMSPBatchRequest(args)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				frame, err := client.Request(cmd.Context(), msp.MSPMultipleMsp, payload)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				responses, warnings := parseMSPBatchResponses(requests, frame.Payload, decodePayload)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"msp": map[string]any{
						"code":           frame.Code,
						"code_name":      "MSP_MULTIPLE_MSP",
						"protocol":       frame.Version,
						"version":        frame.Version,
						"direction_hint": string(msp.DirectionRead),
						"command_source": "src/main/msp/msp_protocol.h",
						"request_count":  len(requests),
						"response_count": len(responses),
						"truncated":      len(responses) < len(requests),
						"responses":      responses,
					},
				})
				env.Warnings = append(env.Warnings, warnings...)
				return env
			})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "request CODE",
		Short: "Send a raw MSP request and return raw payload hex",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, meta, err := parseMSPCode(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			payload, err := hex.DecodeString(strings.TrimPrefix(payloadHex, "0x"))
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			op := connection.ReadOnly
			unknownCommand := meta.Source == ""
			if unknownCommand || msp.IsLikelyWriteCode(code) {
				if !a.opts.yes {
					message := "raw MSP write-like command requires --yes"
					if unknownCommand {
						message = "raw MSP command without compiled metadata requires --yes"
					}
					return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", message))
				}
				op = connection.Dangerous
			}
			return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
				frame, err := client.Request(cmd.Context(), code, payload)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				mspData := map[string]any{
					"code":           frame.Code,
					"code_name":      meta.Name,
					"protocol":       frame.Version,
					"version":        frame.Version,
					"direction_hint": string(meta.Direction),
					"command_source": meta.Source,
					"command_line":   meta.Line,
					"payload_hex":    hex.EncodeToString(frame.Payload),
					"length":         len(frame.Payload),
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"msp": mspData,
				})
				if op != connection.ReadOnly {
					detail := "raw MSP write-like command sent"
					if unknownCommand {
						detail = "raw MSP command without compiled metadata sent"
					}
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "raw_msp", Detail: detail})
				}
				if decodePayload {
					decoded, supported, decodeErr := decodeMSPPayload(code, frame.Payload)
					if decodeErr != nil {
						env.Warnings = append(env.Warnings, output.Warning{
							Code:    "decode_failed",
							Message: decodeErr.Error(),
						})
					}
					if supported {
						mspData["decoded"] = decoded
						mspData["decode_supported"] = true
						mspData["decode_requested"] = true
					} else {
						mspData["decode_supported"] = false
						mspData["decode_requested"] = true
					}
				}
				return env
			})
		},
	})
	cmd.PersistentFlags().StringVar(&payloadHex, "payload-hex", "", "hex payload bytes")
	cmd.PersistentFlags().BoolVar(&decodePayload, "decode", false, "decode known payloads into structured JSON")
	return cmd
}

type mspBatchRequest struct {
	Code      uint16 `json:"code"`
	CodeName  string `json:"code_name"`
	Protocol  uint8  `json:"protocol"`
	Direction string `json:"direction"`
	Source    string `json:"source"`
	Line      int    `json:"line"`
}

func buildMSPBatchRequest(args []string) ([]mspBatchRequest, []byte, error) {
	requests := make([]mspBatchRequest, 0, len(args))
	payload := make([]byte, 0, len(args))
	for _, arg := range args {
		code, meta, err := parseMSPCode(arg)
		if err != nil {
			return nil, nil, err
		}
		if code > 0xff {
			return nil, nil, fmt.Errorf("%s uses MSP v%d code 0x%x; MSP_MULTIPLE_MSP supports only one-byte MSP v1 codes", meta.Name, meta.Protocol, code)
		}
		if meta.Source == "" {
			return nil, nil, fmt.Errorf("%s has no compiled metadata; use msp request --yes for raw diagnostics", arg)
		}
		if meta.Protocol != 1 {
			return nil, nil, fmt.Errorf("%s uses MSP protocol v%d; MSP_MULTIPLE_MSP supports MSP v1 command codes", meta.Name, meta.Protocol)
		}
		if meta.Direction != msp.DirectionRead {
			return nil, nil, fmt.Errorf("%s is %s; MSP batch accepts generated read-only commands only", meta.Name, meta.Direction)
		}
		if code == msp.MSPMultipleMsp {
			return nil, nil, errors.New("MSP_MULTIPLE_MSP cannot be nested in an MSP batch request")
		}
		requests = append(requests, mspBatchRequest{
			Code:      code,
			CodeName:  meta.Name,
			Protocol:  meta.Protocol,
			Direction: string(meta.Direction),
			Source:    meta.Source,
			Line:      meta.Line,
		})
		payload = append(payload, byte(code))
	}
	return requests, payload, nil
}

func parseMSPBatchResponses(requests []mspBatchRequest, payload []byte, decode bool) ([]map[string]any, []output.Warning) {
	responses := make([]map[string]any, 0, len(requests))
	var warnings []output.Warning
	offset := 0
	for i, request := range requests {
		if offset >= len(payload) {
			warnings = append(warnings, output.Warning{Code: "batch_response_missing", Message: fmt.Sprintf("missing MSP batch response for %s", request.CodeName)})
			break
		}
		length := int(payload[offset])
		offset++
		if offset+length > len(payload) {
			warnings = append(warnings, output.Warning{Code: "batch_response_truncated", Message: fmt.Sprintf("truncated MSP batch response for %s", request.CodeName)})
			length = len(payload) - offset
		}
		responsePayload := payload[offset : offset+length]
		offset += length
		entry := map[string]any{
			"index":          i,
			"code":           request.Code,
			"code_name":      request.CodeName,
			"protocol":       request.Protocol,
			"direction_hint": request.Direction,
			"command_source": request.Source,
			"command_line":   request.Line,
			"payload_hex":    hex.EncodeToString(responsePayload),
			"length":         len(responsePayload),
		}
		if decode {
			decoded, supported, decodeErr := decodeMSPPayload(request.Code, responsePayload)
			if decodeErr != nil {
				warnings = append(warnings, output.Warning{Code: "decode_failed", Message: fmt.Sprintf("%s: %s", request.CodeName, decodeErr.Error())})
			}
			entry["decode_requested"] = true
			entry["decode_supported"] = supported
			if supported {
				entry["decoded"] = decoded
			}
		}
		responses = append(responses, entry)
	}
	if offset < len(payload) {
		warnings = append(warnings, output.Warning{Code: "batch_extra_bytes", Message: fmt.Sprintf("%d trailing bytes after MSP batch responses", len(payload)-offset)})
	}
	return responses, warnings
}

func parseMSPCode(raw string) (uint16, msp.CommandMeta, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return 0, msp.CommandMeta{}, errors.New("MSP code is required")
	}
	if command, ok := msp.LookupCommandByName(trimmed); ok {
		return command.Code, command, nil
	}

	upper := strings.ToUpper(trimmed)
	if command, ok := msp.LookupCommandByName(upper); ok {
		return command.Code, command, nil
	}
	if !strings.HasPrefix(upper, "MSP_") {
		if command, ok := msp.LookupCommandByName("MSP_" + upper); ok {
			return command.Code, command, nil
		}
	}

	code64, err := strconv.ParseUint(trimmed, 0, 16)
	if err != nil {
		return 0, msp.CommandMeta{}, fmt.Errorf("invalid MSP code %q", trimmed)
	}
	code := uint16(code64)
	command, ok := msp.LookupCommand(code)
	if ok {
		return code, command, nil
	}
	return code, msp.CommandMeta{Name: fmt.Sprintf("MSP_%d", code)}, nil
}

func (a *app) runBackupCommand(cmd *cobra.Command, cliLine string, redact bool, rawCLI bool) error {
	return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
		lines, err := client.ExecCLI(cmd.Context(), cliLine)
		if err != nil {
			return a.failure(commandPath(cmd), &target, err)
		}
		raw := strings.Join(lines, "\n")
		redactedLines := lines
		var redacted []string
		if redact {
			redactedLines, redacted = redactLines(lines)
			raw = strings.Join(redactedLines, "\n")
		}
		if rawCLI {
			return output.Success(commandPath(cmd), &target, raw)
		}
		doc := bfconfig.Parse(redactedLines, settings.DefaultRegistry)
		env := output.Success(commandPath(cmd), &target, map[string]any{
			"command":           cliLine,
			"raw":               raw,
			"lines":             redactedLines,
			"sections":          doc.Sections,
			"inventory":         bfconfig.BuildInventory(doc),
			"configuration":     doc,
			"redacted":          redact,
			"redacted_classes":  redacted,
			"raw_authoritative": true,
		})
		return env
	})
}

func (a *app) withClient(ctx context.Context, command string, op connection.OperationClass, fn func(*connection.Client, output.Target) output.Envelope) error {
	connect := a.connect
	if connect == nil {
		connect = connection.Connect
	}
	client, targetInfo, err := connect(ctx, a.connectionConfig(), op)
	target := toOutputTarget(targetInfo)
	if err != nil {
		return a.render(a.failure(command, &target, err))
	}
	defer client.Close()
	return a.render(fn(client, target))
}

func (a *app) connectionConfig() connection.Config {
	return connection.Config{
		Port:             a.opts.port,
		Baud:             a.opts.baud,
		Timeout:          a.opts.timeout,
		AutoPort:         a.opts.autoPort,
		AllowUnsupported: a.opts.allowUnsupported,
	}
}

func (a *app) render(env output.Envelope) error {
	out := a.out
	if out == nil {
		out = os.Stdout
	}
	if err := output.Render(out, a.opts.format, env); err != nil {
		return err
	}
	if !env.OK {
		return exitError(1)
	}
	return nil
}

func (a *app) failure(command string, target *output.Target, err error) output.Envelope {
	var coded *connection.CodedError
	if errors.As(err, &coded) {
		env := output.Failure(command, target, coded.Code, coded.Message)
		if len(coded.Candidates) > 0 {
			env.Data = map[string]any{"candidates": coded.Candidates}
		}
		return env
	}
	return output.Failure(command, target, "error", err.Error())
}

func (a *app) probePorts(ctx context.Context, ports []connection.PortInfo) []map[string]any {
	var results []map[string]any
	for _, p := range ports {
		if !p.Candidate {
			results = append(results, map[string]any{
				"port":      p.Name,
				"candidate": false,
				"ok":        false,
				"reason":    p.Reason,
			})
			continue
		}
		cfg := a.connectionConfig()
		cfg.Port = p.Name
		cfg.AllowUnsupported = true
		client, target, err := connection.Connect(ctx, cfg, connection.ReadOnly)
		if client != nil {
			_ = client.Close()
		}
		if err != nil {
			results = append(results, map[string]any{
				"port":      p.Name,
				"candidate": true,
				"ok":        false,
				"error":     err.Error(),
			})
			continue
		}
		results = append(results, map[string]any{
			"port":      p.Name,
			"candidate": true,
			"ok":        true,
			"target":    target,
			"support":   probeSupport(target),
			"metadata":  probeMetadata(),
		})
	}
	return results
}

func probeSupport(target connection.TargetInfo) bfcommands.FirmwareSupport {
	return bfcommands.EvaluateFirmwareSupport(target.Variant, target.FirmwareVersion, target.MSPAPIVersion)
}

func probeMetadata() map[string]any {
	registry := settings.DefaultRegistry
	return map[string]any{
		"settings_source_firmware": registry.SourceFirmware,
		"settings_generated":       registry.Generated,
		"settings_count":           len(registry.Settings),
		"settings_source_files":    append([]string(nil), registry.SourceFiles...),
	}
}

func toOutputTarget(target connection.TargetInfo) output.Target {
	return output.Target{
		Port:            target.Port,
		AutoDetected:    target.AutoDetected,
		SelectionReason: target.SelectionReason,
		Variant:         target.Variant,
		FirmwareVersion: target.FirmwareVersion,
		MSPAPIVersion:   target.MSPAPIVersion,
		MSPProtocol:     target.MSPProtocol,
	}
}

func addStringWarnings(env *output.Envelope, warnings []string) {
	for _, warning := range warnings {
		env.Warnings = append(env.Warnings, output.Warning{
			Code:    "partial_result",
			Message: warning,
		})
	}
}

func commandPath(cmd *cobra.Command) string {
	if cmd == nil {
		return "betaflight-cli"
	}
	return cmd.CommandPath()
}

func platformHint() string {
	return "USB serial only. On macOS prefer /dev/cu.* ports; on Linux check dialout/uucp permissions; on Windows use COM ports."
}

func parseCLISections(lines []string) map[string][]string {
	return bfconfig.Parse(lines, settings.DefaultRegistry).Sections
}

func redactLines(lines []string) ([]string, []string) {
	var out []string
	classes := map[string]bool{}
	sensitive := []string{"pilot_name", "craft_name", "callsign", "bind", "signature", "uid", "name"}
	for _, line := range lines {
		redacted := line
		lower := strings.ToLower(line)
		for _, key := range sensitive {
			if strings.Contains(lower, key) {
				redacted = redactValue(line)
				classes[key] = true
				break
			}
		}
		out = append(out, redacted)
	}
	var classList []string
	for class := range classes {
		classList = append(classList, class)
	}
	return out, classList
}

func redactValue(line string) string {
	if i := strings.Index(line, "="); i >= 0 {
		return strings.TrimSpace(line[:i+1]) + " REDACTED"
	}
	fields := strings.Fields(line)
	if len(fields) <= 1 {
		return line
	}
	return fields[0] + " REDACTED"
}
