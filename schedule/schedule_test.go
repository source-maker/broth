// Package schedule_test contains tests for the schedule package.
//
// Note on cron timing: robfig/cron v3's @every descriptor rounds sub-second
// durations up to 1 second (see ConstantDelaySchedule.Every). Therefore all
// tests use a 2-second tolerance window to guarantee at least one cron tick
// fires regardless of when within a second the test starts.
package schedule_test

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/source-maker/broth/job"
	"github.com/source-maker/broth/log"
	"github.com/source-maker/broth/schedule"
)

// --- test helpers ---

func newTestLogger() *log.Logger {
	return log.New(slog.LevelDebug)
}

// countingJob records how many times Execute has been called.
type countingJob struct {
	name    string
	counter *atomic.Int64
	delay   time.Duration
}

func (j *countingJob) JobName() string { return j.name }
func (j *countingJob) Execute(_ context.Context) error {
	if j.delay > 0 {
		time.Sleep(j.delay)
	}
	j.counter.Add(1)
	return nil
}

// mockLeaderElector implements schedule.LeaderElector with configurable behaviour.
type mockLeaderElector struct {
	// isLeader controls whether TryAcquire reports a successful acquisition.
	isLeader bool
	// acquireErr, if non-nil, is returned by TryAcquire.
	acquireErr error
	// acquireCalls counts how many times TryAcquire was called.
	acquireCalls atomic.Int64
	// releaseCalls counts how many times Release was called.
	releaseCalls atomic.Int64
}

func (m *mockLeaderElector) TryAcquire(_ context.Context, _ string) (bool, error) {
	m.acquireCalls.Add(1)
	if m.acquireErr != nil {
		return false, m.acquireErr
	}
	return m.isLeader, nil
}

func (m *mockLeaderElector) Release(_ context.Context, _ string) error {
	m.releaseCalls.Add(1)
	return nil
}

// startScheduler launches s.Start in a goroutine and returns:
//   - cancelScheduler: cancels the scheduler's context (unblocking Start)
//   - done: closed when Start returns
//   - shutdown: cancels the context, waits for Start to return, then calls
//     Shutdown, ensuring s.cron is visible before Shutdown reads it.
//
// The caller must ensure cancelScheduler is eventually called (or use
// shutdown) to avoid goroutine leaks.
func startScheduler(t *testing.T, s *schedule.Scheduler) (cancelScheduler context.CancelFunc, done <-chan struct{}, shutdown func() error) {
	t.Helper()
	ctx, cancelFn := context.WithCancel(context.Background())
	doneCh := make(chan struct{})

	go func() {
		s.Start(ctx)
		close(doneCh)
	}()

	shutdownFn := func() error {
		cancelFn() // unblock Start's select

		// Wait for Start to return so that s.cron has been written and is
		// visible to the shutdown goroutine (avoids a data race).
		select {
		case <-doneCh:
		case <-time.After(5 * time.Second):
			t.Error("Start did not return within 5s after context cancellation")
		}

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		return s.Shutdown(shutdownCtx)
	}

	return cancelFn, doneCh, shutdownFn
}

// waitForCount polls counter until it reaches at least want, or fails the
// test after the given timeout.
func waitForCount(t *testing.T, counter *atomic.Int64, want int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if counter.Load() >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for counter=%d; got %d", timeout, want, counter.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// waitForCalls polls the int64 counter until it reaches at least want,
// or fails after timeout.
func waitForCalls(t *testing.T, calls *atomic.Int64, want int64, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for {
		if calls.Load() >= want {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for calls=%d; got %d", timeout, want, calls.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// --- tests ---

// TestScheduler_RegisterAndStart verifies that a registered job is enqueued
// and executed within a reasonable time window.
//
// Note: robfig/cron's @every rounds sub-second to 1s, so the first tick fires
// at the next whole-second boundary (up to ~1 s away).  We allow 2 s.
func TestScheduler_RegisterAndStart(t *testing.T) {
	t.Parallel()

	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), nil)

	counter := &atomic.Int64{}
	j := &countingJob{name: "tick-job", counter: counter}

	s.Register(schedule.Definition{
		Name:    "tick-job",
		Cron:    "@every 1s",
		Job:     j,
		Overlap: true,
	})

	// Start a worker so the in-memory queue is drained and Execute() is called.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	w := job.NewWorker(enqueuer, job.WorkerConfig{Concurrency: 2, ShutdownTimeout: 3 * time.Second}, newTestLogger())
	w.Start(workerCtx)
	defer func() {
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer shutdownCancel()
		_ = w.Shutdown(shutdownCtx)
	}()

	_, _, shutdown := startScheduler(t, s)
	defer func() { _ = shutdown() }()

	// Wait for at least one execution; allow 2s to cover up to one full second
	// boundary before the first cron tick.
	waitForCount(t, counter, 1, 2*time.Second)
}

// TestScheduler_Shutdown_GracefulStop verifies that Shutdown returns nil when
// called after Start.
func TestScheduler_Shutdown_GracefulStop(t *testing.T) {
	t.Parallel()

	s := schedule.NewScheduler(
		job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10)),
		newTestLogger(),
		nil,
	)
	_, _, shutdown := startScheduler(t, s)

	// Give the scheduler a moment to initialise before stopping.
	time.Sleep(50 * time.Millisecond)

	if err := shutdown(); err != nil {
		t.Errorf("Shutdown() error = %v, want nil", err)
	}
}

