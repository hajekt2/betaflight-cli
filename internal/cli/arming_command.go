package cli

import (
	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) armingCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "arming",
		Short: "Inspect and control the remote arming lock",
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Show arming-disable flags and whether the remote arming lock is engaged",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				state, err := bfcommands.ReadArmingLockState(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{"arming_lock": state})
			})
		},
	})
	lock := func(lock bool) *cobra.Command {
		action := "unlock"
		summary := "release the remote arming lock so the FC can arm again"
		if lock {
			action = "lock"
			summary = "engage the remote arming lock (sets the MSP arming-disable flag)"
		}
		return &cobra.Command{
			Use:   action,
			Short: summary,
			RunE: func(cmd *cobra.Command, _ []string) error {
				if !a.opts.yes {
					return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "arming lock changes the arming gate at runtime; pass --yes"))
				}
				return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
					result, err := bfcommands.SetArmingLock(cmd.Context(), client, lock)
					if err != nil {
						return a.failure(commandPath(cmd), &target, err)
					}
					env := output.Success(commandPath(cmd), &target, map[string]any{"arming_lock": result})
					env.SideEffects = append(env.SideEffects, output.SideEffect{
						Type:    "arming_lock",
						Command: "MSP_SET_ARMING_DISABLED",
						Detail:  "arming gate changed at runtime; not persisted to configuration",
					})
					return env
				})
			},
		}
	}
	cmd.AddCommand(lock(true))
	cmd.AddCommand(lock(false))
	return cmd
}
