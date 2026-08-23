package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/hajekt2/betaflight-cli/internal/connection"
)

func TestSamplerForUnknownProfile(t *testing.T) {
	sampler, err := SamplerFor(WatchProfile("bogus"))
	if err == nil {
		t.Fatal("expected error for unknown profile")
	}
	if sampler != nil {
		t.Fatal("expected nil sampler for unknown profile")
	}
	if want := `unknown watch profile "bogus"`; !strings.Contains(err.Error(), want) {
		t.Fatalf("err = %q, want prefix %q", err.Error(), want)
	}
}

func TestSamplerForKnownProfiles(t *testing.T) {
	for _, profile := range ValidWatchProfiles() {
		sampler, err := SamplerFor(WatchProfile(profile))
		if err != nil {
			t.Fatalf("profile %s: unexpected error: %v", profile, err)
		}
		if sampler == nil {
			t.Fatalf("profile %s: expected sampler", profile)
		}
	}
}

func TestNormalizedWatchInterval(t *testing.T) {
	cases := []struct {
		in   time.Duration
		want time.Duration
	}{
		{0, MinWatchInterval},
		{10 * time.Millisecond, MinWatchInterval},
		{MinWatchInterval, MinWatchInterval},
		{250 * time.Millisecond, 250 * time.Millisecond},
	}
	for _, tc := range cases {
		if got := NormalizedWatchInterval(tc.in); got != tc.want {
			t.Fatalf("NormalizedWatchInterval(%v) = %v, want %v", tc.in, got, tc.want)
		}
	}
}

// seqSampler produces "value-N" per call, failing on sequence numbers in failAt.
type seqSampler struct {
	calls  int
	failAt map[int]bool
}

func (s *seqSampler) sample(_ context.Context, _ *connection.Client) (any, error) {
	s.calls++
	seq := s.calls
	if s.failAt[seq] {
		return nil, fmt.Errorf("sample failure %d", seq)
	}
	return fmt.Sprintf("value-%d", seq), nil
}

func collectEvents(t *testing.T, events *[]*WatchEvent) func(any) error {
	t.Helper()
	return func(item any) error {
		event, ok := item.(*WatchEvent)
		if !ok {
			t.Fatalf("emit got %T, want *WatchEvent", item)
		}
		*events = append(*events, event)
		return nil
	}
}

func TestRunWatchLoopCountStopsAfterN(t *testing.T) {
	ctx := context.Background()
	s := &seqSampler{}
	var events []*WatchEvent

	ticks := make(chan time.Time, 8)
	ticks <- time.Now()
	ticks <- time.Now()

	err := RunWatchLoop(ctx, LoopOptions{Count: 3}, ticks, s.sample, collectEvents(t, &events))
	if err != nil {
		t.Fatalf("RunWatchLoop = %v", err)
	}
	if len(events) != 3 {
		t.Fatalf("events = %d (%+v), want 3", len(events), events)
	}
	for i, ev := range events {
		if ev.Seq != i+1 {
			t.Fatalf("events[%d].Seq = %d, want %d", i, ev.Seq, i+1)
		}
		if ev.Err != nil {
			t.Fatalf("events[%d].Err = %v, want nil", i, ev.Err)
		}
		if ev.Value != fmt.Sprintf("value-%d", i+1) {
			t.Fatalf("events[%d].Value = %v", i, ev.Value)
		}
	}
	if s.calls != 3 {
		t.Fatalf("sampler called %d times, want 3", s.calls)
	}
}

