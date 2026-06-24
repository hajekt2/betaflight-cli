package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/bfconfig"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/hajekt2/betaflight-cli/internal/settings"
	"github.com/spf13/cobra"
)

func (a *app) cliCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "cli", Short: "Run Betaflight CLI commands"}
	runCLIRaw := func(cmd *cobra.Command, raw string) error {
		class := classifyCLISequence(raw)
		if class != cliReadOnly && !a.opts.yes {
			reason := fmt.Sprintf("%q is classified as writable; pass --yes or use safer domain-specific commands", raw)
			if class == cliDangerous {
				reason = fmt.Sprintf("%q is classified as dangerous; pass --yes or use safer domain-specific commands", raw)
			}
			env := output.Failure(commandPath(cmd), nil, "confirmation_required", reason)
			return a.render(env)
		}
		op := connection.ReadOnly
		if class == cliWrite {
			op = connection.Write
		}
		if class == cliDangerous {
			op = connection.Dangerous
		}
		return a.withClient(cmd.Context(), commandPath(cmd), op, func(client *connection.Client, target output.Target) output.Envelope {
			lines, err := client.ExecCLI(cmd.Context(), raw)
			if err != nil {
				return a.failure(commandPath(cmd), &target, err)
			}
			envData := map[string]any{
				"command": raw,
				"lines":   lines,
				"raw":     strings.Join(lines, "\n"),
			}
			if isConfigurationRead(raw) {
				doc := bfconfig.Parse(lines, settings.DefaultRegistry)
				envData["configuration"] = doc
				envData["sections"] = doc.Sections
				envData["raw_authoritative"] = true
			}
			env := output.Success(commandPath(cmd), &target, envData)
			if class != cliReadOnly {
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "cli_command", Command: raw})
			}
			return env
		})
	}
	cmd.AddCommand(&cobra.Command{
		Use:   "exec COMMAND",
		Short: "Run a framed non-interactive Betaflight CLI command",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runCLIRaw(cmd, args[0])
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "diff",
		Short: "Run non-interactive `diff all` and parse response",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCLIRaw(cmd, "diff all")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "dump",
		Short: "Run non-interactive `dump all` and parse response",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runCLIRaw(cmd, "dump all")
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "interactive",
		Short: "Enter interactive Betaflight CLI mode using #",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "interactive CLI mode allows writes and requires --yes"))
			}
			client, _, err := connection.Connect(cmd.Context(), a.connectionConfig(), connection.Dangerous)
			if err != nil {
				return a.render(a.failure(commandPath(cmd), nil, err))
			}
			defer client.Close()
			return client.RunInteractive(os.Stdin, os.Stdout)
		},
	})
	return cmd
}

func (a *app) backupCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "backup", Short: "Create backups and diffs"}
	cmd.AddCommand(a.backupCreateCommand())
	cmd.AddCommand(a.backupDiffCommand())
	return cmd
}

func (a *app) backupCreateCommand() *cobra.Command {
	var redact bool
	var rawCLI bool
	returnCmd := &cobra.Command{
		Use:   "create",
		Short: "Create restore-oriented backup",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runBackupCommand(cmd, "dump all", redact, rawCLI)
		},
	}
	returnCmd.Flags().BoolVar(&redact, "redact", false, "redact sensitive values for sharing")
	returnCmd.Flags().BoolVar(&rawCLI, "raw-cli", false, "emit raw CLI text as the command data")
	return returnCmd
}

func (a *app) backupDiffCommand() *cobra.Command {
	var redact bool
	var rawCLI bool
	returnCmd := &cobra.Command{
		Use:   "diff",
		Short: "Create compact diff output",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.runBackupCommand(cmd, "diff all", redact, rawCLI)
		},
	}
	returnCmd.Flags().BoolVar(&redact, "redact", false, "redact sensitive values for sharing")
	returnCmd.Flags().BoolVar(&rawCLI, "raw-cli", false, "emit raw CLI text as the command data")
	return returnCmd
}

func (a *app) runBackupCommand(cmd *cobra.Command, cliLine string, redact bool, rawCLI bool) error {
	return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
		lines, err := client.ExecCLI(cmd.Context(), cliLine)
		if err != nil {
			return a.failure(commandPath(cmd), &target, err)
		}
		raw := strings.Join(lines, "\n")
		redactedLines := lines
		var redacted []string
		if redact {
			redactedLines, redacted = redactLines(lines)
			raw = strings.Join(redactedLines, "\n")
		}
		if rawCLI {
			return output.Success(commandPath(cmd), &target, raw)
		}
		doc := bfconfig.Parse(redactedLines, settings.DefaultRegistry)
		env := output.Success(commandPath(cmd), &target, map[string]any{
			"command":           cliLine,
			"raw":               raw,
			"lines":             redactedLines,
			"sections":          doc.Sections,
			"inventory":         bfconfig.BuildInventory(doc),
			"configuration":     doc,
			"redacted":          redact,
			"redacted_classes":  redacted,
			"raw_authoritative": true,
		})
		return env
	})
}

func parseCLISections(lines []string) map[string][]string {
	return bfconfig.Parse(lines, settings.DefaultRegistry).Sections
}

func redactLines(lines []string) ([]string, []string) {
	var out []string
	classes := map[string]bool{}
	sensitive := []string{"pilot_name", "craft_name", "callsign", "bind", "signature", "uid", "name"}
	for _, line := range lines {
		redacted := line
		lower := strings.ToLower(line)
		for _, key := range sensitive {
			if strings.Contains(lower, key) {
				redacted = redactValue(line)
				classes[key] = true
				break
			}
		}
		out = append(out, redacted)
	}
	var classList []string
	for class := range classes {
		classList = append(classList, class)
	}
	return out, classList
}

func redactValue(line string) string {
	if i := strings.Index(line, "="); i >= 0 {
		return strings.TrimSpace(line[:i+1]) + " REDACTED"
	}
	fields := strings.Fields(line)
	if len(fields) <= 1 {
		return line
	}
	return fields[0] + " REDACTED"
}
