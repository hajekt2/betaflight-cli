package cli

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
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
	root.PersistentFlags().StringVar(&a.opts.port, "port", "", "USB serial port, such as COM3 or /dev/tty.usbmodem01")
	root.PersistentFlags().BoolVar(&a.opts.autoPort, "auto-port", false, "explicitly allow automatic port selection for writes")
	root.PersistentFlags().BoolVar(&a.opts.allowUnsupported, "allow-unsupported", false, "allow domain commands outside supported firmware metadata")
	root.PersistentFlags().IntVar(&a.opts.baud, "baud", 115200, "serial baud rate")
	root.PersistentFlags().DurationVar(&a.opts.timeout, "timeout", 2*time.Second, "serial read timeout")
	root.PersistentFlags().StringVar(&a.opts.format, "format", "json", "output format: json or text")
	root.PersistentFlags().BoolVar(&a.opts.yes, "yes", false, "confirm non-interactive write or dangerous action")

	root.AddCommand(a.versionCommand())
	root.AddCommand(a.capabilitiesCommand())
	root.AddCommand(a.portsCommand())
	root.AddCommand(a.doctorCommand())
	root.AddCommand(a.infoCommand())
	root.AddCommand(a.firmwareCommand())
	root.AddCommand(a.targetCommand())
	root.AddCommand(a.textCommand())
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

func (a *app) blackboxCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "blackbox", Short: "Inspect and analyze Blackbox logs"}
	cmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Read Blackbox configuration over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				config, err := bfcommands.ReadBlackboxConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"blackbox": config,
				})
			})
		},
	})
	inspectCmd := &cobra.Command{
		Use:   "inspect FILE",
		Short: "Inspect a Blackbox log file without connecting to hardware",
		Args: func(cmd *cobra.Command, args []string) error {
			logIndex, err := cmd.Flags().GetInt("log-index")
			if err != nil {
				return err
			}
			switch {
			case logIndex >= 0 && len(args) != 0:
				return fmt.Errorf("use either FILE or --log-index, not both")
			case logIndex < 0 && len(args) != 1:
				return fmt.Errorf("requires FILE or --log-index")
			case logIndex >= 0 && len(args) == 0:
				return nil
			default:
				return cobra.ExactArgs(1)(cmd, args)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			logIndex, err := cmd.Flags().GetInt("log-index")
			if err != nil {
				return err
			}
			size, err := cmd.Flags().GetUint32("size")
			if err != nil {
				return err
			}
			blockSize, err := cmd.Flags().GetUint16("block-size")
			if err != nil {
				return err
			}
			if logIndex >= 0 {
				return a.inspectOnboardBlackboxLog(cmd, logIndex, size, blockSize)
			}
			path := args[0]
			return a.inspectBlackboxFile(cmd, path)
		},
	}
	inspectCmd.Flags().Int("log-index", -1, "inspect one detected onboard Blackbox log by zero-based index")
	inspectCmd.Flags().Uint32("size", 256*1024, "number of onboard bytes to scan when using --log-index")
	inspectCmd.Flags().Uint16("block-size", 4096, "requested MSP_DATAFLASH_READ block size when using --log-index")
	cmd.AddCommand(inspectCmd)
	cmd.AddCommand(a.blackboxListCommand())
	cmd.AddCommand(a.blackboxExportCommand())
	return cmd
}

func (a *app) storageCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "storage", Short: "Inspect Dataflash and SD card storage"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read Dataflash and SD card summaries over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				storage, warnings, err := bfcommands.ReadStorageStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"storage":  storage,
					"warnings": warnings,
				})
			})
		},
	})
	cmd.AddCommand(a.storageExportCommand())
	cmd.AddCommand(a.storageEraseCommand())
	return cmd
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

func (a *app) sensorsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "sensors", Short: "Inspect sensor configuration and live sensor state"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read sensor configuration and raw IMU data over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				sensors, warnings, err := bfcommands.ReadSensorStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"sensors":  sensors,
					"warnings": warnings,
				})
			})
		},
	})
	return cmd
}

