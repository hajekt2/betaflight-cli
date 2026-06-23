package commands

import (
	"context"
	"fmt"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

type RuntimeStatus struct {
	Runtime           *Status             `json:"runtime"`
	ActiveSensorNames []string            `json:"active_sensor_names,omitempty"`
	Arming            *ArmingDisableState `json:"arming,omitempty"`
	RebootRequired    *bool               `json:"reboot_required,omitempty"`
}

func ReadRuntimeStatus(ctx context.Context, client *connection.Client) (*RuntimeStatus, error) {
	status, err := readStatus(ctx, client)
	if err != nil {
		return nil, fmt.Errorf("runtime status unavailable: %w", err)
	}
	out := &RuntimeStatus{
		Runtime:           status,
		ActiveSensorNames: activeSensorNames(status.ActiveSensors),
		Arming:            DecodeArmingDisableState(status),
	}
	if status.ConfigStateFlag != nil {
		rebootRequired := *status.ConfigStateFlag&1 != 0
		out.RebootRequired = &rebootRequired
	}
	return out, nil
}
