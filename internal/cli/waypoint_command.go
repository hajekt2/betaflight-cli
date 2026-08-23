package cli

import (
	"encoding/json"
	"fmt"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
	"github.com/spf13/cobra"
)

// wpCommand exposes the waypoint mission domain over the firmware CLI text
// interface (cliWaypoint in upstream src/main/cli/cli.c). Reads are framed
// non-interactive CLI reads; writes are plan-first per ADR-0004/0009/0015 and
// applied as `waypoint ...` lines through the shared plan/apply machinery.
func (a *app) waypointCommand() *cobra.Command {
	cmd := &cobra.Command{Use: "wp", Short: "Inspect and change waypoint missions over the firmware CLI"}

	cmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List stored waypoints from the flight controller",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				list, warnings, err := bfcommands.ListWaypoints(cmd.Context(), client)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"waypoints": list})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	})

	cmd.AddCommand(&cobra.Command{
		Use:   "get INDEX",
		Short: "Read one stored waypoint by index",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			index, err := parseUint8Arg("index", args[0])
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.withClient(cmd.Context(), commandPath(cmd), connection.ReadOnly, func(client *connection.Client, target output.Target) output.Envelope {
				wp, warnings, err := bfcommands.GetWaypoint(cmd.Context(), client, index)
				if err != nil {
					return a.failure(commandPath(cmd), &target, err)
				}
				env := output.Success(commandPath(cmd), &target, map[string]any{"waypoint": wp})
				addStringWarnings(&env, warnings)
				return env
			})
		},
	})

	var setFlags changeFlags
	setJSON := &cobra.Command{
		Use:   "set-json FILE",
		Short: "Plan or apply a full waypoint mission from JSON using firmware CLI syntax",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := a.readInput(args[0])
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "read_failed", err.Error()))
			}
			points, err := parseWaypointRowsJSON(data)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			lines, err := bfcommands.BuildWaypointSetPlan(points)
			if err != nil {
				return validationFailure(a, cmd, err)
			}
			return a.planOrApplyCLIWithOperation(cmd, lines, "waypoints", setFlags, connection.Write)
		},
	}
	addChangeFlags(setJSON, &setFlags)
	cmd.AddCommand(setJSON)

	var clearFlags changeFlags
	clear := &cobra.Command{
		Use:   "clear",
		Short: "Plan or remove every stored waypoint",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return a.planOrApplyCLIWithOperation(cmd, bfcommands.BuildWaypointClearPlan(), "waypoints", clearFlags, connection.Dangerous)
		},
	}
	addChangeFlags(clear, &clearFlags)
	cmd.AddCommand(clear)

	return cmd
}

type waypointRow struct {
	Number       uint8   `json:"number"`
	Type         string  `json:"type"`
	Pattern      string  `json:"pattern"`
	LatitudeE7   *int64  `json:"latitude_e7"`
	LongitudeE7  *int64  `json:"longitude_e7"`
	LatitudeDec  *string `json:"latitude_degrees"`
	LongitudeDec *string `json:"longitude_degrees"`
	AltitudeCM   *int32  `json:"altitude_cm"`
	SpeedCMS     *uint16 `json:"speed_cm_s"`
	DurationDS   *uint16 `json:"duration_ds"`
}

// parseWaypointRowsJSON accepts {"waypoints":[...]} or a bare array of
// waypoint rows. Coordinates accept degrees*1e7 integers (canonical wire form
// from src/main/pg/flight_plan.h) or decimal strings matching the firmware
// coordinate parser. Pattern defaults to NONE; altitude/speed/duration default
// to zero like the firmware defaults.
func parseWaypointRowsJSON(data []byte) ([]bfcommands.Waypoint, error) {
	var wrapped struct {
		Waypoint  *waypointRow  `json:"waypoint"`
		Waypoints []waypointRow `json:"waypoints"`
	}
	if err := json.Unmarshal(data, &wrapped); err != nil {
		var rows []waypointRow
		if rowErr := json.Unmarshal(data, &rows); rowErr != nil {
			return nil, fmt.Errorf("invalid waypoint JSON: %w", err)
		}
		return convertWaypointRows(rows)
	}
	switch {
	case wrapped.Waypoint != nil:
		return convertWaypointRows([]waypointRow{*wrapped.Waypoint})
	case wrapped.Waypoints != nil:
		return convertWaypointRows(wrapped.Waypoints)
	}
	return nil, fmt.Errorf("no waypoints found; provide {\"waypoints\":[...]}")
}

func convertWaypointRows(rows []waypointRow) ([]bfcommands.Waypoint, error) {
	if len(rows) == 0 {
		return nil, fmt.Errorf("at least one waypoint is required; use 'wp clear' to remove the mission")
	}
	points := make([]bfcommands.Waypoint, 0, len(rows))
	for i, row := range rows {
		wp := bfcommands.Waypoint{
			Number:     row.Number,
			Type:       row.Type,
			Pattern:    row.Pattern,
			AltitudeCM: 0,
			SpeedCMS:   0,
			DurationDS: 0,
		}
		if wp.Type == "" {
			return nil, fmt.Errorf("waypoint %d: type is required (%s)", i, joinWaypointTypes())
		}
		if wp.Pattern == "" {
			wp.Pattern = "NONE"
		}
		switch {
		case row.LatitudeE7 != nil:
			wp.LatitudeE7 = *row.LatitudeE7
		case row.LatitudeDec != nil:
			lat, err := bfcommands.ParseDecimalCoordinate(*row.LatitudeDec)
			if err != nil {
				return nil, fmt.Errorf("waypoint %d latitude_degrees: %v", i, err)
			}
			wp.LatitudeE7 = lat
		default:
			return nil, fmt.Errorf("waypoint %d: latitude_e7 or latitude_degrees is required", i)
		}
		switch {
		case row.LongitudeE7 != nil:
			wp.LongitudeE7 = *row.LongitudeE7
		case row.LongitudeDec != nil:
			lon, err := bfcommands.ParseDecimalCoordinate(*row.LongitudeDec)
			if err != nil {
				return nil, fmt.Errorf("waypoint %d longitude_degrees: %v", i, err)
			}
			wp.LongitudeE7 = lon
		default:
			return nil, fmt.Errorf("waypoint %d: longitude_e7 or longitude_degrees is required", i)
		}
		if row.AltitudeCM != nil {
			wp.AltitudeCM = *row.AltitudeCM
		}
		if row.SpeedCMS != nil {
			wp.SpeedCMS = *row.SpeedCMS
		}
		if row.DurationDS != nil {
			wp.DurationDS = *row.DurationDS
		}
		points = append(points, wp)
	}
	return points, nil
}

func joinWaypointTypes() string {
	out := ""
	for i, name := range bfcommands.WaypointTypeNames {
		if i > 0 {
			out += ", "
		}
		out += name
	}
	return out
}
