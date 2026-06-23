package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	bfcommands "github.com/hajekt2/betaflight-cli/internal/commands"
	"github.com/hajekt2/betaflight-cli/internal/connection"
	"github.com/hajekt2/betaflight-cli/internal/output"
)

const (
	minMotorTestValue = 1000
	maxMotorTestValue = 2000
	maxMotorTestIndex = 15
)

type motorTestPlan struct {
	Kind                  string              `json:"kind"`
	Applied               bool                `json:"applied"`
	Dangerous             bool                `json:"dangerous"`
	MotorIndex            int                 `json:"motor_index"`
	Value                 int                 `json:"value"`
	DurationMS            int64               `json:"duration_ms"`
	CommandPreview        string              `json:"command_preview"`
	StopCommandPreview    string              `json:"stop_command_preview"`
	Stopped               bool                `json:"stopped"`
	Preflight             *motorTestPreflight `json:"preflight,omitempty"`
	PostStop              *motorTestPostStop  `json:"post_stop,omitempty"`
	Audit                 *motorTestAudit     `json:"audit,omitempty"`
	ResponseLines         map[string][]string `json:"response_lines,omitempty"`
	RequiredConfirmations []string            `json:"required_confirmations"`
	SafetyChecks          []safetyCheck       `json:"safety_checks"`
	RecommendedPreflight  []string            `json:"recommended_preflight"`
	ApplyMessage          string              `json:"apply_message"`
}

type motorTestPreflight struct {
	Source            string   `json:"source"`
	ReadOnly          bool     `json:"read_only"`
	ArmingBlocked     *bool    `json:"arming_blocked,omitempty"`
	RebootRequired    *bool    `json:"reboot_required,omitempty"`
	ActiveModes       []string `json:"active_modes,omitempty"`
	ActiveArmingFlags []string `json:"active_arming_flags,omitempty"`
}

type motorTestPostStop struct {
	Source          string   `json:"source"`
	ReadOnly        bool     `json:"read_only"`
	Outputs         []uint16 `json:"outputs,omitempty"`
	TelemetryRPM    []uint32 `json:"telemetry_rpm,omitempty"`
	OutputOrder     []uint8  `json:"output_order,omitempty"`
	WarningMessages []string `json:"warning_messages,omitempty"`
}

type motorTestAudit struct {
	Source            string   `json:"source"`
	ReadOnlyEvidence  bool     `json:"read_only_evidence"`
	DurationMS        int64    `json:"duration_ms"`
	StartCommand      string   `json:"start_command"`
	StopCommand       string   `json:"stop_command"`
	Confirmations     []string `json:"confirmations"`
	SafetyPassed      bool     `json:"safety_passed"`
	PreflightCaptured bool     `json:"preflight_captured"`
	PostStopCaptured  bool     `json:"post_stop_captured"`
	StopAttempted     bool     `json:"stop_attempted"`
	StopSucceeded     bool     `json:"stop_succeeded"`
	WarningMessages   []string `json:"warning_messages,omitempty"`
}

type safetyCheck struct {
	Name     string `json:"name"`
	Passed   bool   `json:"passed"`
	Required bool   `json:"required"`
	Detail   string `json:"detail"`
}

func (a *app) motorTestPlanCommand() *cobra.Command {
	return a.motorTestCommand("test-plan", "Plan a high-risk motor output test without sending it", false)
}

func (a *app) motorTestApplyCommand() *cobra.Command {
	return a.motorTestCommand("test-apply", "Run a tightly bounded high-risk motor output test", true)
}

