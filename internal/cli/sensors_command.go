package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

func (a *app) sensorsCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "sensors", Short: "Inspect sensor configuration and live sensor state"}
	cmd.AddCommand(&cobra.Command{
		Use:   "status",
		Short: "Read sensor configuration and raw IMU data over MSP",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				sensors, warnings, err := bfcommands.ReadSensorStatus(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				return output.Success(commandPath(cmd), &target, map[string]any{
					"sensors":  sensors,
					"warnings": warnings,
				})
			})
		},
	})
	cmd.AddCommand(a.sensorCalibrationCommand("calibrate-accelerometer", "Calibrate the accelerometer over MSP", bfcommands.SensorCalibrationAccelerometer))
	cmd.AddCommand(a.sensorCalibrationCommand("calibrate-magnetometer", "Calibrate the magnetometer over MSP", bfcommands.SensorCalibrationMagnetometer))
	cmd.AddCommand(a.sensorHardwareConfigCommand())
	cmd.AddCommand(a.sensorHardwareConfigJSONCommand())
	cmd.AddCommand(a.sensorAlignmentCommand())
	cmd.AddCommand(a.sensorAlignmentJSONCommand())
	cmd.AddCommand(a.sensorCompassDeclinationCommand())
	cmd.AddCommand(a.sensorCompassJSONCommand())
	return cmd
}

func (a *app) sensorAlignmentCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-alignment MAG_ALIGNMENT GYRO_ENABLED_MASK [MAG_ROLL MAG_PITCH MAG_YAW]",
		Short: "Set sensor alignment through MSP_SET_SENSOR_ALIGNMENT",
		Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.MaximumNArgs(5)),
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 && len(args) != 5 {
				return validationFailureMessage(a, cmd, "expected either 2 arguments or 5 arguments with custom magnetometer roll, pitch, and yaw")
			}
			magAlignment, err := parseUint8Arg("mag_alignment", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			gyroEnabledMask, err := parseUint8Arg("gyro_enabled_mask", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			config := bfcommands.SensorAlignmentSetConfig{
				MagnetometerAlign: magAlignment,
				GyroEnabledMask:   gyroEnabledMask,
			}
			if len(args) == 5 {
				roll, err := parseInt16Arg("mag_roll", args[2])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				pitch, err := parseInt16Arg("mag_pitch", args[3])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				yaw, err := parseInt16Arg("mag_yaw", args[4])
				if err != nil {
					return validationFailure(a, cmd, err)
				}
				config.MagCustomAlignment = &bfcommands.Axis3i16{Roll: roll, Pitch: pitch, Yaw: yaw}
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "sensor alignment changes sensor settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetSensorAlignment(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"sensor_alignment": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "sensor_alignment",
					Command: "MSP_SET_SENSOR_ALIGNMENT",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) sensorAlignmentJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-alignment-json FILE",
		Short: "Set sensor alignment from JSON through MSP_SET_SENSOR_ALIGNMENT",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseSensorAlignmentJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "sensor alignment changes sensor settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetSensorAlignment(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"sensor_alignment": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "sensor_alignment",
					Command: "MSP_SET_SENSOR_ALIGNMENT",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseSensorAlignmentJSON(data []byte) (bfcommands.SensorAlignmentSetConfig, error) {
	var wrapped struct {
		SensorAlignment *bfcommands.SensorAlignmentSetConfig `json:"sensor_alignment"`
		Alignment       *bfcommands.SensorAlignmentSetConfig `json:"alignment"`
		Config          *bfcommands.SensorAlignmentSetConfig `json:"config"`
		Sensors         *struct {
			Alignment *bfcommands.SensorAlignmentSetConfig `json:"alignment"`
		} `json:"sensors"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.SensorAlignmentSetConfig{}, err
	}
	switch {
	case wrapped.SensorAlignment != nil:
		return *wrapped.SensorAlignment, nil
	case wrapped.Alignment != nil:
		return *wrapped.Alignment, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case wrapped.Sensors != nil && wrapped.Sensors.Alignment != nil:
		return *wrapped.Sensors.Alignment, nil
	}
	var config bfcommands.SensorAlignmentSetConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.SensorAlignmentSetConfig{}, err
	}
	return config, nil
}

func (a *app) sensorHardwareConfigCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config ACC_HARDWARE BARO_HARDWARE MAG_HARDWARE RANGEFINDER_HARDWARE",
		Short: "Set sensor hardware IDs through MSP_SET_SENSOR_CONFIG",
		Args:  cobra.ExactArgs(4),
		RunE: func(cmd *cobra.Command, args []string) error {
			acc, err := parseUint8Arg("acc_hardware", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			baro, err := parseUint8Arg("baro_hardware", args[1])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			mag, err := parseUint8Arg("mag_hardware", args[2])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			rangefinder, err := parseUint8Arg("rangefinder_hardware", args[3])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "sensor hardware configuration changes sensor settings; pass --yes"))
			}
			config := bfcommands.SensorHardwareConfig{
				Accelerometer: acc,
				Barometer:     baro,
				Magnetometer:  mag,
				Rangefinder:   rangefinder,
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetSensorHardwareConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"sensor_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "sensor_config",
					Command: "MSP_SET_SENSOR_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) sensorHardwareConfigJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-config-json FILE",
		Short: "Set sensor hardware IDs from JSON through MSP_SET_SENSOR_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			config, err := parseSensorHardwareConfigJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "sensor hardware configuration changes sensor settings; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetSensorHardwareConfig(cmd.Context(), client, config)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"sensor_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "sensor_config",
					Command: "MSP_SET_SENSOR_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseSensorHardwareConfigJSON(data []byte) (bfcommands.SensorHardwareConfig, error) {
	var wrapped struct {
		SensorConfig   *bfcommands.SensorHardwareConfig `json:"sensor_config"`
		HardwareConfig *bfcommands.SensorHardwareConfig `json:"hardware_config"`
		Config         *bfcommands.SensorHardwareConfig `json:"config"`
		Sensors        *struct {
			HardwareConfig *bfcommands.SensorHardwareConfig `json:"hardware_config"`
		} `json:"sensors"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return bfcommands.SensorHardwareConfig{}, err
	}
	switch {
	case wrapped.SensorConfig != nil:
		return *wrapped.SensorConfig, nil
	case wrapped.HardwareConfig != nil:
		return *wrapped.HardwareConfig, nil
	case wrapped.Config != nil:
		return *wrapped.Config, nil
	case wrapped.Sensors != nil && wrapped.Sensors.HardwareConfig != nil:
		return *wrapped.Sensors.HardwareConfig, nil
	}
	var config bfcommands.SensorHardwareConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return bfcommands.SensorHardwareConfig{}, err
	}
	return config, nil
}

func (a *app) sensorCompassDeclinationCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-compass-declination DECI_DEGREES",
		Short: "Set compass declination through MSP_SET_COMPASS_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			declination, err := parseInt16Arg("deci_degrees", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "compass declination changes sensor configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetCompassConfig(cmd.Context(), client, declination)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"compass_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "compass_config",
					Command: "MSP_SET_COMPASS_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func (a *app) sensorCompassJSONCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "set-compass-json FILE",
		Short: "Set compass declination from JSON through MSP_SET_COMPASS_CONFIG",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			declination, err := parseCompassDeclinationJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "compass declination changes sensor configuration; pass --yes"))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Write, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.SetCompassConfig(cmd.Context(), client, declination)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"compass_config": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "compass_config",
					Command: "MSP_SET_COMPASS_CONFIG",
					Detail:  "configuration changed but not saved",
				})
				return env
			})
		},
	}
}

