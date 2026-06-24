package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

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
	cmd.AddCommand(&cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set Blackbox configuration from JSON through MSP_SET_BLACKBOX_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseBlackboxConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "Blackbox config changes require --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetBlackboxConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"blackbox_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "blackbox_config",
					Command: "MSP_SET_BLACKBOX_CONFIG",
					Detail:  "Blackbox config changed but not saved",
				})
				return env
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

func parseBlackboxConfigJSON(data []byte) (bfcommands.BlackboxConfig, error) {
	var wrapped struct {
		Blackbox       *bfcommands.BlackboxConfig `json:"blackbox"`
		BlackboxConfig *bfcommands.BlackboxConfig `json:"blackbox_config"`
		Config         *bfcommands.BlackboxConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.BlackboxConfig{}, err
	}
	if wrapped.BlackboxConfig != nil {
		return *wrapped.BlackboxConfig, nil
	}
	if wrapped.Blackbox != nil {
		return *wrapped.Blackbox, nil
	}
	if wrapped.Config != nil {
		return *wrapped.Config, nil
	}
	var config bfcommands.BlackboxConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.BlackboxConfig{}, err
	}
	return config, nil
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
