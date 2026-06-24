package cli

import (
	"context"
	"runtime"
	"strings"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
	"github.com/spf13/cobra"
)

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
