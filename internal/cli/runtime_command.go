package cli

import (
	"encoding/json"
	"fmt"
	"time"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

func (a *app) statusCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Read compact runtime status over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				status, err := bfcommands.ReadRuntimeStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"status": status,
				})
			})
		},
	}
}

func (a *app) tasksCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "tasks", Short: "Inspect scheduler task diagnostics"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read scheduler task statistics through Betaflight CLI",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				tasks, err := bfcommands.ReadTaskStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{
					"tasks": tasks,
				})
				env.SideEffects = append(env.SideEffects, output.SideEffect{Type: "task_stats_reset", Command: "tasks", Detail: "Betaflight resets max task execution statistics after printing tasks"})
				return env
			})
		},
	})
	return cmd
}

func (a *app) systemCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "system", Short: "Inspect CLI-backed system diagnostics"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read Betaflight CLI status output as structured diagnostics",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				system, err := bfcommands.ReadSystemStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"system": system,
				})
			})
		},
	})
	return cmd
}

func (a *app) telemetryCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "telemetry", Short: "Read telemetry"}
	readSnapshot := func(cmd *cobra.Command, _ []string) error {
		return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
			telemetry, warnings := bfcommands.ReadTelemetry(cmd.Context(), client)
			env := output.Success(commandPath(cmd), &target, map[string]any{
				"telemetry": telemetry,
			})
			addStringWarnings(&env, warnings)
			return env
		})
	}
	cmd.RunE = readSnapshot
	cmd.AddCommand(&cobra.Command{
		Use:   "snapshot",
		Short: "Read one telemetry snapshot",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return readSnapshot(cmd, nil)
		},
	})
	return cmd
}

func (a *app) debugCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "debug", Short: "Inspect live debug channels and trim values"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read MSP debug values and accelerometer trims",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				debug, warnings, err := bfcommands.ReadDebugStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"debug":    debug,
					"warnings": warnings,
				})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-accelerometer-trim PITCH ROLL",
		Short: "Set accelerometer trim over MSP",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			pitch, err := parseInt16Arg("PITCH", args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			roll, err := parseInt16Arg("ROLL", args[1])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "accelerometer trim changes calibration state; pass --yes"))
			}
			trim := bfcommands.AccelerometerTrim{Pitch: pitch, Roll: roll}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetAccelerometerTrim(cmd.Context(), client, trim)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"accelerometer_trim": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "accelerometer_trim",
					Command: result.MSPName,
					Detail:  "accelerometer trim updated",
				})
				return env
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-accelerometer-trim-json FILE",
		Short: "Set accelerometer trim from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			trim, err := parseAccelerometerTrimJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "accelerometer trim changes calibration state; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetAccelerometerTrim(cmd.Context(), client, trim)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"accelerometer_trim": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "accelerometer_trim",
					Command: result.MSPName,
					Detail:  "accelerometer trim updated",
				})
				return env
			})
		},
	})
	return cmd
}

func parseAccelerometerTrimJSON(data []byte) (bfcommands.AccelerometerTrim, error) {
	var wrapped struct {
		AccelerometerTrim *bfcommands.AccelerometerTrim `json:"accelerometer_trim"`
		Trim              *bfcommands.AccelerometerTrim `json:"trim"`
		Config            *bfcommands.AccelerometerTrim `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.AccelerometerTrim{}, err
	}
	switch {
	case wrapped.AccelerometerTrim != nil:
		return *wrapped.AccelerometerTrim, nil
	case wrapped.Trim != nil:
		return *wrapped.Trim, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var trim bfcommands.AccelerometerTrim
	if err := json.Unmarshal(data, &trim); err != nil {
		return bfcommands.AccelerometerTrim{}, err
	}
	return trim, nil
}

func (a *app) environmentCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "environment", Short: "Inspect live altitude, rangefinder, and analog readings"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read altitude, rangefinder, and analog readings over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				environment, warnings, err := bfcommands.ReadEnvironmentStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"environment": environment,
					"warnings":    warnings,
				})
			})
		},
	})
	return cmd
}

func (a *app) rtcCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "rtc", Short: "Inspect flight controller real-time clock"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read RTC datetime over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				rtc, warnings, err := bfcommands.ReadRTCStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"rtc":      rtc,
					"warnings": warnings,
				})
			})
		},
	})
	var timestamp string
	var now bool
	set := &cobra.Command{
		Use:   "set",
		Short: "Set RTC datetime over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if (timestamp == "") == !now {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", "use exactly one of --timestamp or --now"))
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "rtc set changes flight controller time; pass --yes"))
			}
			value := time.Now().UTC()
			if timestamp != "" {
				parsed, err := time.Parse(time.RFC3339Nano, timestamp)
				if err != nil {
					return a.render(output.Failure(commandPath(cmd), nil, "validation_error", fmt.Sprintf("invalid --timestamp: %v", err)))
				}
				value = parsed
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRTC(cmd.Context(), client, value)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rtc": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rtc_set",
					Command: result.MSPName,
					Detail:  "flight controller RTC updated",
				})
				return env
			})
		},
	}
	set.Flags().StringVar(&timestamp, "timestamp", "", "UTC timestamp to set, formatted as RFC3339/RFC3339Nano")
	set.Flags().BoolVar(&now, "now", false, "set RTC to the local machine's current UTC time")
	cmd.AddCommand(set)
	cmd.AddCommand(&cobra.Command{
		Use:   "set-json FILE",
		Short: "Set RTC datetime from JSON over MSP",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			value, err := parseRTCSetJSON(data, time.Now)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "rtc set changes flight controller time; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetRTC(cmd.Context(), client, value)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"rtc": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "rtc_set",
					Command: result.MSPName,
					Detail:  "flight controller RTC updated",
				})
				return env
			})
		},
	})
	return cmd
}

func parseRTCSetJSON(data []byte, now func() time.Time) (time.Time, error) {
	var wrapped struct {
		Timestamp    string `json:"timestamp"`
		TimestampUTC string `json:"timestamp_utc"`
		ISOUTC       string `json:"iso_utc"`
		Now          *bool  `json:"now"`
		RTC          *struct {
			Timestamp    string `json:"timestamp"`
			TimestampUTC string `json:"timestamp_utc"`
			ISOUTC       string `json:"iso_utc"`
			Now          *bool  `json:"now"`
		} `json:"rtc"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return time.Time{}, err
	}
	timestamp := firstNonEmpty(wrapped.Timestamp, wrapped.TimestampUTC, wrapped.ISOUTC)
	useNow := wrapped.Now != nil && *wrapped.Now
	if wrapped.RTC != nil {
		timestamp = firstNonEmpty(wrapped.RTC.Timestamp, wrapped.RTC.TimestampUTC, wrapped.RTC.ISOUTC, timestamp)
		useNow = useNow || wrapped.RTC.Now != nil && *wrapped.RTC.Now
	}
	if (timestamp == "") == !useNow {
		return time.Time{}, fmt.Errorf("use exactly one of timestamp/timestamp_utc/iso_utc or now")
	}
	if useNow {
		return now().UTC(), nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid timestamp: %w", err)
	}
	return parsed, nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