func (a *app) beeperCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "beeper", Short: "Inspect beeper and DShot beacon configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Read beeper configuration over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				beeper, err := bfcommands.ReadBeeperConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"beeper": beeper,
				})
			})
		},
	})
	var enableFlags changeFlags
	enable := &cobra.Command{
		Use:   "enable NAME",
		Short: "Plan or enable one beeper condition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := bfcommands.ValidateBeeperModeName(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"beeper " + mode}, "beeper", enableFlags)
		},
	}
	addChangeFlags(enable, &enableFlags)
	cmd.AddCommand(enable)
	var disableFlags changeFlags
	disable := &cobra.Command{
		Use:   "disable NAME",
		Short: "Plan or disable one beeper condition",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := bfcommands.ValidateBeeperModeName(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{"beeper -" + mode}, "beeper", disableFlags)
		},
	}
	addChangeFlags(disable, &disableFlags)
	cmd.AddCommand(disable)
	return cmd
}

func (a *app) transponderCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "transponder", Short: "Inspect IR transponder configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "config",
		Short: "Read transponder provider and code data over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				transponder, err := bfcommands.ReadTransponderConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"transponder": transponder,
				})
			})
		},
	})
	var setProviderFlags changeFlags
	setProvider := &cobra.Command{
		Use:   "set-provider NAME",
		Short: "Plan or set transponder provider",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			provider, err := bfcommands.ValidateTransponderProviderName(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{fmt.Sprintf("set transponder_provider = %s", provider)}, "transponder", setProviderFlags)
		},
	}
	addChangeFlags(setProvider, &setProviderFlags)
	cmd.AddCommand(setProvider)
	var setDataFlags changeFlags
	setData := &cobra.Command{
		Use:   "set-data BYTES",
		Short: "Plan or set transponder data bytes",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := bfcommands.ValidateTransponderData(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{bfcommands.FormatTransponderData(data)}, "transponder", setDataFlags)
		},
	}
	addChangeFlags(setData, &setDataFlags)
	cmd.AddCommand(setData)
	return cmd
}

func (a *app) mixerCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "mixer", Short: "Inspect mixer and motor direction configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read mixer mode and motor direction over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				mixer, err := bfcommands.ReadMixerStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"mixer": mixer,
				})
			})
		},
	})
	return cmd
}

func (a *app) motorsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "motors", Short: "Inspect motor configuration and live outputs"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read motor configuration, outputs, and telemetry over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				motors, warnings, err := bfcommands.ReadMotorStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"motors":   motors,
					"warnings": warnings,
				})
			})
		},
	})
	cmd.AddCommand(a.motorTestPlanCommand())
	cmd.AddCommand(a.motorTestApplyCommand())
	return cmd
}