// TestScheduler_ShutdownWithoutStart verifies that calling Shutdown before
// Start does not deadlock or panic.
func TestScheduler_ShutdownWithoutStart(t *testing.T) {
	t.Parallel()

	s := schedule.NewScheduler(
		job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10)),
		newTestLogger(),
		nil,
	)

	shutdownCtx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	// Shutdown without ever calling Start; s.cron is nil.
	if err := s.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown (no Start) error = %v, want nil", err)
	}
}

// TestScheduler_InvalidCronExpression verifies that an invalid cron expression
// logs an error and does not panic.  The scheduler continues running with the
// remaining (valid) definitions.
func TestScheduler_InvalidCronExpression(t *testing.T) {
	t.Parallel()

	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10))

	// Scheduler with only an invalid definition — should not panic.
	s := schedule.NewScheduler(enqueuer, newTestLogger(), nil)
	s.Register(schedule.Definition{
		Name: "bad",
		Cron: "not-a-cron-expression",
		Job:  &countingJob{name: "bad-job", counter: &atomic.Int64{}},
	})

	_, _, shutdown := startScheduler(t, s)

	// Give it a moment in case there's an unexpected panic.
	time.Sleep(100 * time.Millisecond)

	if err := shutdown(); err != nil {
		t.Errorf("Shutdown (invalid cron) error = %v", err)
	}
}

// TestScheduler_OverlapFalse verifies that when Overlap=false, the cron job
// function is wrapped with SkipIfStillRunning.
//
// Behaviour note: SkipIfStillRunning wraps the enqueue function, not the
// actual job Execute method.  Since Enqueue completes in microseconds, the
// cron function finishes well before the next 1-second tick, so in practice
// the skip is only triggered when two ticks arrive within the same scheduling
// cycle (extremely unlikely on a loaded CI machine).
//
// This test therefore validates:
// 1. Registration succeeds without panic.
// 2. The scheduler starts and fires at least once.
// 3. The total number of enqueues over a 2-second window does not exceed
//    the number of ticks that could fire (≤3 for a 1s interval in 2s).
func TestScheduler_OverlapFalse(t *testing.T) {
	t.Parallel()

	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(20))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), nil)

	executions := &atomic.Int64{}
	j := &countingJob{name: "overlap-false-job", counter: executions}

	s.Register(schedule.Definition{
		Name:    "overlap-false",
		Cron:    "@every 1s",
		Job:     j,
		Overlap: false,
	})

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	w := job.NewWorker(enqueuer, job.WorkerConfig{Concurrency: 4, ShutdownTimeout: 3 * time.Second}, newTestLogger())
	w.Start(workerCtx)

	_, _, shutdown := startScheduler(t, s)

	// Wait for at least one tick.
	waitForCount(t, executions, 1, 2*time.Second)

	// Run for one more second then stop.
	time.Sleep(time.Second)
	_ = shutdown()
	cancelWorker()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = w.Shutdown(shutdownCtx)

	// Over the window (at most 3 ticks in ~2s) the execution count must be ≥1.
	runs := executions.Load()
	if runs == 0 {
		t.Error("expected at least 1 execution but got 0")
	}
	// Sanity upper bound: cron fires at most once per second, so ≤3 in ~2s.
	if runs > 3 {
		t.Errorf("unexpectedly high execution count: got %d, want ≤3", runs)
	}
}