func parseCompassDeclinationJSON(data []byte) (int16, error) {
	var wrapped struct {
		CompassConfig *bfcommands.CompassConfig `json:"compass_config"`
		Compass       *bfcommands.CompassConfig `json:"compass"`
		Config        *bfcommands.CompassConfig `json:"config"`
		Sensors       *struct {
			Compass *bfcommands.CompassConfig `json:"compass"`
		} `json:"sensors"`
		DeclinationDeciDegrees *int16 `json:"declination_deci_degrees"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		return 0, err
	}
	switch {
	case wrapped.CompassConfig != nil:
		return wrapped.CompassConfig.DeclinationDeciDegrees, nil
	case wrapped.Compass != nil:
		return wrapped.Compass.DeclinationDeciDegrees, nil
	case wrapped.Config != nil:
		return wrapped.Config.DeclinationDeciDegrees, nil
	case wrapped.Sensors != nil && wrapped.Sensors.Compass != nil:
		return wrapped.Sensors.Compass.DeclinationDeciDegrees, nil
	case wrapped.DeclinationDeciDegrees != nil:
		return *wrapped.DeclinationDeciDegrees, nil
	}
	var compass bfcommands.CompassConfig
	if err := json.Unmarshal(data, &compass); err != nil {
		return 0, err
	}
	return compass.DeclinationDeciDegrees, nil
}

func (a *app) sensorCalibrationCommand(use, short string, kind bfcommands.SensorCalibrationKind) *cobra.Command {
	return &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !a.opts.yes {
				return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", fmt.Sprintf("%s changes sensor calibration state; pass --yes", commandPath(cmd))))
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.Dangerous, func(client *connection.Client, target output.Target) output.Envelope {
				result, err := bfcommands.CalibrateSensor(cmd.Context(), client, kind)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"sensor_calibration": result})
				env.SideEffects = append(env.SideEffects, output.SideEffect{
					Type:    "sensor_calibration",
					Command: result.MSPName,
					Detail:  fmt.Sprintf("%s calibration requested", result.Kind),
				})
				return env
			})
		},
	}
}
