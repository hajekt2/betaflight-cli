package cli

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/batch"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

type importCommandOptions struct {
	file            string
	includeDefaults bool
	save            bool
}

func (a *app) restoreCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "restore", Short: "Plan and apply restore files from Betaflight CLI output"}
	cmd.AddCommand(a.importPlanCommand("plan", "Validate and print a restore plan without connecting", "restore", "restore_text"))
	cmd.AddCommand(a.importApplyCommand("apply", "Apply a validated restore plan", "restore", "restore_text"))
	return cmd
}

func (a *app) presetsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "presets", Short: "Plan and apply local Betaflight preset files"}
	cmd.AddCommand(a.importPlanCommand("plan", "Validate and print a local preset plan without connecting", "preset", "preset_text"))
	cmd.AddCommand(a.importApplyCommand("apply", "Apply a local preset plan", "preset", "preset_text"))
	cmd.AddCommand(a.presetsFetchCommand())
	return cmd
}

type presetFetchOptions struct {
	file            string
	includeDefaults bool
	apply           bool
	save            bool
	httpTimeout     time.Duration
}

type presetFetchMetadata struct {
	URL          string `json:"url"`
	HTTPStatus   int    `json:"http_status"`
	ContentType  string `json:"content_type"`
	ContentLength int64 `json:"content_length"`
	ChecksumSHA256 string `json:"checksum_sha256"`
	FetchedAt    string `json:"fetched_at"`
	ETag         string `json:"etag,omitempty"`
	LastModified string `json:"last_modified,omitempty"`
}

func (a *app) presetsFetchCommand() *cobra.Command {
	var opts presetFetchOptions
	cmd := &cobra.Command{
		Use:   "fetch URL",
		Short: "Fetch a remote preset, validate it, and optionally apply",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts.file = args[0]
			imported, metadata, err := a.fetchImportPlan(cmd.Context(), opts, "preset")
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: opts.includeDefaults})
			if !ok {
				return a.render(env)
			}
			if !opts.apply {
				return a.render(output.Success(commandPath(cmd), nil, map[string]any{
					"source":  metadata,
					"plan":    importPlanData(imported, importCommandOptions{includeDefaults: opts.includeDefaults, save: opts.save}, false),
					"kind":    imported.Plan.Kind,
					"applied": false,
				}))
			}
			if !a.opts.yes {
				return a.render(importApplyConfirmationFailure(cmd, opts.save))
			}
			op := connection.Write
			if opts.includeDefaults {
				op = connection.Dangerous
			}
			return a.applyImportPlan(cmd, imported, importCommandOptions{
				includeDefaults: opts.includeDefaults,
				save:            opts.save,
			}, op)
		},
	}
	cmd.Flags().BoolVar(&opts.includeDefaults, "include-defaults", false, "include exact defaults nosave lines from the remote preset; apply requires --yes")
	cmd.Flags().BoolVar(&opts.apply, "apply", false, "apply fetched preset lines instead of previewing plan")
	cmd.Flags().BoolVar(&opts.save, "save", false, "persist after applying fetched lines; requires --yes")
	cmd.Flags().DurationVar(&opts.httpTimeout, "timeout", 15*time.Second, "HTTP request timeout for remote preset fetch")
	return cmd
}

func (a *app) importPlanCommand(use, short, kind, sourceFormat string) *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			imported, err := a.readImportPlan(opts, kind, sourceFormat)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: opts.includeDefaults})
			if !ok {
				return a.render(env)
			}
			return a.render(output.Success(commandPath(cmd), nil, importPlanData(imported, opts, false)))
		},
	}
	addImportFlags(cmd, &opts, false)
	return cmd
}

func (a *app) importApplyCommand(use, short, kind, sourceFormat string) *cobra.Command {
	var opts importCommandOptions
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.opts.yes {
				return a.render(importApplyConfirmationFailure(cmd, opts.save))
			}
			imported, err := a.readImportPlan(opts, kind, sourceFormat)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			env, ok := a.validateChangePlan(cmd, imported.Plan, planValidationOptions{allowDefaultsNoSave: opts.includeDefaults})
			if !ok {
				return a.render(env)
			}
			op := connection.Write
			if opts.includeDefaults {
				op = connection.Dangerous
			}
			return a.applyImportPlan(cmd, imported, opts, op)
		},
	}
	addImportFlags(cmd, &opts, true)
	return cmd
}

func addImportFlags(cmd *cobra.Command, opts *importCommandOptions, apply bool) {
	cmd.Flags().StringVar(&opts.file, "file", "-", "restore or preset file path, or - for stdin")
	cmd.Flags().BoolVar(&opts.includeDefaults, "include-defaults", false, "include exact defaults nosave lines from the import file; apply requires --yes")
	if apply {
		cmd.Flags().BoolVar(&opts.save, "save", false, "persist after applying; requires --yes")
	}
}

