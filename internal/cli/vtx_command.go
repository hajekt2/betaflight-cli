package cli

import (
	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

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
