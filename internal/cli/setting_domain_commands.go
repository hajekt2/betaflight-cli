package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
)

type settingDomain struct {
	use     string
	short   string
	matches func(settings.Metadata) bool
}

func (a *app) settingDomainCommand(domain settingDomain) *cobra.Command {
	cmd := &cobra.Command{Use: domain.use, Short: domain.short}
	cmd.AddCommand(a.configListCommand("list", "List current "+domain.use+" settings", func(doc bfconfig.Document) any {
		return map[string]any{
			"settings": currentDomainSettings(doc, domain.matches),
			"metadata": settings.DefaultRegistry.Filter(domain.matches),
		}
	}))
	var flags changeFlags
	set := &cobra.Command{
		Use:   "set NAME VALUE",
		Short: "Plan or set one " + domain.use + " setting",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, value := args[0], args[1]
			metadata, ok := settings.DefaultRegistry.Lookup(name)
			if !ok || !domain.matches(metadata) {
				return a.render(output.Failure(commandPath(cmd), nil, "unknown_setting", fmt.Sprintf("%q is not a known %s setting", name, domain.use)))
			}
			if err := metadata.Validate(value); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.planOrApplyCLI(cmd, []string{fmt.Sprintf("set %s = %s", metadata.Name, value)}, domain.use, flags)
		},
	}
	addChangeFlags(set, &flags)
	cmd.AddCommand(set)
	if domain.use == "vtx" {
		cmd.AddCommand(a.vtxConfigCommand())
	}
	if domain.use == "receiver" {
		cmd.AddCommand(a.receiverStatusCommand())
	}
	if domain.use == "gps" {
		cmd.AddCommand(a.gpsStatusCommand())
	}
	if domain.use == "osd" {
		cmd.AddCommand(a.osdStatusCommand())
	}
	return cmd
}

func (a *app) receiverStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read receiver configuration and live channels over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				receiver, warnings, err := bfcommands.ReadReceiverStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"receiver": receiver,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) gpsStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read GPS configuration, position, and Rescue state over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				gps, warnings, err := bfcommands.ReadGPSStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"gps":      gps,
					"warnings": warnings,
				})
			})
		},
	}
}

func (a *app) osdStatusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read OSD configuration, canvas, and active warning text over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				osd, warnings, err := bfcommands.ReadOSDStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"osd":      osd,
					"warnings": warnings,
				})
			})
		},
	}
}

func currentDomainSettings(doc bfconfig.Document, matches func(settings.Metadata) bool) []bfconfig.Setting {
	out := []bfconfig.Setting{}
	for _, setting := range doc.Settings {
		metadata, ok := settings.DefaultRegistry.Lookup(setting.Name)
		if ok && matches(metadata) {
			out = append(out, setting)
		}
	}
	return out
}

func pidDomain() settingDomain {
	return settingDomain{use: "pid", short: "Inspect and change PID settings", matches: func(s settings.Metadata) bool {
		name := s.Name
		if strings.HasPrefix(name, "p_") || strings.HasPrefix(name, "i_") || strings.HasPrefix(name, "d_") || strings.HasPrefix(name, "f_") {
			return true
		}
		return strings.Contains(s.PG, "PID") && !rateSettingName(name) && !filterSettingName(name)
	}}
}

func ratesDomain() settingDomain {
	return settingDomain{use: "rates", short: "Inspect and change rate settings", matches: func(s settings.Metadata) bool {
		return s.Scope == "rateprofile" || rateSettingName(s.Name)
	}}
}

func filtersDomain() settingDomain {
	return settingDomain{use: "filters", short: "Inspect and change filter settings", matches: func(s settings.Metadata) bool {
		return filterSettingName(s.Name)
	}}
}

func receiverDomain() settingDomain {
	return settingDomain{use: "receiver", short: "Inspect and change receiver settings", matches: func(s settings.Metadata) bool {
		name := s.Name
		return strings.Contains(s.PG, "RX") || strings.HasPrefix(name, "serialrx_") || strings.Contains(name, "deadband") || strings.Contains(name, "rssi")
	}}
}

func vtxDomain() settingDomain {
	return settingDomain{use: "vtx", short: "Inspect and change VTX settings", matches: func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "VTX") || strings.HasPrefix(s.Name, "vtx_")
	}}
}

func osdDomain() settingDomain {
	return settingDomain{use: "osd", short: "Inspect and change OSD settings", matches: func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "OSD") || strings.HasPrefix(s.Name, "osd_")
	}}
}

func gpsDomain() settingDomain {
	return settingDomain{use: "gps", short: "Inspect and change GPS settings", matches: func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "GPS") || strings.HasPrefix(s.Name, "gps_")
	}}
}

func failsafeDomain() settingDomain {
	return settingDomain{use: "failsafe", short: "Inspect and change failsafe settings", matches: func(s settings.Metadata) bool {
		return strings.Contains(s.PG, "FAILSAFE") || strings.HasPrefix(s.Name, "failsafe_")
	}}
}

func rateSettingName(name string) bool {
	return strings.Contains(name, "rate") || strings.Contains(name, "expo") || strings.HasPrefix(name, "thr_") || strings.HasPrefix(name, "tpa_")
}

func filterSettingName(name string) bool {
	return strings.Contains(name, "lpf") ||
		strings.Contains(name, "notch") ||
		strings.Contains(name, "filter") ||
		strings.HasPrefix(name, "dyn_notch") ||
		strings.HasPrefix(name, "rpm_filter")
}
