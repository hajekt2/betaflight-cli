package cli

import (
	"fmt"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

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
