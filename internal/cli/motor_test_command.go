package cli

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/hajekt2/betaflight-cli/internal/output"
)

const (
	minMotorTestValue = 1000
	maxMotorTestValue = 2000
	maxMotorTestIndex = 15
)

type motorTestPlan struct {
	Kind                    string        `json:"kind"`
	Applied                 bool          `json:"applied"`
	Dangerous               bool          `json:"dangerous"`
	MotorIndex              int           `json:"motor_index"`
	Value                   int           `json:"value"`
	DurationMS              int64         `json:"duration_ms"`
	CommandPreview          string        `json:"command_preview"`
	StopCommandPreview      string        `json:"stop_command_preview"`
	RequiredConfirmations   []string      `json:"required_confirmations"`
	SafetyChecks            []safetyCheck `json:"safety_checks"`
	RecommendedPreflight    []string      `json:"recommended_preflight"`
	UnsupportedApplyMessage string        `json:"unsupported_apply_message"`
}

type safetyCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

func (a *app) motorTestPlanCommand() *cobra.Command {
	var motor int
	var value int
	var duration time.Duration
	var propsOff bool
	var batteryAware bool
	cmd := &cobra.Command{
		Use:   "test-plan",
		Short: "Plan a high-risk motor output test without sending it",
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := buildMotorTestPlan(motor, value, duration, propsOff, batteryAware)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			return a.render(output.Success(commandPath(cmd), nil, map[string]any{
				"motor_test_plan": plan,
			}))
		},
	}
	cmd.Flags().IntVar(&motor, "motor", 0, "zero-based motor index to test")
	cmd.Flags().IntVar(&value, "value", minMotorTestValue, "motor output value in Betaflight motor command units")
	cmd.Flags().DurationVar(&duration, "duration", time.Second, "planned test duration, capped at 5s")
	cmd.Flags().BoolVar(&propsOff, "props-off", false, "acknowledge propellers are removed")
	cmd.Flags().BoolVar(&batteryAware, "battery-aware", false, "acknowledge external power and bench safety have been reviewed")
	return cmd
}

func buildMotorTestPlan(motor, value int, duration time.Duration, propsOff, batteryAware bool) (motorTestPlan, error) {
	switch {
	case motor < 0 || motor > maxMotorTestIndex:
		return motorTestPlan{}, fmt.Errorf("--motor must be between 0 and %d", maxMotorTestIndex)
	case value < minMotorTestValue || value > maxMotorTestValue:
		return motorTestPlan{}, fmt.Errorf("--value must be between %d and %d", minMotorTestValue, maxMotorTestValue)
	case duration <= 0:
		return motorTestPlan{}, fmt.Errorf("--duration must be positive")
	case duration > 5*time.Second:
		return motorTestPlan{}, fmt.Errorf("--duration is capped at 5s")
	}
	checks := []safetyCheck{
		{Name: "props_off", Passed: propsOff, Required: true, Detail: "propellers must be removed before any future motor output apply command"},
		{Name: "battery_awareness", Passed: batteryAware, Required: true, Detail: "operator must review power source, bench restraint, and ESC arming state"},
		{Name: "dry_run_only", Passed: true, Required: true, Detail: "this command only creates a plan and never sends motor output"},
	}
	return motorTestPlan{
		Kind:               "motor_test",
		Applied:            false,
		Dangerous:          true,
		MotorIndex:         motor,
		Value:              value,
		DurationMS:         duration.Milliseconds(),
		CommandPreview:     fmt.Sprintf("motor %d %d", motor, value),
		StopCommandPreview: fmt.Sprintf("motor %d %d", motor, minMotorTestValue),
		RequiredConfirmations: []string{
			"--yes",
			"--props-off",
			"--battery-aware",
		},
		SafetyChecks: checks,
		RecommendedPreflight: []string{
			"remove all propellers",
			"secure the frame on a bench",
			"verify the intended motor index",
			"keep duration and throttle value minimal",
			"be ready to disconnect power",
		},
		UnsupportedApplyMessage: "motor output apply is intentionally not implemented yet; this plan defines the safety contract first",
	}, nil
}
