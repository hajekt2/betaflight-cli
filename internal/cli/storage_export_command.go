package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	pkgblackbox "github.com/hajekt2/betaflight-cli/pkg/blackbox"
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

type dataflashExportOptions struct {
	Path      string
	Offset    uint32
	Size      uint32
	BlockSize uint16
	Force     bool
}

type blackboxExport struct {
	Kind          string                  `json:"kind"`
	Path          string                  `json:"path"`
	ExportedBytes uint32                  `json:"exported_bytes"`
	Completed     bool                    `json:"completed"`
	Inspection    *pkgblackbox.Inspection `json:"inspection,omitempty"`
	Dataflash     dataflashExport         `json:"dataflash_export"`
}

type exportOptionError struct {
	code    string
	message string
}

func (e exportOptionError) Error() string {
	return e.message
}

func (a *app) storageExportCommand() *cobra.Command {
	opts := dataflashExportOptions{
		BlockSize: defaultDataflashBlockSize,
	}
	cmd := &cobra.Command{
		Use:   "export FILE",
		Short: "Export Dataflash contents to a local file",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Path = args[0]
			if err := validateDataflashExportOptions(opts); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, exportOptionErrorCode(err), err.Error()))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				export, warnings, exportEnv := a.performDataflashExport(cmd, client, &target, opts)
				if exportEnv != nil {
					return *exportEnv
				}
				envValue := output.Success(commandPath(cmd), &target, map[string]any{
					"dataflash_export": export,
				})
				envValue.SideEffects = append(envValue.SideEffects, output.SideEffect{
					Type:    "file_write",
					Command: opts.Path,
					Detail:  fmt.Sprintf("wrote %d bytes of dataflash data", export.ExportedBytes),
				})
				addStringWarnings(&envValue, warnings)
				return envValue
			})
		},
	}
	cmd.Flags().Uint32Var(&opts.Offset, "offset", 0, "start address within Dataflash")
	cmd.Flags().Uint32Var(&opts.Size, "size", 0, "number of bytes to export, defaulting to used bytes from offset")
	cmd.Flags().Uint16Var(&opts.BlockSize, "block-size", defaultDataflashBlockSize, "requested MSP_DATAFLASH_READ block size")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite the output file if it already exists")
	return cmd
}

func (a *app) blackboxExportCommand() *cobra.Command {
	opts := dataflashExportOptions{
		BlockSize: defaultDataflashBlockSize,
	}
	cmd := &cobra.Command{
		Use:   "export FILE",
		Short: "Export onboard Blackbox data to a local file and inspect it",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.Path = args[0]
			if err := validateDataflashExportOptions(opts); err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, exportOptionErrorCode(err), err.Error()))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				config, err := bfcommands.ReadBlackboxConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				export, warnings, exportEnv := a.performDataflashExport(cmd, client, &target, opts)
				if exportEnv != nil {
					return *exportEnv
				}
				file, err := os.Open(opts.Path)
				if err != nil {
					return output.Failure(commandPath(cmd), &target, "file_error", err.Error())
				}
				defer file.Close()
				inspection, err := inspectBlackboxReader(file)
				if err != nil {
					return output.Failure(commandPath(cmd), &target, "blackbox_parse_error", err.Error())
				}
				result := blackboxExport{
					Kind:          "blackbox_export",
					Path:          opts.Path,
					ExportedBytes: export.ExportedBytes,
					Completed:     export.Completed,
					Inspection:    &inspection,
					Dataflash:     export,
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"blackbox_export": result,
					"blackbox":        config,
					"inspection":      inspection,
				})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "file_write",
					Command: opts.Path,
					Detail:  fmt.Sprintf("wrote %d bytes of onboard blackbox data", export.ExportedBytes),
				})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	}
	cmd.Flags().Uint32Var(&opts.Offset, "offset", 0, "start address within onboard Blackbox storage")
	cmd.Flags().Uint32Var(&opts.Size, "size", 0, "number of bytes to export, defaulting to used bytes from offset")
	cmd.Flags().Uint16Var(&opts.BlockSize, "block-size", defaultDataflashBlockSize, "requested MSP_DATAFLASH_READ block size")
	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite the output file if it already exists")
	return cmd
}

