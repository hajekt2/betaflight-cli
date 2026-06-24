package cli

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
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