func TestRunWatchLoopSamplingErrorContinues(t *testing.T) {
	ctx := context.Background()
	s := &seqSampler{failAt: map[int]bool{2: true, 4: true}}
	var events []*WatchEvent

	ticks := make(chan time.Time, 8)
	ticks <- time.Now()
	ticks <- time.Now()
	ticks <- time.Now()
	ticks <- time.Now()

	err := RunWatchLoop(ctx, LoopOptions{Count: 5}, ticks, s.sample, collectEvents(t, &events))
	if err != nil {
		t.Fatalf("RunWatchLoop = %v", err)
	}
	if len(events) != 5 {
		t.Fatalf("events = %d, want 5 (errors must not abort the stream)", len(events))
	}
	for _, seq := range []int{2, 4} {
		ev := events[seq-1]
		if ev.Err == nil || ev.Value != nil {
			t.Fatalf("events[%d] = %+v, want Err set and Value nil", seq-1, ev)
		}
		if want := fmt.Sprintf("sample failure %d", seq); !strings.Contains(ev.Err.Error(), want) {
			t.Fatalf("events[%d].Err = %v, want containing %q", seq-1, ev.Err, want)
		}
	}
	for _, seq := range []int{1, 3, 5} {
		ev := events[seq-1]
		if ev.Err != nil || ev.Value == nil {
			t.Fatalf("events[%d] = %+v, want successful sample", seq-1, ev)
		}
	}
}

func TestRunWatchLoopCancelEndsCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	s := &seqSampler{}
	firstEmitted := make(chan struct{})
	emitted := 0
	emit := func(item any) error {
		emitted++
		if emitted == 1 {
			close(firstEmitted)
		}
		return nil
	}

	ticks := make(chan time.Time)
	done := make(chan error, 1)
	go func() {
		done <- RunWatchLoop(ctx, LoopOptions{}, ticks, s.sample, emit)
	}()

	// The first sample is immediate; once it lands, cancel while the loop
	// blocks waiting for tick 2.
	select {
	case <-firstEmitted:
	case <-time.After(time.Second):
		t.Fatal("first sample never emitted")
	}
	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RunWatchLoop after cancel = %v, want nil", err)
		}
	case <-time.After(time.Second):
		t.Fatal("RunWatchLoop did not return after cancellation")
	}
	if emitted != 1 {
		t.Fatalf("emitted %d events, want exactly 1", emitted)
	}
}

func TestRunWatchLoopExpiredDurationTakesNoSamples(t *testing.T) {
	ctx := context.Background()
	s := &seqSampler{}
	var events []*WatchEvent

	err := RunWatchLoop(ctx, LoopOptions{Duration: -time.Second}, make(chan time.Time), s.sample, collectEvents(t, &events))
	if err != nil {
		t.Fatalf("RunWatchLoop = %v", err)
	}
	if len(events) != 0 || s.calls != 0 {
		t.Fatalf("expired duration sampled: events=%d calls=%d, want 0/0", len(events), s.calls)
	}
}

func TestRunWatchLoopEmitErrorAborts(t *testing.T) {
	ctx := context.Background()
	s := &seqSampler{}
	emitErr := errors.New("sink closed")
	calls := 0
	emit := func(item any) error {
		calls++
		if calls == 2 {
			return emitErr
		}
		return nil
	}

	ticks := make(chan time.Time, 4)
	ticks <- time.Now()
	err := RunWatchLoop(ctx, LoopOptions{Count: 5}, ticks, s.sample, emit)
	if !errors.Is(err, emitErr) {
		t.Fatalf("RunWatchLoop = %v, want wrapped emit error", err)
	}
	if calls != 2 {
		t.Fatalf("emit called %d times, want 2", calls)
	}
}

func TestRunWatchLoopRequiresTickAndSampler(t *testing.T) {
	ctx := context.Background()
	s := &seqSampler{}
	noOpEmit := func(any) error { return nil }
	if err := RunWatchLoop(ctx, LoopOptions{}, nil, s.sample, noOpEmit); err == nil {
		t.Fatal("nil tick channel must be rejected")
	}
	if err := RunWatchLoop(ctx, LoopOptions{}, make(chan time.Time, 1), nil, noOpEmit); err == nil {
		t.Fatal("nil sampler must be rejected")
	}
	if err := RunWatchLoop(ctx, LoopOptions{}, make(chan time.Time, 1), s.sample, nil); err == nil {
		t.Fatal("nil emit must be rejected")
	}
}
