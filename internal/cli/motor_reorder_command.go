package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) motorReorderCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "motor-reorder", Short: "Inspect and change motor output reordering"}
	cmd.AddCommand(&cobra.Command{
		Use:   "show",
		Short: "Read the current motor output reordering over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				reorder, err := bfcommands.ReadMotorOutputReordering(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"motor_output_reordering": reorder,
				})
			})
		},
	})
	cmd.AddCommand(a.motorReorderSetCommand())
	return cmd
}

func (a *app) motorReorderSetCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set ORDER_JSON_FILE",
		Short: "Set motor output reordering from JSON through MSP2_SET_MOTOR_OUTPUT_REORDERING",
		Long: `Set motor output reordering from a JSON file (or "-" for stdin).

The JSON payload is {"order":[0,1,3,2]} where order[physical] names the motor
index routed to that output position. Entries beyond the array keep identity
mapping on the flight controller. The change is not persisted until a save.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			order, err := parseMotorReorderJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			// Structural validation before connecting; the range bound uses the
			// protocol maximum because the live motor count is unknown offline.
			if err := bfcommands.ValidateMotorReordering(order.Order, bfcommands.MaxSupportedMotors); err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "motor reordering changes affect motor output; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetMotorOutputReordering(cmd.Context(), client, order.Order)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"motor_reorder": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "motor_reorder",
					Command: "MSP2_SET_MOTOR_OUTPUT_REORDERING",
					Detail:  "motor output reordering changed but not saved",
				})
				return env
			})
		},
	}
}

func parseMotorReorderJSON(data []byte) (bfcommands.MotorOutputReordering, error) {
	var wrapped struct {
		MotorOutputReordering *bfcommands.MotorOutputReordering `json:"motor_output_reordering"`
		MotorReorder          *bfcommands.MotorOutputReordering `json:"motor_reorder"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.MotorOutputReordering{}, fmt.Errorf("invalid motor reorder JSON: %w", err)
	}
	if wrapped.MotorOutputReordering != nil {
		return *wrapped.MotorOutputReordering, nil
	}
	if wrapped.MotorReorder != nil {
		return *wrapped.MotorReorder, nil
	}
	var order bfcommands.MotorOutputReordering
	if err := json.Unmarshal(data, &order); err != nil {
		return bfcommands.MotorOutputReordering{}, fmt.Errorf("invalid motor reorder JSON: expected {\"order\":[...]}, %w", err)
	}
	if order.Order == nil {
		return bfcommands.MotorOutputReordering{}, fmt.Errorf("invalid motor reorder JSON: missing \"order\" array")
	}
	return order, nil
}
