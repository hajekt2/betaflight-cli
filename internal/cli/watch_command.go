package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) watchCommand() *cobra.Command {
	var (
		interval time.Duration
		count    int
		duration time.Duration
	)
	cmd := &cobra.Command{
		Use:       "watch <profile>",
		Short:     "Stream sampled flight controller data as newline-delimited JSON envelopes",
		ValidArgs: bfcommands.ValidWatchProfiles(),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", fmt.Sprintf("watch requires exactly one profile argument (%s)", strings.Join(bfcommands.ValidWatchProfiles(), ", "))))
			}
			sampler, err := bfcommands.SamplerFor(bfcommands.WatchProfile(args[0]))
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			opts := bfcommands.LoopOptions{
				Interval: interval,
				Count:    count,
				Duration: duration,
			}
			return a.runWatch(cmd, sampler, opts)
		},
	}
	cmd.Flags().DurationVar(&interval, "interval", 250*time.Millisecond, "time between samples, minimum 50ms")
	cmd.Flags().IntVar(&count, "count", 0, "number of samples to emit; 0 streams until interrupted")
	cmd.Flags().DurationVar(&duration, "duration", 0, "stop sampling after this wall-clock duration; 0 means unbounded")
	return cmd
}

// runWatch opens one read-only connection and emits one compact JSON envelope
// per sample as a single NDJSON line. It mirrors withClient but loops inside
// the connected phase instead of rendering a single envelope.
func (a *app) runWatch(cmd *cobra.Command, sampler bfcommands.Sampler, opts bfcommands.LoopOptions) error {
	command := commandPath(cmd)
	op := connection.ReadOnly
	connect := a.connect
	if connect == nil {
		connect = connection.Connect
	}
	cfg := a.connectionConfig()
	client, targetInfo, err := connect(cmd.Context(), cfg, op)
	target := toOutputTarget(targetInfo)
	if err != nil {
		env := a.failure(command, &target, err)
		a.addVerboseConnectionDiagnostics(&env, op, cfg, targetInfo, err)
		return a.render(env)
	}
	defer client.Close()

	out := a.out
	if out == nil {
		out = os.Stdout
	}

	ctx := cmd.Context()
	ticker := time.NewTicker(bfcommands.NormalizedWatchInterval(opts.Interval))
	defer ticker.Stop()

	bound := func(ctx context.Context, _ *connection.Client) (any, error) {
		return sampler(ctx, client)
	}
	emit := func(item any) error {
		event, ok := item.(*bfcommands.WatchEvent)
		if !ok {
			return fmt.Errorf("unexpected watch event type %T", item)
		}
		env := watchEnvelope(command, &target, event)
		a.addUnsupportedFirmwareWarning(&env, targetInfo)
		line, err := json.Marshal(env)
		if err != nil {
			return fmt.Errorf("watch encode: %w", err)
		}
		if _, err := out.Write(append(line, '\n')); err != nil {
			return fmt.Errorf("watch write: %w", err)
		}
		return nil
	}

	if err := bfcommands.RunWatchLoop(ctx, opts, ticker.C, bound, emit); err != nil {
		return a.render(output.Failure(commandPath(cmd), &target, "watch_error", err.Error()))
	}
	return nil
}

// watchEnvelope maps one loop event onto the standard envelope schema with a
// 1-based seq in data and the sampled payload nested under data.sample so
// consumers always have a stable root key; sampling failures become ok=false
// failure envelopes so the NDJSON stream stays valid line-by-line even on
// errors.
func watchEnvelope(command string, target *output.Target, event *bfcommands.WatchEvent) output.Envelope {
	if event.Err != nil {
		env := output.Failure(command, target, "sample_error", event.Err.Error())
		env.Data = map[string]any{"seq": event.Seq}
		return env
	}
	data := map[string]any{"seq": event.Seq}
	sample := any(event.Value)
	if event.Value != nil {
		if fields, ok := flattenJSON(event.Value); ok {
			sample = fields
		}
	}
	data["sample"] = sample
	return output.Success(command, target, data)
}

// flattenJSON turns a sampled value into its JSON object fields so they nest
// under data.sample without colliding with data.seq.
func flattenJSON(value any) (map[string]any, bool) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, false
	}
	fields := map[string]any{}
	if err := json.Unmarshal(raw, &fields); err != nil {
		return nil, false
	}
	return fields, true
}
