package cli

import (
	"encoding/json"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) mixerCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "mixer", Short: "Inspect mixer and motor direction configuration"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read mixer mode and motor direction over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				mixer, err := bfcommands.ReadMixerStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"mixer": mixer,
				})
			})
		},
	})
	cmd.AddCommand(&cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set mixer mode and motor direction from JSON through MSP_SET_MIXER_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseMixerConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "mixer config changes affect motor output; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetMixerConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"mixer_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "mixer_config",
					Command: "MSP_SET_MIXER_CONFIG",
					Detail:  "mixer config changed but not saved",
				})
				return env
			})
		},
	})
	return cmd
}

func parseMixerConfigJSON(data []byte) (bfcommands.MixerConfig, error) {
	var wrapped struct {
		MixerConfig *bfcommands.MixerConfig `json:"mixer_config"`
		Config      *bfcommands.MixerConfig `json:"config"`
		Mixer       *struct {
			Mode              *bfcommands.MixerMode `json:"mixer"`
			YawMotorsReversed *bool                 `json:"yaw_motors_reversed"`
		} `json:"mixer"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.MixerConfig{}, err
	}
	if wrapped.MixerConfig != nil {
		return *wrapped.MixerConfig, nil
	}
	if wrapped.Config != nil {
		return *wrapped.Config, nil
	}
	if wrapped.Mixer != nil && wrapped.Mixer.Mode != nil {
		config := bfcommands.MixerConfig{Mode: wrapped.Mixer.Mode.ID}
		if wrapped.Mixer.YawMotorsReversed != nil {
			config.YawMotorsReversed = *wrapped.Mixer.YawMotorsReversed
		}
		return config, nil
	}
	var config bfcommands.MixerConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.MixerConfig{}, err
	}
	return config, nil
}

func (a *app) motorsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "motors", Short: "Inspect motor configuration and live outputs"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read motor configuration, outputs, and telemetry over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				motors, warnings, err := bfcommands.ReadMotorStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"motors":   motors,
					"warnings": warnings,
				})
			})
		},
	})
	cmd.AddCommand(a.motorConfigCommand())
	cmd.AddCommand(a.motorConfigJSONCommand())
	cmd.AddCommand(a.motor3DConfigCommand())
	cmd.AddCommand(a.motor3DConfigJSONCommand())
	cmd.AddCommand(a.motorTestPlanCommand())
	cmd.AddCommand(a.motorTestApplyCommand())
	return cmd
}

func (a *app) motorConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config MAX_THROTTLE MIN_COMMAND MOTOR_POLES USE_DSHOT_TELEMETRY",
		Short: "Set motor configuration through MSP_SET_MOTOR_CONFIG",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			maxThrottle, err := parseUint16Arg("max_throttle", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			minCommand, err := parseUint16Arg("min_command", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			motorPoles, err := parseUint8Arg("motor_poles", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			useDShotTelemetry, err := parseBoolFlagArg("use_dshot_telemetry", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "motor configuration changes motor settings; pass --yes"))
			}
			config := bfcommands.MotorConfigSetConfig{
				MaxThrottle:       maxThrottle,
				MinCommand:        minCommand,
				MotorPoles:        motorPoles,
				UseDShotTelemetry: useDShotTelemetry,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetMotorConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"motor_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "motor_config",
					Command: "MSP_SET_MOTOR_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) motorConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set motor configuration from JSON through MSP_SET_MOTOR_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseMotorConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "motor configuration changes motor settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetMotorConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"motor_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "motor_config",
					Command: "MSP_SET_MOTOR_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseMotorConfigJSON(data []byte) (bfcommands.MotorConfigSetConfig, error) {
	var wrapped struct {
		MotorConfig *bfcommands.MotorConfigSetConfig `json:"motor_config"`
		Motors      *bfcommands.MotorConfigSetConfig `json:"motors"`
		Config      *bfcommands.MotorConfigSetConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.MotorConfigSetConfig{}, err
	}
	switch {
	case wrapped.MotorConfig != nil:
		return *wrapped.MotorConfig, nil
	case wrapped.Motors != nil:
		return *wrapped.Motors, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var config bfcommands.MotorConfigSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.MotorConfigSetConfig{}, err
	}
	return config, nil
}

func (a *app) motor3DConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-3d-config DEADBAND_LOW DEADBAND_HIGH NEUTRAL",
		Short: "Set 3D motor deadband and neutral values through MSP_SET_MOTOR_3D_CONFIG",
		Args:  cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			low, err := parseUint16Arg("deadband_low", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			high, err := parseUint16Arg("deadband_high", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			neutral, err := parseUint16Arg("neutral", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "3D motor configuration changes motor settings; pass --yes"))
			}
			config := bfcommands.Motor3DConfig{DeadbandLow: low, DeadbandHigh: high, Neutral: neutral}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetMotor3DConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"motor_3d_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "motor_3d_config",
					Command: "MSP_SET_MOTOR_3D_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) motor3DConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-3d-config-json FILE",
		Short: "Set 3D motor deadband and neutral values from JSON through MSP_SET_MOTOR_3D_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseMotor3DConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "3D motor configuration changes motor settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetMotor3DConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"motor_3d_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "motor_3d_config",
					Command: "MSP_SET_MOTOR_3D_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseMotor3DConfigJSON(data []byte) (bfcommands.Motor3DConfig, error) {
	var wrapped struct {
		Motor3DConfig *bfcommands.Motor3DConfig `json:"motor_3d_config"`
		Motors        *bfcommands.Motor3DConfig `json:"motors"`
		Config        *bfcommands.Motor3DConfig `json:"config"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.Motor3DConfig{}, err
	}
	switch {
	case wrapped.Motor3DConfig != nil:
		return *wrapped.Motor3DConfig, nil
	case wrapped.Motors != nil:
		return *wrapped.Motors, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	}
	var config bfcommands.Motor3DConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.Motor3DConfig{}, err
	}
	return config, nil
}
