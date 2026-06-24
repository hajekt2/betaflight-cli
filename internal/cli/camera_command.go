package cli

import (
	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) cameraCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "camera",
		Short: "Control analog camera menu keys through MSP",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "keys",
		Short: "List supported camera control keys",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{"camera_keys": bfcommands.ListCameraControlKeys()}))
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "press KEY",
		Short: "Plan or send one camera control key press",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key, err := bfcommands.ParseCameraControlKey(args[0])
			if err != nil {
				return validationFailureMessage(a, cmd, err.Error())
			}
			if !a.opts.yes {
				result := bfcommands.PlanCameraControlPress(key)
				return a.render(output.Success(commandPath(cmd), nil, map[string]any{"camera_control": result}))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.PressCameraControl(cmd.Context(), client, key)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"camera_control": result})
				if !result.Supported {
					env.Warnings = append(env.Warnings, output.Warning{Code: "unsupported_msp", Message: result.UnsupportedReason})
				}
				return env
			})
		},
	})
	return cmd
}