func (a *app) motorTestCommand(use, short string, apply bool) *cobra.Command {
	var motor int
	var value int
	var duration time.Duration
	var propsOff bool
	var batteryAware bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := buildMotorTestPlan(motor, value, duration, propsOff, batteryAware)
			if err != nil {
				return a.render(output.Failure(commandPath(cmd), nil, "validation_error", err.Error()))
			}
			if apply {
				return a.applyMotorTestPlan(cmd, plan)
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

func (a *app) applyMotorTestPlan(cmd *cobra.Command, plan motorTestPlan) error {
	if !a.opts.yes {
		return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "motor output can spin hardware; pass --yes"))
	}
	if !motorSafetyPassed(plan, "props_off") {
		return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "motor output requires --props-off"))
	}
	if !motorSafetyPassed(plan, "battery_awareness") {
		return a.render(output.Failure(commandPath(cmd), nil, "confirmation_required", "motor output requires --battery-aware"))
	}
	return a.withClient(cmd.Context(), commandPath(cmd), connection.Dangerous, func(client *connection.Client, target output.Target) output.Envelope {
		preflight, err := readMotorTestPreflight(cmd.Context(), client)
		if err != nil {
			return a.failure(commandPath(cmd), &target, fmt.Errorf("motor preflight status unavailable: %w", err))
		}
		plan.Preflight = preflight
		responses := map[string][]string{}
		startLines, err := client.ExecCLI(cmd.Context(), plan.CommandPreview)
		if err != nil {
			return a.failure(commandPath(cmd), &target, err)
		}
		responses[plan.CommandPreview] = startLines
		timer := time.NewTimer(time.Duration(plan.DurationMS) * time.Millisecond)
		select {
		case <-cmd.Context().Done():
			if !timer.Stop() {
				<-timer.C
			}
		case <-timer.C:
		}
		stopLines, stopErr := client.ExecCLI(context.Background(), plan.StopCommandPreview)
		responses[plan.StopCommandPreview] = stopLines
		plan.Applied = true
		plan.Stopped = stopErr == nil
		plan.ResponseLines = responses
		postStopWarnings := []string{}
		postStop, postStopErr := readMotorTestPostStop(cmd.Context(), client)
		if postStopErr != nil {
			postStopWarnings = append(postStopWarnings, fmt.Sprintf("post-stop motor status unavailable: %v", postStopErr))
		} else {
			plan.PostStop = postStop
		}
		plan.Audit = buildMotorTestAudit(plan, stopErr, postStopWarnings)
		env := output.Success(commandPath(cmd), &target, map[string]any{
			"motor_test_plan": plan,
		})
		env.SideEffects = append(env.SideEffects, output.SideEffect{
			Type:    "motor_output",
			Command: plan.CommandPreview,
			Detail:  fmt.Sprintf("motor output sent for %dms; stop command attempted", plan.DurationMS),
		})
		if stopErr != nil {
			addStringWarnings(&env, []string{fmt.Sprintf("motor stop command returned an error: %v", stopErr)})
		}
		if len(postStopWarnings) > 0 {
			addStringWarnings(&env, postStopWarnings)
		}
		return env
	})
}

func readMotorTestPreflight(ctx context.Context, client *connection.Client) (*motorTestPreflight, error) {
	status, err := bfcommands.ReadRuntimeStatus(ctx, client)
	if err != nil {
		return nil, err
	}
	preflight := &motorTestPreflight{
		Source:   "runtime status",
		ReadOnly: true,
	}
	if status.Health != nil {
		preflight.ArmingBlocked = status.Health.ArmingBlocked
		preflight.RebootRequired = status.Health.RebootRequired
	}
	if status.FlightModes != nil {
		preflight.ActiveModes = append([]string(nil), status.FlightModes.ActiveNames...)
	}
	if status.Arming != nil {
		for _, flag := range status.Arming.ActiveFlags {
			if flag.Name != "" {
				preflight.ActiveArmingFlags = append(preflight.ActiveArmingFlags, flag.Name)
			}
		}
	}
	return preflight, nil
}

func readMotorTestPostStop(ctx context.Context, client *connection.Client) (*motorTestPostStop, error) {
	status, warnings, err := bfcommands.ReadMotorStatus(ctx, client)
	if err != nil {
		return nil, err
	}
	post := &motorTestPostStop{
		Source:          "motor status",
		ReadOnly:        true,
		Outputs:         append([]uint16(nil), status.Outputs...),
		OutputOrder:     append([]uint8(nil), status.OutputOrder...),
		WarningMessages: append([]string(nil), warnings...),
	}
	if len(status.Telemetry) > 0 {
		post.TelemetryRPM = make([]uint32, 0, len(status.Telemetry))
		for _, item := range status.Telemetry {
			post.TelemetryRPM = append(post.TelemetryRPM, item.RPM)
		}
	}
	return post, nil
}

func motorSafetyPassed(plan motorTestPlan, name string) bool {
	for _, check := range plan.SafetyChecks {
		if check.Name == name {
			return check.Passed
		}
	}
	return false
}

func buildMotorTestAudit(plan motorTestPlan, stopErr error, warnings []string) *motorTestAudit {
	return &motorTestAudit{
		Source:            "motor test apply",
		ReadOnlyEvidence:  true,
		DurationMS:        plan.DurationMS,
		StartCommand:      plan.CommandPreview,
		StopCommand:       plan.StopCommandPreview,
		Confirmations:     append([]string(nil), plan.RequiredConfirmations...),
		SafetyPassed:      motorSafetyPassed(plan, "props_off") && motorSafetyPassed(plan, "battery_awareness"),
		PreflightCaptured: plan.Preflight != nil,
		PostStopCaptured:  plan.PostStop != nil,
		StopAttempted:     true,
		StopSucceeded:     stopErr == nil,
		WarningMessages:   append([]string(nil), warnings...),
	}
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
		ApplyMessage: "test-plan is offline only; use test-apply with all confirmations to run the bounded motor output command",
	}, nil
}
