package cli

import (
	"bytes"
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
	LogIndex      *int                    `json:"log_index,omitempty"`
	ExportedBytes uint32                  `json:"exported_bytes"`
	Completed     bool                    `json:"completed"`
	Inspection    *pkgblackbox.Inspection `json:"inspection,omitempty"`
	Dataflash     dataflashExport         `json:"dataflash_export"`
}

type blackboxLogList struct {
	Kind          string                     `json:"kind"`
	Offset        uint32                     `json:"offset"`
	RequestedSize *uint32                    `json:"requested_size,omitempty"`
	BytesRead     uint32                     `json:"bytes_read"`
	BlockSize     uint16                     `json:"block_size"`
	LogCount      int                        `json:"log_count"`
	Logs          []pkgblackbox.LogSummary   `json:"logs"`
	Blackbox      *bfcommands.BlackboxConfig `json:"blackbox,omitempty"`
	Storage       *bfcommands.StorageStatus  `json:"storage,omitempty"`
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
	var logIndex int
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
				export, warnings, exportEnv := a.performBlackboxExport(cmd, client, &target, config, opts, logIndex)
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
				if logIndex >= 0 {
					result.LogIndex = &logIndex
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
	cmd.Flags().IntVar(&logIndex, "log-index", -1, "export one detected onboard Blackbox log by zero-based index")
	return cmd
}

func (a *app) blackboxListCommand() *cobra.Command {
	var offset uint32
	var size uint32
	var blockSize uint16
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List onboard Blackbox logs found in Dataflash storage",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if blockSize == 0 {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "--block-size must be positive"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				config, err := bfcommands.ReadBlackboxConfig(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				data, storage, warnings, env := a.readDataflashBytes(cmd, client, &target, dataflashExportOptions{
					Offset:    offset,
					Size:      size,
					BlockSize: blockSize,
				})
				if env != nil {
					return *env
				}
				logs := []pkgblackbox.LogSummary{}
				if len(data) > 0 {
					var err error
					logs, err = pkgblackbox.ListLogs(bytes.NewReader(data))
					if err != nil {
						return output.Failure(commandPath(cmd), &target, "blackbox_parse_error", err.Error())
					}
				}
				result := blackboxLogList{
					Kind:      "blackbox_log_list",
					Offset:    offset,
					BytesRead: uint32(len(data)),
					BlockSize: blockSize,
					LogCount:  len(logs),
					Logs:      logs,
					Blackbox:  config,
					Storage:   storage,
				}
				if size != 0 {
					result.RequestedSize = &size
				}
				envValue := output.Success(commandPath(cmd), &target, map[string]any{
					"blackbox_logs": result,
				})
				addStringWarnings(&envValue, warnings)
				return envValue
			})
		},
	}
	cmd.Flags().Uint32Var(&offset, "offset", 0, "start address within onboard Blackbox storage")
	cmd.Flags().Uint32Var(&size, "size", 0, "number of bytes to scan, defaulting to used bytes from offset")
	cmd.Flags().Uint16Var(&blockSize, "block-size", defaultDataflashBlockSize, "requested MSP_DATAFLASH_READ block size")
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
	data, storage, warnings, env := a.readDataflashBytes(cmd, client, target, opts)
	if env != nil {
		return dataflashExport{}, nil, env
	}
	return a.writeDataflashExport(cmd, target, opts, storage, warnings, data)
}

func (a *app) performBlackboxExport(cmd *cobra.Command, client *connection.Client, target *output.Target, _ *bfcommands.BlackboxConfig, opts dataflashExportOptions, logIndex int) (dataflashExport, []string, *output.Envelope) {
	data, storage, warnings, env := a.readDataflashBytes(cmd, client, target, opts)
	if env != nil {
		return dataflashExport{}, nil, env
	}
	if logIndex < 0 {
		return a.writeDataflashExport(cmd, target, opts, storage, warnings, data)
	}
	logs, err := pkgblackbox.ListLogs(bytes.NewReader(data))
	if err != nil {
		failure := output.Failure(commandPath(cmd), target, "blackbox_parse_error", err.Error())
		return dataflashExport{}, nil, &failure
	}
	if logIndex >= len(logs) {
		failure := output.Failure(commandPath(cmd), target, "validation_error", fmt.Sprintf("--log-index %d is out of range for %d detected log(s)", logIndex, len(logs)))
		return dataflashExport{}, nil, &failure
	}
	selected := logs[logIndex]
	start := int(selected.OffsetBytes)
	end := start + int(selected.SizeBytes)
	exportOpts := opts
	exportOpts.Offset = opts.Offset + uint32(start)
	exportOpts.Size = uint32(selected.SizeBytes)
	return a.writeDataflashExport(cmd, target, exportOpts, storage, warnings, data[start:end])
}

