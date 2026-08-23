package cli

import (
	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) rxBindCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rx-bind",
		Short: "Trigger receiver binding over MSP",
	}
	cmd.AddCommand(a.rxBindStartCommand())
	return cmd
}

func (a *app) rxBindStartCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Send bind pulses to a SPI or serial RX after explicit confirmation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			warnings := []string{
				"remove propellers and keep the aircraft away from people before triggering bind",
				"stay within a few meters of the receiver while bind pulses are sent",
				"binding only works when the RX supports it (SPI RX or CRSF-SRXL2 with software binding)",
			}
			if !a.opts.yes {
				env := output.Failure(commandPath(cmd), nil, "confirmation_required", "rx bind puts the receiver into bind mode; remove props and stay near the receiver; pass --yes")
				addStringWarnings(&env, warnings)
				return a.render(env)
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Dangerous, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.StartRxBind(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rx_bind": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rx_bind",
					Command: "MSP2_BETAFLIGHT_BIND",
					Detail:  "bind pulses triggered on supported SPI/serial RX",
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	}
}
