package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

const defaultDataflashBlockSize = 4096

type dataflashExport struct {
	Kind          string                    `json:"kind"`
	Path          string                    `json:"path"`
	Offset        uint32                    `json:"offset"`
	RequestedSize *uint32                   `json:"requested_size,omitempty"`
	ExportedBytes uint32                    `json:"exported_bytes"`
	BlockSize     uint16                    `json:"block_size"`
	ChunkCount    int                       `json:"chunk_count"`
	Completed     bool                      `json:"completed"`
	Truncated     bool                      `json:"truncated"`
	Storage       *bfcommands.StorageStatus `json:"storage,omitempty"`
	Chunks        []dataflashExportChunk    `json:"chunks,omitempty"`
}

type dataflashExportChunk struct {
	Address    uint32 `json:"address"`
	DataLength uint16 `json:"data_length"`
}

func (a *app) storageExportCommand() *cobra.Command {
	var offset uint32
	var size uint32
	var blockSize uint16
	var force bool
	cmd := &cobra.Command{
		Use:   "export FILE",
		Short: "Export Dataflash contents to a local file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			if blockSize == 0 {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "--block-size must be positive"))
			}
			if !force {
				if _, err := os.Stat(path); err == nil {
					return a.render(output.Failure(commandPath(cmd), nil, "file_exists", "output file exists; pass --force to overwrite"))
				} else if !os.IsNotExist(err) {
					return a.render(output.Failure(commandPath(cmd), nil, "file_error", err.Error()))
				}
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				storage, warnings, err := bfcommands.ReadStorageStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				if storage.Dataflash == nil || !storage.Dataflash.Supported {
					return output.Failure(commandPath(cmd), &target, "unsupported_storage", "dataflash is not supported on this target")
				}
				if !storage.Dataflash.Ready {
					return output.Failure(commandPath(cmd), &target, "storage_unavailable", "dataflash is not ready")
				}
				if offset > storage.Dataflash.UsedBytes {
					return output.Failure(commandPath(cmd), &target, "validation_error", fmt.Sprintf("--offset %d exceeds used dataflash bytes %d", offset, storage.Dataflash.UsedBytes))
				}
				sizeToRead := storage.Dataflash.UsedBytes - offset
				if size != 0 {
					sizeToRead = size
				}
				if sizeToRead == 0 {
					export := dataflashExport{
						Kind:          "dataflash_export",
						Path:          path,
						Offset:        offset,
						ExportedBytes: 0,
						BlockSize:     blockSize,
						ChunkCount:    0,
						Completed:     true,
						Storage:       storage,
					}
					if size != 0 {
						export.RequestedSize = &size
					}
					env := output.Success(commandPath(cmd), &target, map[string]any{"dataflash_export": export})
					addStringWarnings(&env, warnings)
					return env
				}
				file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
				if err != nil {
					return output.Failure(commandPath(cmd), &target, "file_error", err.Error())
				}
				defer file.Close()
				export := dataflashExport{
					Kind:      "dataflash_export",
					Path:      path,
					Offset:    offset,
					BlockSize: blockSize,
					Storage:   storage,
					Chunks:    []dataflashExportChunk{},
				}
				if size != 0 {
					export.RequestedSize = &size
				}
				address := offset
				remaining := sizeToRead
				for remaining > 0 {
					requestSize := blockSize
					if remaining < uint32(requestSize) {
						requestSize = uint16(remaining)
					}
					chunk, err := bfcommands.ReadDataflashChunk(cmd.Context(), client, bfcommands.DataflashReadRequest{
						Address:          address,
						BlockSize:        requestSize,
						AllowCompression: false,
					})
					if err != nil {
						return a.failure(commandPath(cmd), &target, err)
					}
					if chunk.Address != address {
						return output.Failure(commandPath(cmd), &target, "protocol_error", fmt.Sprintf("dataflash chunk address 0x%08x does not match requested address 0x%08x", chunk.Address, address))
					}
					if chunk.DataLength == 0 {
						export.Truncated = true
						break
					}
					if _, err := file.Write(chunk.Data); err != nil {
						return output.Failure(commandPath(cmd), &target, "file_error", err.Error())
					}
					export.Chunks = append(export.Chunks, dataflashExportChunk{
						Address:    chunk.Address,
						DataLength: chunk.DataLength,
					})
					export.ExportedBytes += uint32(chunk.DataLength)
					export.ChunkCount++
					address += uint32(chunk.DataLength)
					if remaining < uint32(chunk.DataLength) {
						remaining = 0
					} else {
						remaining -= uint32(chunk.DataLength)
					}
					if uint16(len(chunk.Data)) < requestSize {
						export.Truncated = remaining > 0
						break
					}
				}
				export.Completed = !export.Truncated && export.ExportedBytes == sizeToRead
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"dataflash_export": export,
				})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "file_write",
					Command: path,
					Detail:  fmt.Sprintf("wrote %d bytes of dataflash data", export.ExportedBytes),
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	}
	cmd.Flags().Uint32Var(&offset, "offset", 0, "start address within Dataflash")
	cmd.Flags().Uint32Var(&size, "size", 0, "number of bytes to export, defaulting to used bytes from offset")
	cmd.Flags().Uint16Var(&blockSize, "block-size", defaultDataflashBlockSize, "requested MSP_DATAFLASH_READ block size")
	cmd.Flags().BoolVar(&force, "force", false, "overwrite the output file if it already exists")
	return cmd
}
