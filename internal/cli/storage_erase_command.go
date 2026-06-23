package cli

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

type storageErasePlan struct {
	Kind                  string                `json:"kind"`
	Applied               bool                  `json:"applied"`
	Dangerous             bool                  `json:"dangerous"`
	Command               string                `json:"command"`
	RequiredConfirmations []string              `json:"required_confirmations"`
	Warnings              []string              `json:"warnings,omitempty"`
	Before                *storageEraseSnapshot `json:"before,omitempty"`
	After                 *storageEraseSnapshot `json:"after,omitempty"`
	Audit                 *storageEraseAudit    `json:"audit,omitempty"`
}

type storageEraseSnapshot struct {
	hazardousReadOnlyEvidence
	Dataflash *bfcommands.DataflashSummary `json:"dataflash,omitempty"`
	SDCard    *bfcommands.SDCardSummary    `json:"sdcard,omitempty"`
}

type storageEraseAudit struct {
	hazardousActionAudit
	FreedBytes uint32 `json:"freed_bytes,omitempty"`
}

func (a *app) storageEraseCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "erase",
		Short: "Erase Dataflash storage after explicit confirmation",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan := storageErasePlan{
				Kind:      "storage_erase",
				Dangerous: true,
				Command:   "MSP_DATAFLASH_ERASE",
				RequiredConfirmations: []string{
					"--yes",
				},
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "dataflash erase is destructive; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Dangerous, func(client *connection.Client, target output.Target) output.Envelope {
				before, warnings, err := readStorageEraseSnapshot(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, fmt.Errorf("storage preflight unavailable: %w", err))
				}
				plan.Before = before
				plan.Warnings = append(plan.Warnings, warnings...)
				result, err := bfcommands.EraseDataflash(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				after, postWarnings, err := readStorageEraseSnapshot(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, fmt.Errorf("storage post-erase status unavailable: %w", err))
				}
				plan.After = after
				plan.Warnings = append(plan.Warnings, postWarnings...)
				plan.Applied = true
				plan.Audit = buildStorageEraseAudit(plan)
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"storage_erase": plan,
					"erase_result":  result,
				})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "dataflash_erase",
					Command: plan.Command,
					Detail:  "dataflash erase request sent",
				})
				addStringWarnings(&env, plan.Warnings)
				return env
			})
		},
	}
}

func readStorageEraseSnapshot(ctx context.Context, client *connection.Client) (*storageEraseSnapshot, []string, error) {
	status, warnings, err := bfcommands.ReadStorageStatus(ctx, client)
	if err != nil {
		return nil, nil, err
	}
	return &storageEraseSnapshot{
		hazardousReadOnlyEvidence: hazardousReadOnlyEvidence{
			Source:   "storage status",
			ReadOnly: true,
		},
		Dataflash: status.Dataflash,
		SDCard:    status.SDCard,
	}, warnings, nil
}

func buildStorageEraseAudit(plan storageErasePlan) *storageEraseAudit {
	freed := uint32(0)
	if plan.Before != nil && plan.Before.Dataflash != nil && plan.After != nil && plan.After.Dataflash != nil {
		if plan.Before.Dataflash.UsedBytes > plan.After.Dataflash.UsedBytes {
			freed = plan.Before.Dataflash.UsedBytes - plan.After.Dataflash.UsedBytes
		}
	}
	return &storageEraseAudit{
		hazardousActionAudit: hazardousActionAudit{
			Source:              "storage erase",
			ReadOnlyEvidence:    true,
			RequestedDurationMS: 0,
			ElapsedDurationMS:   0,
			StartCommand:        plan.Command,
			StopCommand:         "",
			Confirmations:       append([]string(nil), plan.RequiredConfirmations...),
			SafetyPassed:        true,
			PreflightCaptured:   plan.Before != nil,
			PostActionCaptured:  plan.After != nil,
			StopAttempted:       false,
			StopSucceeded:       false,
			WarningMessages:     append([]string(nil), plan.Warnings...),
		},
		FreedBytes: freed,
	}
}
