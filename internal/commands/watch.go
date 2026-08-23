package commands

import (
	"context"
	"fmt"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

// Sampler reads one sample from an already-connected flight controller.
// The client argument lets callers bind a concrete connection via a closure;
// RunWatchLoop passes nil because it never opens connections itself.
type Sampler func(ctx context.Context, client *connection.Client) (any, error)

// WatchProfile selects which FC data stream a watch command samples.
type WatchProfile string

const (
	WatchProfileTelemetry WatchProfile = "telemetry"
	WatchProfileStatus    WatchProfile = "status"
	WatchProfileMotors    WatchProfile = "motors"
	WatchProfileGPS       WatchProfile = "gps"
)

// MinWatchInterval is the fastest sampling rate the watch loop allows.
const MinWatchInterval = 50 * time.Millisecond

// NormalizedWatchInterval clamps d to MinWatchInterval.
func NormalizedWatchInterval(d time.Duration) time.Duration {
	if d < MinWatchInterval {
		return MinWatchInterval
	}
	return d
}

type watchTelemetrySample struct {
	Telemetry Telemetry `json:"telemetry"`
	Warnings  []string  `json:"warnings,omitempty"`
}

type watchMotorsSample struct {
	Outputs []uint16 `json:"outputs"`
}

type watchGPSSample struct {
	Position *GPSPosition `json:"position"`
}

// ValidWatchProfiles lists accepted profile names in stable order.
func ValidWatchProfiles() []string {
	return []string{
		string(WatchProfileTelemetry),
		string(WatchProfileStatus),
		string(WatchProfileMotors),
		string(WatchProfileGPS),
	}
}

// SamplerFor returns the sampler for a watch profile.
func SamplerFor(profile WatchProfile) (Sampler, error) {
	switch profile {
	case WatchProfileTelemetry:
		return func(ctx context.Context, client *connection.Client) (any, error) {
			tel, warnings := ReadTelemetry(ctx, client)
			return watchTelemetrySample{Telemetry: tel, Warnings: warnings}, nil
		}, nil
	case WatchProfileStatus:
		return func(ctx context.Context, client *connection.Client) (any, error) {
			return ReadRuntimeStatus(ctx, client)
		}, nil
	case WatchProfileMotors:
		return func(ctx context.Context, client *connection.Client) (any, error) {
			outputs, err := readMotorOutputs(ctx, client)
			if err != nil {
				return nil, err
			}
			return watchMotorsSample{Outputs: outputs}, nil
		}, nil
	case WatchProfileGPS:
		return func(ctx context.Context, client *connection.Client) (any, error) {
			position, err := readGPSPosition(ctx, client)
			if err != nil {
				return nil, err
			}
			return watchGPSSample{Position: position}, nil
		}, nil
	default:
		return nil, fmt.Errorf("unknown watch profile %q; valid profiles: %s", string(profile), joinWatchProfiles())
	}
}

func joinWatchProfiles() string {
	out := ""
	for i, p := range ValidWatchProfiles() {
		if i > 0 {
			out += ", "
		}
		out += p
	}
	return out
}

// LoopOptions controls RunWatchLoop.
type LoopOptions struct {
	// Interval is the sampling period; values below MinWatchInterval are
	// clamped up by NormalizedWatchInterval. It describes caller intent and is
	// not enforced against externally injected ticks.
	Interval time.Duration
	// Count is the maximum number of samples; 0 means unlimited.
	Count int
	// Duration bounds total loop wall time measured from loop start;
	// 0 means no time bound. A negative Duration expires immediately.
	Duration time.Duration
}

// WatchEvent is one item emitted by RunWatchLoop. Exactly one of Value or Err
// is set: Err carries a sampling failure that did not abort the stream.
type WatchEvent struct {
	Seq   int   `json:"seq"`
	Value any   `json:"value,omitempty"`
	Err   error `json:"error,omitempty"`
}

// RunWatchLoop samples on each tick until a limit or cancellation ends it.
//
// Behavior:
//   - The first sample happens immediately, before waiting for a tick; every
//     later sample waits for one value from tick (which must be non-nil).
//   - ctx cancellation stops the loop cleanly with a nil error.
//   - opts.Count > 0 stops after that many emitted events.
//   - opts.Duration != 0 sets a wall-clock deadline from loop start; once past
//     it, no further samples are taken. The deadline fires independently of
//     tick arrivals, so the loop never overshoots it by more than one sample.
//   - Sampling errors emit a *WatchEvent with Err set and the loop CONTINUES;
//     only emit failures abort the loop (returned wrapped).
func RunWatchLoop(ctx context.Context, opts LoopOptions, tick <-chan time.Time, sample Sampler, emit func(any) error) error {
	if tick == nil {
		return fmt.Errorf("watch loop requires a tick channel")
	}
	if sample == nil {
		return fmt.Errorf("watch loop requires a sampler")
	}
	if emit == nil {
		return fmt.Errorf("watch loop requires an emit function")
	}

	var deadline time.Time
	if opts.Duration != 0 {
		deadline = time.Now().Add(opts.Duration)
	}
	expired := func() bool {
		return !deadline.IsZero() && !time.Now().Before(deadline)
	}

	seq := 0
	sampleOnce := func() error {
		seq++
		value, err := sample(ctx, nil)
		if err != nil {
			if ctx.Err() != nil {
				// Cancellation raced the sample; end cleanly instead of
				// emitting a spurious error event.
				return nil
			}
			return emit(&WatchEvent{Seq: seq, Err: err})
		}
		return emit(&WatchEvent{Seq: seq, Value: value})
	}

	var deadlineTimer *time.Timer
	if !deadline.IsZero() {
		deadlineTimer = time.NewTimer(time.Until(deadline))
		defer deadlineTimer.Stop()
	}

	for {
		if ctx.Err() != nil || expired() || (opts.Count > 0 && seq >= opts.Count) {
			return nil
		}
		if err := sampleOnce(); err != nil {
			return fmt.Errorf("watch emit: %w", err)
		}
		if opts.Count > 0 && seq >= opts.Count {
			return nil
		}
		select {
		case <-ctx.Done():
			return nil
		case <-deadlineChan(deadlineTimer):
			return nil
		case <-tick:
		}
	}
}

// deadlineChan returns the timer's channel, or a nil channel that blocks
// forever when the loop runs unbounded.
func deadlineChan(timer *time.Timer) <-chan time.Time {
	if timer == nil {
		return nil
	}
	return timer.C
}