func validateDataflashExportOptions(opts dataflashExportOptions) error {
	if opts.BlockSize == 0 {
		return exportOptionError{code: "validation_error", message: "--block-size must be positive"}
	}
	if !opts.Force {
		if _, err := os.Stat(opts.Path); err == nil {
			return exportOptionError{code: "file_exists", message: "output file exists; pass --force to overwrite"}
		} else if !os.IsNotExist(err) {
			return exportOptionError{code: "file_error", message: err.Error()}
		}
	}
	return nil
}

func exportOptionErrorCode(err error) string {
	var coded exportOptionError
	if ok := AsExportOptionError(err, &coded); ok {
		return coded.code
	}
	return "validation_error"
}

func AsExportOptionError(err error, target *exportOptionError) bool {
	coded, ok := err.(exportOptionError)
	if !ok {
		return false
	}
	*target = coded
	return true
}

func (a *app) performDataflashExport(cmd *cobra.Command, client *connection.Client, target *output.Target, opts dataflashExportOptions) (dataflashExport, []string, *output.Envelope) {
	storage, warnings, err := bfcommands.ReadStorageStatus(cmd.Context(), client)
	if err != nil {
		env := a.failure(commandPath(cmd), target, err)
		return dataflashExport{}, nil, &env
	}
	if storage.Dataflash == nil || !storage.Dataflash.Supported {
		env := output.Failure(commandPath(cmd), target, "unsupported_storage", "dataflash is not supported on this target")
		return dataflashExport{}, nil, &env
	}
	if !storage.Dataflash.Ready {
		env := output.Failure(commandPath(cmd), target, "storage_unavailable", "dataflash is not ready")
		return dataflashExport{}, nil, &env
	}
	if opts.Offset > storage.Dataflash.UsedBytes {
		env := output.Failure(commandPath(cmd), target, "validation_error", fmt.Sprintf("--offset %d exceeds used dataflash bytes %d", opts.Offset, storage.Dataflash.UsedBytes))
		return dataflashExport{}, nil, &env
	}
	sizeToRead := storage.Dataflash.UsedBytes - opts.Offset
	if opts.Size != 0 {
		sizeToRead = opts.Size
	}
	export := dataflashExport{
		Kind:      "dataflash_export",
		Path:      opts.Path,
		Offset:    opts.Offset,
		BlockSize: opts.BlockSize,
		Storage:   storage,
		Chunks:    []dataflashExportChunk{},
	}
	if opts.Size != 0 {
		export.RequestedSize = &opts.Size
	}
	if sizeToRead == 0 {
		export.Completed = true
		return export, warnings, nil
	}
	file, err := os.OpenFile(opts.Path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		env := output.Failure(commandPath(cmd), target, "file_error", err.Error())
		return dataflashExport{}, nil, &env
	}
	defer file.Close()
	address := opts.Offset
	remaining := sizeToRead
	for remaining > 0 {
		requestSize := opts.BlockSize
		if remaining < uint32(requestSize) {
			requestSize = uint16(remaining)
		}
		chunk, err := bfcommands.ReadDataflashChunk(cmd.Context(), client, bfcommands.DataflashReadRequest{
			Address:          address,
			BlockSize:        requestSize,
			AllowCompression: false,
		})
		if err != nil {
			env := a.failure(commandPath(cmd), target, err)
			return dataflashExport{}, nil, &env
		}
		if chunk.Address != address {
			env := output.Failure(commandPath(cmd), target, "protocol_error", fmt.Sprintf("dataflash chunk address 0x%08x does not match requested address 0x%08x", chunk.Address, address))
			return dataflashExport{}, nil, &env
		}
		if chunk.DataLength == 0 {
			export.Truncated = true
			break
		}
		if _, err := file.Write(chunk.Data); err != nil {
			env := output.Failure(commandPath(cmd), target, "file_error", err.Error())
			return dataflashExport{}, nil, &env
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
	return export, warnings, nil
}

func inspectBlackboxReader(reader io.Reader) (pkgblackbox.Inspection, error) {
	return pkgblackbox.Inspect(reader)
}