func (a *app) writeDataflashExport(cmd *cobra.Command, target *output.Target, opts dataflashExportOptions, storage *bfcommands.StorageStatus, warnings []string, data []byte) (dataflashExport, []string, *output.Envelope) {
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
	if err := os.WriteFile(opts.Path, data, 0o600); err != nil {
		failure := output.Failure(commandPath(cmd), target, "file_error", err.Error())
		return dataflashExport{}, nil, &failure
	}
	export.ExportedBytes = uint32(len(data))
	if len(data) == 0 {
		export.Completed = true
		return export, warnings, nil
	}
	address := opts.Offset
	remaining := uint32(len(data))
	for remaining > 0 {
		chunkSize := opts.BlockSize
		if remaining < uint32(chunkSize) {
			chunkSize = uint16(remaining)
		}
		export.Chunks = append(export.Chunks, dataflashExportChunk{
			Address:    address,
			DataLength: chunkSize,
		})
		export.ChunkCount++
		address += uint32(chunkSize)
		remaining -= uint32(chunkSize)
	}
	export.Completed = true
	return export, warnings, nil
}

func (a *app) readDataflashBytes(cmd *cobra.Command, client *connection.Client, target *output.Target, opts dataflashExportOptions) ([]byte, *bfcommands.StorageStatus, []string, *output.Envelope) {
	storage, warnings, err := bfcommands.ReadStorageStatus(cmd.Context(), client)
	if err != nil {
		failure := a.failure(commandPath(cmd), target, err)
		return nil, nil, nil, &failure
	}
	if storage.Dataflash == nil || !storage.Dataflash.Supported {
		failure := output.Failure(commandPath(cmd), target, "unsupported_storage", "dataflash is not supported on this target")
		return nil, nil, nil, &failure
	}
	if !storage.Dataflash.Ready {
		failure := output.Failure(commandPath(cmd), target, "storage_unavailable", "dataflash is not ready")
		return nil, nil, nil, &failure
	}
	if opts.Offset > storage.Dataflash.UsedBytes {
		failure := output.Failure(commandPath(cmd), target, "validation_error", fmt.Sprintf("--offset %d exceeds used dataflash bytes %d", opts.Offset, storage.Dataflash.UsedBytes))
		return nil, nil, nil, &failure
	}
	sizeToRead := storage.Dataflash.UsedBytes - opts.Offset
	if opts.Size != 0 {
		sizeToRead = opts.Size
	}
	if sizeToRead == 0 {
		return nil, storage, warnings, nil
	}
	buffer := bytes.NewBuffer(make([]byte, 0, sizeToRead))
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
			failure := a.failure(commandPath(cmd), target, err)
			return nil, nil, nil, &failure
		}
		if chunk.Address != address {
			failure := output.Failure(commandPath(cmd), target, "protocol_error", fmt.Sprintf("dataflash chunk address 0x%08x does not match requested address 0x%08x", chunk.Address, address))
			return nil, nil, nil, &failure
		}
		if chunk.DataLength == 0 {
			break
		}
		if _, err := buffer.Write(chunk.Data); err != nil {
			failure := output.Failure(commandPath(cmd), target, "file_error", err.Error())
			return nil, nil, nil, &failure
		}
		address += uint32(chunk.DataLength)
		if remaining < uint32(chunk.DataLength) {
			remaining = 0
		} else {
			remaining -= uint32(chunk.DataLength)
		}
		if uint16(len(chunk.Data)) < requestSize {
			break
		}
	}
	return buffer.Bytes(), storage, warnings, nil
}

func inspectBlackboxReader(reader io.Reader) (pkgblackbox.Inspection, error) {
	return pkgblackbox.Inspect(reader)
}

func (a *app) inspectBlackboxFile(cmd *cobra.Command, path string) error {
	var reader io.Reader
	if path == "-" {
		reader = a.in
		if reader == nil {
			reader = os.Stdin
		}
	} else {
		file, err := os.Open(path)
		if err != nil {
			return a.render(output.Failure(commandPath(cmd), nil, "file_error", err.Error()))
		}
		defer file.Close()
		reader = file
	}
	inspection, err := inspectBlackboxReader(reader)
	if err != nil {
		return a.render(output.Failure(commandPath(cmd), nil, "blackbox_parse_error", err.Error()))
	}
	return a.render(output.Success(commandPath(cmd), nil, map[string]any{
		"file":       path,
		"inspection": inspection,
	}))
}

func (a *app) inspectOnboardBlackboxLog(cmd *cobra.Command, logIndex int, size uint32, blockSize uint16) error {
	return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
		config, err := bfcommands.ReadBlackboxConfig(cmd.Context(), client)
		if err != nil {
			return a.failure(commandPath(cmd), &target, err)
		}
		if blockSize == 0 {
			return output.Failure(commandPath(cmd), &target, "validation_error", "--block-size must be positive")
		}
		data, storage, warnings, env := a.readDataflashBytes(cmd, client, &target, dataflashExportOptions{
			Offset:    0,
			Size:      size,
			BlockSize: blockSize,
		})
		if env != nil {
			return *env
		}
		logs, err := pkgblackbox.ListLogs(bytes.NewReader(data))
		if err != nil {
			return output.Failure(commandPath(cmd), &target, "blackbox_parse_error", err.Error())
		}
		if logIndex < 0 || logIndex >= len(logs) {
			return output.Failure(commandPath(cmd), &target, "validation_error", fmt.Sprintf("--log-index %d is out of range for %d detected log(s)", logIndex, len(logs)))
		}
		selected := logs[logIndex]
		inspection := selected.Inspection
		envValue := output.Success(commandPath(cmd), &target, map[string]any{
			"log_index":  logIndex,
			"blackbox":   config,
			"storage":    storage,
			"inspection": inspection,
		})
		addStringWarnings(&envValue, warnings)
		return envValue
	})
}