func (a *app) readImportPlan(opts importCommandOptions, kind, sourceFormat string) (batch.ImportResult, error) {
	data, err := a.readInput(opts.file)
	if err != nil {
		return batch.ImportResult{}, err
	}
	return batch.ImportCLI(data, batch.ImportOptions{
		Kind:            kind,
		SourceFormat:    sourceFormat,
		IncludeDefaults: opts.includeDefaults,
	})
}

func (a *app) fetchImportPlan(ctx context.Context, opts presetFetchOptions, kind string) (batch.ImportResult, presetFetchMetadata, error) {
	rawURL := strings.TrimSpace(opts.file)
	parsed, err := url.ParseRequestURI(rawURL)
	if err != nil {
		return batch.ImportResult{}, presetFetchMetadata{URL: rawURL}, err
	}
	if parsed.Scheme != "https" && parsed.Scheme != "http" {
		return batch.ImportResult{}, presetFetchMetadata{URL: rawURL}, fmt.Errorf("unsupported preset URL scheme %q", parsed.Scheme)
	}

	client := &http.Client{Timeout: opts.httpTimeout}
	request, err := http.NewRequestWithContext(ctx, "GET", parsed.String(), nil)
	if err != nil {
		return batch.ImportResult{}, presetFetchMetadata{URL: rawURL}, err
	}
	request.Header.Set("User-Agent", "betaflight-cli-presets-fetch")
	response, err := client.Do(request)
	if err != nil {
		return batch.ImportResult{}, presetFetchMetadata{URL: rawURL}, err
	}
	defer response.Body.Close()
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return batch.ImportResult{}, presetFetchMetadata{URL: rawURL, HTTPStatus: response.StatusCode}, fmt.Errorf("preset fetch failed with status %d", response.StatusCode)
	}
	data, err := io.ReadAll(response.Body)
	if err != nil {
		return batch.ImportResult{}, presetFetchMetadata{URL: rawURL}, err
	}
	hash := sha256.Sum256(data)
	imported, err := batch.ImportCLI(data, batch.ImportOptions{
		Kind:            kind,
		SourceFormat:    "preset_text",
		IncludeDefaults: opts.includeDefaults,
	})
	if err != nil {
		return batch.ImportResult{}, presetFetchMetadata{}, err
	}
	return imported, presetFetchMetadata{
		URL:           rawURL,
		HTTPStatus:    response.StatusCode,
		ContentType:   response.Header.Get("Content-Type"),
		ContentLength: int64(len(data)),
		ChecksumSHA256: hex.EncodeToString(hash[:]),
		FetchedAt:     time.Now().UTC().Format(time.RFC3339),
		ETag:          response.Header.Get("ETag"),
		LastModified:  response.Header.Get("Last-Modified"),
	}, nil
}

func (a *app) applyImportPlan(cmd *cobra.Command, imported batch.ImportResult, opts importCommandOptions, op connection.OperationClass) error {
	if !a.opts.yes {
		return a.render(importApplyConfirmationFailure(cmd, opts.save))
	}
	if opts.save {
		op = connection.Dangerous
	}
	data := importPlanData(imported, opts, true)
	return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
		responses := map[string][]string{}
		for _, line := range imported.Plan.CLILines {
			responseLines, err := client.ExecCLI(cmd.Context(), line)
			if err != nil {
				return a.failure(commandPath(cmd), &target, err)
			}
			responses[line] = responseLines
		}
		data["response_lines"] = responses
		env := output.Success(commandPath(cmd), &target, data)
		for _, line := range imported.Plan.CLILines {
			env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: line, Detail: "configuration change applied but not saved"})
		}
		if opts.save {
			saveLines, err := client.ExecCLI(cmd.Context(), "save")
			if err != nil {
				addStringWarnings(&env, []string{fmt.Sprintf("save command may have rebooted or disconnected before response completed: %v", err)})
			}
			data["saved"] = true
			data["save_response_lines"] = saveLines
			env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "save", Command: "save", Detail: "configuration persisted; flight controller may reboot or disconnect"})
		}
		return env
	})
}

func importApplyConfirmationFailure(cmd *cobra.Command, save bool) output.Envelope {
	message := "apply requires --yes"
	if save {
		message = "--save requires --yes because it persists and usually reboots the flight controller"
	}
	return output.Failure(commandPath(cmd), nil, "confirmation_required", message)
}

func importPlanData(imported batch.ImportResult, opts importCommandOptions, applied bool) map[string]any {
	return map[string]any{
		"schema_version":    imported.Plan.SchemaVersion,
		"kind":              imported.Plan.Kind,
		"cli_lines":         imported.Plan.CLILines,
		"source_format":     imported.Plan.SourceFormat,
		"include_defaults":  opts.includeDefaults,
		"save_requested":    opts.save,
		"applied":           applied,
		"saved":             false,
		"skipped_lines":     imported.Skipped,
		"raw_authoritative": true,
	}
}
