package cli

import (
	"context"
	"fmt"
	"strings"
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
	Kind                  string               `json:"kind"`
	Applied               bool                 `json:"applied"`
	Dangerous             bool                 `json:"dangerous"`
	AllMotors             bool                 `json:"all_motors"`
	MotorCount            int                  `json:"motor_count,omitempty"`
	MotorIndexes          []int                `json:"motor_indexes,omitempty"`
	MotorIndex            int                  `json:"motor_index"`
	Value                 int                  `json:"value"`
	DurationMS            int64                `json:"duration_ms"`
	CommandPreview        string               `json:"command_preview"`
	StopCommandPreview    string               `json:"stop_command_preview"`
	CommandPreviews       []string             `json:"command_previews"`
	StopCommandPreviews   []string             `json:"stop_command_previews"`
	Stopped               bool                 `json:"stopped"`
	Preflight             *motorTestPreflight  `json:"preflight,omitempty"`
	PostStop              *motorTestPostStop   `json:"post_stop,omitempty"`
	Comparison            *motorTestComparison `json:"comparison,omitempty"`
	Audit                 *motorTestAudit      `json:"audit,omitempty"`
	ResponseLines         map[string][]string  `json:"response_lines,omitempty"`
	RequiredConfirmations []string             `json:"required_confirmations"`
	SafetyChecks          []safetyCheck        `json:"safety_checks"`
	RecommendedPreflight  []string             `json:"recommended_preflight"`
	ApplyMessage          string               `json:"apply_message"`
}

type motorTestPreflight struct {
	hazardousReadOnlyEvidence
	ArmingBlocked     *bool    `json:"arming_blocked,omitempty"`
	RebootRequired    *bool    `json:"reboot_required,omitempty"`
	ActiveModes       []string `json:"active_modes,omitempty"`
	ActiveArmingFlags []string `json:"active_arming_flags,omitempty"`
}

type motorTestPostStop struct {
	hazardousReadOnlyEvidence
	Outputs         []uint16 `json:"outputs,omitempty"`
	TelemetryRPM    []uint32 `json:"telemetry_rpm,omitempty"`
	OutputOrder     []uint8  `json:"output_order,omitempty"`
	WarningMessages []string `json:"warning_messages,omitempty"`
}

type motorTestComparison struct {
	hazardousCaptureSummary
	OutputCount          int      `json:"output_count"`
	NonMinCommandOutputs int      `json:"non_min_command_outputs"`
	MaxOutput            uint16   `json:"max_output,omitempty"`
	MinOutput            uint16   `json:"min_output,omitempty"`
	TelemetrySampleCount int      `json:"telemetry_sample_count"`
	Notes                []string `json:"notes,omitempty"`
}