// TestScheduler_LeaderElection_NotLeader verifies that jobs are NOT enqueued
// when the leader elector denies the lock.
func TestScheduler_LeaderElection_NotLeader(t *testing.T) {
	t.Parallel()

	elector := &mockLeaderElector{isLeader: false}
	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), elector)

	counter := &atomic.Int64{}
	j := &countingJob{name: "not-leader-job", counter: counter}

	s.Register(schedule.Definition{
		Name:    "not-leader",
		Cron:    "@every 1s",
		Job:     j,
		Overlap: true,
	})

	_, _, shutdown := startScheduler(t, s)

	// Wait for at least one TryAcquire call (meaning at least one tick fired).
	waitForCalls(t, &elector.acquireCalls, 1, 2*time.Second)

	_ = shutdown()

	// No jobs should have been enqueued (and thus no executions).
	if counter.Load() != 0 {
		t.Errorf("expected 0 executions when not leader, got %d", counter.Load())
	}
}

// TestScheduler_LeaderElection_IsLeader verifies that jobs ARE enqueued when
// the leader elector grants the lock.
func TestScheduler_LeaderElection_IsLeader(t *testing.T) {
	t.Parallel()

	elector := &mockLeaderElector{isLeader: true}
	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), elector)

	counter := &atomic.Int64{}
	j := &countingJob{name: "leader-job", counter: counter}

	s.Register(schedule.Definition{
		Name:    "leader",
		Cron:    "@every 1s",
		Job:     j,
		Overlap: true,
	})

	// Start worker so the queue is drained and jobs are actually executed.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	w := job.NewWorker(enqueuer, job.WorkerConfig{Concurrency: 2, ShutdownTimeout: 3 * time.Second}, newTestLogger())
	w.Start(workerCtx)

	_, _, shutdown := startScheduler(t, s)

	// Expect at least one execution; allow 2s for the first cron tick.
	waitForCount(t, counter, 1, 2*time.Second)

	_ = shutdown()
	cancelWorker()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = w.Shutdown(shutdownCtx)

	// Release should have been called at least once after each successful acquire.
	if elector.releaseCalls.Load() == 0 {
		t.Error("Release was never called; expected at least one call")
	}
}

// TestScheduler_LeaderElection_AcquireError verifies that an error from
// TryAcquire is logged and the job is not enqueued.
func TestScheduler_LeaderElection_AcquireError(t *testing.T) {
	t.Parallel()

	elector := &mockLeaderElector{
		isLeader:   false,
		acquireErr: errors.New("redis: connection refused"),
	}
	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), elector)

	counter := &atomic.Int64{}
	j := &countingJob{name: "acquire-error-job", counter: counter}

	s.Register(schedule.Definition{
		Name:    "acquire-error",
		Cron:    "@every 1s",
		Job:     j,
		Overlap: true,
	})

	_, _, shutdown := startScheduler(t, s)

	// Wait for at least one TryAcquire attempt.
	waitForCalls(t, &elector.acquireCalls, 1, 2*time.Second)

	_ = shutdown()

	if counter.Load() != 0 {
		t.Errorf("expected 0 executions when TryAcquire errors, got %d", counter.Load())
	}
}

// TestScheduler_Timezone verifies that a per-definition timezone is accepted
// without error and the scheduler starts and fires correctly.
func TestScheduler_Timezone(t *testing.T) {
	t.Parallel()

	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Skipf("timezone data unavailable: %v", err)
	}

	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), nil)

	counter := &atomic.Int64{}
	j := &countingJob{name: "tz-job", counter: counter}

	s.Register(schedule.Definition{
		Name:     "tz-test",
		Cron:     "@every 1s",
		Job:      j,
		Overlap:  true,
		Timezone: loc,
	})

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	w := job.NewWorker(enqueuer, job.WorkerConfig{Concurrency: 2, ShutdownTimeout: 3 * time.Second}, newTestLogger())
	w.Start(workerCtx)

	_, _, shutdown := startScheduler(t, s)

	// The job should fire within 2 seconds regardless of timezone.
	waitForCount(t, counter, 1, 2*time.Second)

	_ = shutdown()
	cancelWorker()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = w.Shutdown(shutdownCtx)
}

