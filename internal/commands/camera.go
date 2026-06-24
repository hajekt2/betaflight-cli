package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

type CameraControlKey struct {
	Name string `json:"name"`
	Code uint8  `json:"code"`
}

type CameraControlResult struct {
	Supported         bool             `json:"supported"`
	UnsupportedReason string           `json:"unsupported_reason,omitempty"`
	Key               CameraControlKey `json:"key"`
	MSPCode           uint16           `json:"msp_code"`
	MSPName           string           `json:"msp_name"`
	Applied           bool             `json:"applied"`
	Acknowledged      bool             `json:"acknowledged"`
	SaveRequired      bool             `json:"save_required"`
	Confirmation      string           `json:"confirmation,omitempty"`
}

var cameraControlKeys = []CameraControlKey{
	{Name: "enter", Code: 0},
	{Name: "left", Code: 1},
	{Name: "up", Code: 2},
	{Name: "right", Code: 3},
	{Name: "down", Code: 4},
}

func ListCameraControlKeys() []CameraControlKey {
	out := make([]CameraControlKey, len(cameraControlKeys))
	copy(out, cameraControlKeys)
	return out
}

func ParseCameraControlKey(name string) (CameraControlKey, error) {
	normalized := strings.ToLower(strings.TrimSpace(name))
	for _, key := range cameraControlKeys {
		if key.Name == normalized {
			return key, nil
		}
	}
	return CameraControlKey{}, fmt.Errorf("unknown camera control key %q", name)
}

func PlanCameraControlPress(key CameraControlKey) CameraControlResult {
	return CameraControlResult{
		Supported:    true,
		Key:          key,
		MSPCode:      msp.MSPCameraControl,
		MSPName:      "MSP_CAMERA_CONTROL",
		Applied:      false,
		SaveRequired: false,
		Confirmation: "--yes",
	}
}

func PressCameraControl(ctx context.Context, client *connection.Client, key CameraControlKey) (*CameraControlResult, error) {
	if _, err := client.Request(ctx, msp.MSPCameraControl, []byte{key.Code}); err != nil {
		var coded *connection.CodedError
		if errors.As(err, &coded) && coded.Code == "unsupported_msp" {
			result := PlanCameraControlPress(key)
			result.Supported = false
			result.UnsupportedReason = coded.Message
			result.Confirmation = ""
			return &result, nil
		}
		return nil, fmt.Errorf("camera control key %q failed: %w", key.Name, err)
	}
	return &CameraControlResult{
		Supported:    true,
		Key:          key,
		MSPCode:      msp.MSPCameraControl,
		MSPName:      "MSP_CAMERA_CONTROL",
		Applied:      true,
		Acknowledged: true,
		SaveRequired: false,
	}, nil
}