type motorTestAudit struct {
	hazardousActionAudit
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
	var motorCount int
	var value int
	var duration time.Duration
	var all bool
	var propsOff bool
	var batteryAware bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		RunE: func(cmd *cobra.Command, _ []string) error {
			plan, err := buildMotorTestPlan(motor, all, motorCount, value, duration, propsOff, batteryAware, apply)
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
	cmd.Flags().BoolVar(&all, "all", false, "test all motors from index 0 to motor-count-1")
	cmd.Flags().IntVar(&motorCount, "motor-count", 0, "number of motors to test when --all is set")
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
		startedAt := time.Now()
		var stopErr error
		var startErr error
		for _, command := range plan.CommandPreviews {
			lines, err := client.ExecCLI(cmd.Context(), command)
			responses[command] = lines
			if err != nil {
				startErr = err
				break
			}
		}
		if startErr == nil {
			timer := time.NewTimer(time.Duration(plan.DurationMS) * time.Millisecond)
			select {
			case <-cmd.Context().Done():
				if !timer.Stop() {
					<-timer.C
				}
			case <-timer.C:
			}
		}
		for _, command := range plan.StopCommandPreviews {
			lines, err := client.ExecCLI(context.Background(), command)
			responses[command] = lines
			if err != nil && stopErr == nil {
				stopErr = err
			}
		}
		elapsedMS := time.Since(startedAt).Milliseconds()
		plan.Applied = true
		plan.Stopped = stopErr == nil
		plan.ResponseLines = responses
		postStopWarnings := []string{}
		postStop, postStopErr := readMotorTestPostStop(cmd.Context(), client)
		if postStopErr != nil {
			postStopWarnings = append(postStopWarnings, fmt.Sprintf("post-stop motor status unavailable: %v", postStopErr))
		} else {
			plan.PostStop = postStop
			plan.Comparison = buildMotorTestComparison(plan.Preflight, postStop)
		}
		plan.Audit = buildMotorTestAudit(plan, elapsedMS, stopErr, postStopWarnings)
		env := output.Success(commandPath(cmd), &target, map[string]any{
			"motor_test_plan": plan,
		})
		env.SideEffects = append(env.SideEffects, output.SideEffect{
			Type:    "motor_output",
			Command: strings.Join(plan.CommandPreviews, "; "),
			Detail:  fmt.Sprintf("motor output sent for %dms on %d command(s); stop command(s) attempted", plan.DurationMS, len(plan.StopCommandPreviews)),
		})
		if startErr != nil {
			addStringWarnings(&env, []string{fmt.Sprintf("motor start command returned an error: %v", startErr)})
		}
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
		hazardousReadOnlyEvidence: hazardousReadOnlyEvidence{
			Source:   "runtime status",
			ReadOnly: true,
		},
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
		hazardousReadOnlyEvidence: hazardousReadOnlyEvidence{
			Source:   "motor status",
			ReadOnly: true,
		},
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

func buildMotorTestComparison(preflight *motorTestPreflight, postStop *motorTestPostStop) *motorTestComparison {
	if preflight == nil || postStop == nil {
		return nil
	}
	comparison := &motorTestComparison{
		hazardousCaptureSummary: hazardousCaptureSummary{
			Source:             "preflight to post-stop summary",
			ReadOnly:           true,
			PreflightCaptured:  preflight != nil,
			PostActionCaptured: postStop != nil,
		},
		OutputCount:          len(postStop.Outputs),
		TelemetrySampleCount: len(postStop.TelemetryRPM),
	}
	if len(postStop.Outputs) > 0 {
		comparison.MinOutput = postStop.Outputs[0]
		comparison.MaxOutput = postStop.Outputs[0]
		for _, value := range postStop.Outputs {
			if value != minMotorTestValue {
				comparison.NonMinCommandOutputs++
			}
			if value < comparison.MinOutput {
				comparison.MinOutput = value
			}
			if value > comparison.MaxOutput {
				comparison.MaxOutput = value
			}
		}
		if comparison.NonMinCommandOutputs == 0 {
			comparison.Notes = append(comparison.Notes, "all post-stop outputs are at min command or lower")
		}
	}
	if len(postStop.WarningMessages) > 0 {
		comparison.Notes = append(comparison.Notes, postStop.WarningMessages...)
	}
	return comparison
}

func motorSafetyPassed(plan motorTestPlan, name string) bool {
	for _, check := range plan.SafetyChecks {
		if check.Name == name {
			return check.Passed
		}
	}
	return false
}

func buildMotorTestAudit(plan motorTestPlan, elapsedMS int64, stopErr error, warnings []string) *motorTestAudit {
	return &motorTestAudit{
		hazardousActionAudit: hazardousActionAudit{
			Source:              "motor test apply",
			ReadOnlyEvidence:    true,
			RequestedDurationMS: plan.DurationMS,
			ElapsedDurationMS:   elapsedMS,
			StartCommand:        strings.Join(plan.CommandPreviews, "; "),
			StopCommand:         strings.Join(plan.StopCommandPreviews, "; "),
			Confirmations:       append([]string(nil), plan.RequiredConfirmations...),
			SafetyPassed:        motorSafetyPassed(plan, "props_off") && motorSafetyPassed(plan, "battery_awareness"),
			PreflightCaptured:   plan.Preflight != nil,
			PostActionCaptured:  plan.PostStop != nil,
			StopAttempted:       true,
			StopSucceeded:       stopErr == nil,
			WarningMessages:     append([]string(nil), warnings...),
		},
	}
}

func buildMotorTestPlan(motor int, all bool, motorCount int, value int, duration time.Duration, propsOff, batteryAware bool, apply bool) (motorTestPlan, error) {
	switch {
	case all && motorCount <= 0:
		return motorTestPlan{}, fmt.Errorf("--motor-count must be > 0 when --all is set")
	case all && motorCount > maxMotorTestIndex+1:
		return motorTestPlan{}, fmt.Errorf("--motor-count must be between 1 and %d", maxMotorTestIndex+1)
	case !all && (motor < 0 || motor > maxMotorTestIndex):
		return motorTestPlan{}, fmt.Errorf("--motor must be between 0 and %d", maxMotorTestIndex)
	case value < minMotorTestValue || value > maxMotorTestValue:
		return motorTestPlan{}, fmt.Errorf("--value must be between %d and %d", minMotorTestValue, maxMotorTestValue)
	case duration <= 0:
		return motorTestPlan{}, fmt.Errorf("--duration must be positive")
	case duration > 5*time.Second:
		return motorTestPlan{}, fmt.Errorf("--duration is capped at 5s")
	}

	motorIndexes := []int{motor}
	if all {
		motorIndexes = make([]int, motorCount)
		for i := range motorIndexes {
			motorIndexes[i] = i
		}
	}
	commandPreviews := make([]string, 0, len(motorIndexes))
	stopCommandPreviews := make([]string, 0, len(motorIndexes))
	for _, idx := range motorIndexes {
		commandPreviews = append(commandPreviews, fmt.Sprintf("motor %d %d", idx, value))
		stopCommandPreviews = append(stopCommandPreviews, fmt.Sprintf("motor %d %d", idx, minMotorTestValue))
	}

	recommended := []string{
		"remove all propellers",
		"secure the frame on a bench",
		"verify the intended motor index",
		"keep duration and throttle value minimal",
		"be ready to disconnect power",
	}
	if all {
		recommended = append(recommended, "verify craft motor order and numbering before enabling all-motor tests")
	}

	modeCheck := safetyCheck{Name: "dry_run_only", Passed: true, Required: true, Detail: "this command only creates a plan and never sends motor output"}
	applyMessage := "test-plan is offline only; use test-apply with all confirmations to run the bounded motor output command"
	if apply {
		modeCheck = safetyCheck{Name: "bounded_apply", Passed: true, Required: true, Detail: "this command sends bounded motor output and then attempts a stop command"}
		applyMessage = "test-apply sends motor output after all confirmations, then attempts the generated stop command"
	}

	return motorTestPlan{
		Kind:                  "motor_test",
		Applied:               false,
		Dangerous:             true,
		AllMotors:             all,
		MotorCount:            len(motorIndexes),
		MotorIndexes:          append([]int(nil), motorIndexes...),
		MotorIndex:            motor,
		Value:                 value,
		DurationMS:            duration.Milliseconds(),
		CommandPreview:        commandPreviews[0],
		StopCommandPreview:    stopCommandPreviews[0],
		CommandPreviews:       append([]string(nil), commandPreviews...),
		StopCommandPreviews:   append([]string(nil), stopCommandPreviews...),
		RequiredConfirmations: []string{"--yes", "--props-off", "--battery-aware"},
		SafetyChecks: []safetyCheck{
			{Name: "props_off", Passed: propsOff, Required: true, Detail: "propellers must be removed before any future motor output apply command"},
			{Name: "battery_awareness", Passed: batteryAware, Required: true, Detail: "operator must review power source, bench restraint, and ESC arming state"},
			modeCheck,
		},
		RecommendedPreflight: recommended,
		ApplyMessage:         applyMessage,
	}, nil
}
