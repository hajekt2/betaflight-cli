package commands

import (
	"context"
	"fmt"
	"strings"

	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/pkg/msp"
)

const featureConfigLength = 4

var featureDefinitions = []FeatureDefinition{
	{Bit: 0, Name: "RX_PPM", Group: "receiver"},
	{Bit: 1, Name: "RX_UDP", Group: "receiver"},
	{Bit: 2, Name: "INFLIGHT_ACC_CAL", Group: "other"},
	{Bit: 3, Name: "RX_SERIAL", Group: "receiver"},
	{Bit: 4, Name: "MOTOR_STOP", Group: "motor"},
	{Bit: 5, Name: "SERVO_TILT", Group: "servo"},
	{Bit: 6, Name: "SOFTSERIAL", Group: "serial"},
	{Bit: 7, Name: "GPS", Group: "gps"},
	{Bit: 8, Name: "OPTICALFLOW", Group: "sensor"},
	{Bit: 9, Name: "RANGEFINDER", Group: "sensor"},
	{Bit: 10, Name: "TELEMETRY", Group: "telemetry"},
	{Bit: 12, Name: "3D", Group: "motor"},
	{Bit: 13, Name: "RX_PARALLEL_PWM", Group: "receiver"},
	{Bit: 14, Name: "RX_MSP", Group: "receiver"},
	{Bit: 15, Name: "RSSI_ADC", Group: "receiver"},
	{Bit: 16, Name: "LED_STRIP", Group: "led"},
	{Bit: 17, Name: "DISPLAY", Group: "display"},
	{Bit: 18, Name: "OSD", Group: "osd"},
	{Bit: 20, Name: "CHANNEL_FORWARDING", Group: "servo"},
	{Bit: 21, Name: "TRANSPONDER", Group: "other"},
	{Bit: 22, Name: "AIRMODE", Group: "motor"},
	{Bit: 25, Name: "RX_SPI", Group: "receiver"},
	{Bit: 27, Name: "ESC_SENSOR", Group: "esc"},
	{Bit: 28, Name: "ANTI_GRAVITY", Group: "pid"},
}

type FeatureStatus struct {
	Source       string        `json:"source"`
	Mask         uint32        `json:"mask"`
	Enabled      []FeatureFlag `json:"enabled"`
	EnabledNames []string      `json:"enabled_names"`
	Catalog      []FeatureFlag `json:"catalog"`
	UnknownMask  uint32        `json:"unknown_mask,omitempty"`
}

type FeatureDefinition struct {
	Bit   uint8  `json:"bit"`
	Name  string `json:"name"`
	Group string `json:"group,omitempty"`
}

type FeatureFlag struct {
	Bit     uint8  `json:"bit"`
	Mask    uint32 `json:"mask"`
	Name    string `json:"name"`
	Group   string `json:"group,omitempty"`
	Enabled bool   `json:"enabled"`
	Known   bool   `json:"known"`
}

type FeatureMaskSetResult struct {
	Features     FeatureStatus `json:"features"`
	MSPCode      uint16        `json:"msp_code"`
	MSPName      string        `json:"msp_name"`
	Acknowledged bool          `json:"acknowledged"`
	SaveRequired bool          `json:"save_required"`
}

func NormalizeFeatureName(name string) (string, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(name))
	_, ok := LookupFeatureDefinition(normalized)
	return normalized, ok
}

func LookupFeatureDefinition(name string) (FeatureDefinition, bool) {
	normalized := strings.ToUpper(strings.TrimSpace(name))
	for _, definition := range featureDefinitions {
		if definition.Name == normalized {
			return definition, true
		}
	}
	return FeatureDefinition{}, false
}

func ReadFeatureStatus(ctx context.Context, client *connection.Client) (*FeatureStatus, error) {
	frame, err := client.Request(ctx, msp.MSPFeatureConfig, nil)
	if err != nil {
		return nil, fmt.Errorf("feature config unavailable: %w", err)
	}
	status, err := DecodeFeatureStatus(frame.Payload)
	if err != nil {
		return nil, err
	}
	status.Source = "MSP_FEATURE_CONFIG"
	return status, nil
}

func SetFeatureMask(ctx context.Context, client *connection.Client, mask uint32) (*FeatureMaskSetResult, error) {
	if _, err := client.Request(ctx, msp.MSPSetFeatureConfig, appendU32Payload(nil, mask)); err != nil {
		return nil, fmt.Errorf("feature config request failed: %w", err)
	}
	status, err := DecodeFeatureStatus(appendU32Payload(nil, mask))
	if err != nil {
		return nil, err
	}
	status.Source = "MSP_SET_FEATURE_CONFIG"
	return &FeatureMaskSetResult{
		Features:     *status,
		MSPCode:      msp.MSPSetFeatureConfig,
		MSPName:      "MSP_SET_FEATURE_CONFIG",
		Acknowledged: true,
		SaveRequired: true,
	}, nil
}

func DecodeFeatureStatus(payload []byte) (*FeatureStatus, error) {
	if len(payload) < featureConfigLength {
		return nil, fmt.Errorf("payload length %d is shorter than MSP_FEATURE_CONFIG size %d", len(payload), featureConfigLength)
	}
	r := msp.NewPayloadReader(payload)
	mask, err := r.U32()
	if err != nil {
		return nil, err
	}
	status := &FeatureStatus{
		Mask:         mask,
		Enabled:      []FeatureFlag{},
		EnabledNames: []string{},
		Catalog:      []FeatureFlag{},
	}
	for _, definition := range featureDefinitions {
		flagMask := uint32(1) << definition.Bit
		enabled := mask&flagMask != 0
		flag := FeatureFlag{
			Bit:     definition.Bit,
			Mask:    flagMask,
			Name:    definition.Name,
			Group:   definition.Group,
			Enabled: enabled,
			Known:   true,
		}
		status.Catalog = append(status.Catalog, flag)
		if enabled {
			status.Enabled = append(status.Enabled, flag)
			status.EnabledNames = append(status.EnabledNames, definition.Name)
		}
	}
	status.UnknownMask = mask &^ knownFeatureMask()
	return status, nil
}

func knownFeatureMask() uint32 {
	var mask uint32
	for _, definition := range featureDefinitions {
		mask |= uint32(1) << definition.Bit
	}
	return mask
}