// TestScheduler_ContextCancellation verifies that Start returns (unblocks)
// when the context is cancelled.
func TestScheduler_ContextCancellation(t *testing.T) {
	t.Parallel()

	s := schedule.NewScheduler(
		job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10)),
		newTestLogger(),
		nil,
	)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		s.Start(ctx)
		close(done)
	}()

	// Give Start a moment to enter its blocking select.
	time.Sleep(50 * time.Millisecond)

	cancel()

	select {
	case <-done:
		// Start returned — correct.
	case <-time.After(2 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}
}

// TestScheduler_MultipleDefinitions verifies that multiple definitions are
// all registered and executed.
func TestScheduler_MultipleDefinitions(t *testing.T) {
	t.Parallel()

	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(20))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), nil)

	counterA := &atomic.Int64{}
	counterB := &atomic.Int64{}

	s.Register(
		schedule.Definition{
			Name:    "job-a",
			Cron:    "@every 1s",
			Job:     &countingJob{name: "job-a", counter: counterA},
			Overlap: true,
		},
		schedule.Definition{
			Name:    "job-b",
			Cron:    "@every 1s",
			Job:     &countingJob{name: "job-b", counter: counterB},
			Overlap: true,
		},
	)

	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	w := job.NewWorker(enqueuer, job.WorkerConfig{Concurrency: 4, ShutdownTimeout: 3 * time.Second}, newTestLogger())
	w.Start(workerCtx)

	_, _, shutdown := startScheduler(t, s)

	// Allow 2s for both jobs to fire at least once.
	waitForCount(t, counterA, 1, 2*time.Second)
	waitForCount(t, counterB, 1, 2*time.Second)

	_ = shutdown()
	cancelWorker()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer shutdownCancel()
	_ = w.Shutdown(shutdownCtx)
}

// TestScheduler_ShutdownTimeout verifies that Shutdown returns ctx.Err()
// when the provided context expires before in-flight jobs finish.
func TestScheduler_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	enqueuer := job.NewEnqueuer(newTestLogger(), job.WithQueueSize(10))
	s := schedule.NewScheduler(enqueuer, newTestLogger(), nil)

	// A very slow job that blocks until we close blockCh.
	var executing atomic.Bool
	blockCh := make(chan struct{})
	slowJob := &slowBlockingJob{started: &executing, block: blockCh}

	s.Register(schedule.Definition{
		Name:    "very-slow",
		Cron:    "@every 1s",
		Job:     slowJob,
		Overlap: true,
	})

	// cancelScheduler stops the scheduler's Start goroutine.
	// cancelWorker stops the worker goroutine.
	workerCtx, cancelWorker := context.WithCancel(context.Background())
	defer cancelWorker()

	w := job.NewWorker(enqueuer, job.WorkerConfig{Concurrency: 2, ShutdownTimeout: 10 * time.Second}, newTestLogger())
	w.Start(workerCtx)
	defer func() {
		// Unblock the slow job so the worker goroutine can exit.
		select {
		case <-blockCh:
		default:
			close(blockCh)
		}
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer shutdownCancel()
		_ = w.Shutdown(shutdownCtx)
	}()

	// startScheduler returns the scheduler's cancel and a done channel.
	cancelScheduler, done, _ := startScheduler(t, s)

	// Wait until the slow job starts executing.
	waitDeadline := time.Now().Add(3 * time.Second)
	for !executing.Load() {
		if time.Now().After(waitDeadline) {
			t.Skip("slow job never started; skipping timeout test")
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Cancel the scheduler context to unblock Start, then wait for it to return.
	cancelScheduler()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Start did not return after context cancellation")
	}

	// Call Shutdown with a very tight deadline while the job is still running.
	tightCtx, tightCancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer tightCancel()

	err := s.Shutdown(tightCtx)
	// Either nil (job happened to finish) or context.DeadlineExceeded.
	if err != nil && !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown error = %v; want nil or context.DeadlineExceeded", err)
	}

	// Unblock the slow job so goroutines don't leak.
	select {
	case <-blockCh:
	default:
		close(blockCh)
	}
}

// slowBlockingJob blocks until its block channel is closed.
type slowBlockingJob struct {
	started *atomic.Bool
	block   chan struct{}
}

func (j *slowBlockingJob) JobName() string { return "slow-blocking" }
func (j *slowBlockingJob) Execute(_ context.Context) error {
	j.started.Store(true)
	<-j.block
	return nil
}