func (a *app) versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print build version",
		RunE: func(cmd *cobra.Command, _ []string) error {
			env := output.Success(commandPath(cmd), nil, map[string]string{
				"version":        a.build.Version,
				"commit":         a.build.Commit,
				"date":           a.build.Date,
				"schema_version": output.SchemaVersion,
				"go_target":      "latest stable",
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
				env := output.Success(commandPath(cmd), &target, info)
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
	cmd := &cobra.Command{Use: "text", Short: "Read Betaflight text metadata"}
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
	return cmd
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
					"configuration":      doc,
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
			env := output.Success(commandPath(cmd), &target, telemetry)
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
	return cmd
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
	return cmd
}

func (a *app) cliCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "cli", Short: "Run Betaflight CLI commands"}
	runCLIRaw := func(cmd *cobra.Command, raw string) error {
		class := classifyCLI(raw)
		if class != cliReadOnly && !a.opts.yes {
			env := output.Failure(commandPath(cmd), nil, "confirmation_required", fmt.Sprintf("%q is not classified as read-only; pass --yes or use a safer domain command", raw))
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
					"configuration":      doc,
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
				env := output.Success(commandPath(cmd), nil, plan)
				if !known {
					env.Warnings = append(env.Warnings, output.Warning{Code: "metadata_missing", Message: fmt.Sprintf("no compiled metadata for %q; validation skipped", name)})
				}
				return a.render(env)
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
				env := output.Success(commandPath(cmd), &target, plan)
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: line, Detail: "configuration change applied but not saved"})
				if save {
					saveLines, err := client.ExecCLI(cmd.Context(), "save")
					if err != nil {
						addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
					}
					plan["saved"] = true
					plan["save_response_lines"] = saveLines
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
				}
				return env
			})
		},
	}
	set.Flags().BoolVar(&apply, "apply", false, "send the CLI-backed setting change")
	set.Flags().BoolVar(&save, "save", false, "persist after applying; requires --yes")
	cmd.AddCommand(set)
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
				env := output.Success(commandPath(cmd), &target, map[string]any{"lines": lines})
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
				"registry_version": msp.GeneratedMSPSourceVersion,
				"count":           len(filtered),
				"commands":        filtered,
			}))
		},
	}
	listCmd.Flags().StringVar(&directionFilter, "direction", "", "filter by command direction: read, write, both, unknown")
	listCmd.Flags().StringVar(&protocolFilter, "protocol", "", "filter by MSP protocol version")
	listCmd.Flags().StringVar(&sourceFilter, "source", "", "filter by source file substring")
	listCmd.Flags().StringVar(&nameFilter, "name", "", "filter by command name substring")
	cmd.AddCommand(listCmd)

	cmd.AddCommand(&cobra.Command{
		Use:   "metadata CODE",
		Short: "Show compiled metadata for one MSP command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, meta, err := parseMSPCode(args[0])
			if err != nil {
				return err
			}
			env := output.Success(commandPath(cmd), nil, map[string]any{
				"registry_version": msp.GeneratedMSPSourceVersion,
				"code":             code,
				"code_name":        meta.Name,
				"protocol":         meta.Protocol,
				"direction":        meta.Direction,
				"source":           meta.Source,
				"line":             meta.Line,
			})
			if meta.Source == "" {
				env.Warnings = append(env.Warnings, output.Warning{Code: "unknown_command", Message: "command has no known metadata and may be unsupported"})
			}
			return a.render(env)
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "request CODE",
		Short: "Send a raw MSP request and return raw payload hex",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			code, meta, err := parseMSPCode(args[0])
			if err != nil {
				return err
			}
			payload, err := hex.DecodeString(strings.TrimPrefix(payloadHex, "0x"))
			if err != nil {
				return err
			}
			op := connection.ReadOnly
			if msp.IsLikelyWriteCode(code) {
				if !a.opts.yes {
					return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "raw MSP write-like command requires --yes"))
				}
				op = connection.Dangerous
			}
			return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
				frame, err := client.Request(cmd.Context(), code, payload)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"code":             frame.Code,
					"code_name":        meta.Name,
					"protocol":         frame.Version,
					"version":          frame.Version,
					"direction_hint":   string(meta.Direction),
					"command_source":   meta.Source,
					"command_line":     meta.Line,
					"payload_hex":      hex.EncodeToString(frame.Payload),
					"length":           len(frame.Payload),
				})
				if op != connection.ReadOnly {
					env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "raw_msp", Detail: "raw MSP write-like command sent"})
				}
				return env
			})
		},
	})
	cmd.PersistentFlags().StringVar(&payloadHex, "payload-hex", "", "hex payload bytes")
	return cmd
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

type cliClass int

const (
	cliReadOnly cliClass = iota
	cliWrite
	cliDangerous
)

func classifyCLI(command string) cliClass {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(command)))
	if len(fields) == 0 {
		return cliReadOnly
	}
	first := fields[0]
	switch first {
	case "save", "defaults", "motor", "motors", "dshotprog", "bl", "dfu", "msc", "exit", "reboot", "erase", "beeper":
		return cliDangerous
	case "diff", "dump", "get", "version", "status", "help", "tasks":
		return cliReadOnly
	case "resource":
		if len(fields) == 1 || fields[1] == "show" || fields[1] == "list" {
			return cliReadOnly
		}
		return cliWrite
	case "set", "feature", "serial", "aux", "profile", "rateprofile", "battery_profile", "vtxtable", "mode_color", "color", "led", "servo", "smix", "adjrange", "rxrange", "rxfail":
		return cliWrite
	default:
		return cliWrite
	}
}

func isConfigurationRead(command string) bool {
	fields := strings.Fields(strings.ToLower(strings.TrimSpace(command)))
	if len(fields) == 0 {
		return false
	}
	return fields[0] == "dump" || fields[0] == "diff"
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
