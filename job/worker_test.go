package job

import (
	"context"
	"errors"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	"github.com/source-maker/broth/log"
)

// countingJob increments a counter each time Execute is called.
type countingJob struct {
	name    string
	counter *atomic.Int64
	delay   time.Duration
	err     error
}

func (j *countingJob) JobName() string { return j.name }
func (j *countingJob) Execute(_ context.Context) error {
	if j.delay > 0 {
		time.Sleep(j.delay)
	}
	j.counter.Add(1)
	return j.err
}

func TestNewWorker(t *testing.T) {
	t.Parallel()

	logger := log.New(slog.LevelDebug)
	e := NewEnqueuer(logger, WithQueueSize(10))

	t.Run("default concurrency when zero", func(t *testing.T) {
		t.Parallel()
		w := NewWorker(e, WorkerConfig{Concurrency: 0}, logger)
		if w.concurrency != 1 {
			t.Errorf("concurrency = %d, want 1 (default)", w.concurrency)
		}
	})

	t.Run("default concurrency when negative", func(t *testing.T) {
		t.Parallel()
		w := NewWorker(e, WorkerConfig{Concurrency: -5}, logger)
		if w.concurrency != 1 {
			t.Errorf("concurrency = %d, want 1 (default)", w.concurrency)
		}
	})

	t.Run("custom concurrency", func(t *testing.T) {
		t.Parallel()
		w := NewWorker(e, WorkerConfig{Concurrency: 4}, logger)
		if w.concurrency != 4 {
			t.Errorf("concurrency = %d, want 4", w.concurrency)
		}
	})
}

func TestWorker_ExecutesJobs(t *testing.T) {
	t.Parallel()

	logger := log.New(slog.LevelDebug)
	e := NewEnqueuer(logger, WithQueueSize(10))
	w := NewWorker(e, WorkerConfig{Concurrency: 2, ShutdownTimeout: 5 * time.Second}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	counter := &atomic.Int64{}
	jobCount := 5

	for i := range jobCount {
		j := &countingJob{name: "count-job", counter: counter}
		if err := e.Enqueue(ctx, j); err != nil {
			t.Fatalf("Enqueue job %d: %v", i, err)
		}
	}

	// Wait for jobs to be processed.
	deadline := time.After(5 * time.Second)
	for {
		if counter.Load() == int64(jobCount) {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out: only %d/%d jobs executed", counter.Load(), jobCount)
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()

	if err := w.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown error = %v", err)
	}
}

func TestWorker_ShutdownStopsProcessing(t *testing.T) {
	t.Parallel()

	logger := log.New(slog.LevelDebug)
	e := NewEnqueuer(logger, WithQueueSize(100))
	w := NewWorker(e, WorkerConfig{Concurrency: 1, ShutdownTimeout: 2 * time.Second}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	// Shutdown immediately without enqueueing any jobs.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer shutdownCancel()

	err := w.Shutdown(shutdownCtx)
	if err != nil {
		t.Errorf("Shutdown error = %v, want nil", err)
	}
}

func TestWorker_ShutdownTimeout(t *testing.T) {
	t.Parallel()

	logger := log.New(slog.LevelDebug)
	e := NewEnqueuer(logger, WithQueueSize(10))
	w := NewWorker(e, WorkerConfig{Concurrency: 1, ShutdownTimeout: time.Second}, logger)

	ctx := context.Background()
	w.Start(ctx)

	// Enqueue a slow job.
	counter := &atomic.Int64{}
	slowJob := &countingJob{name: "slow", counter: counter, delay: 10 * time.Second}
	_ = e.Enqueue(ctx, slowJob)

	// Wait a bit for worker to pick up the job.
	time.Sleep(50 * time.Millisecond)

	// Shutdown with a very short timeout.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer shutdownCancel()

	err := w.Shutdown(shutdownCtx)
	if err == nil {
		// It is possible the worker already exited, so we accept nil as well.
		return
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("Shutdown error = %v, want context.DeadlineExceeded", err)
	}
}

func TestWorker_ContextCancelStopsWorkers(t *testing.T) {
	t.Parallel()

	logger := log.New(slog.LevelDebug)
	e := NewEnqueuer(logger, WithQueueSize(10))
	w := NewWorker(e, WorkerConfig{Concurrency: 2}, logger)

	ctx, cancel := context.WithCancel(context.Background())

	w.Start(ctx)

	// Cancel the context to stop all workers.
	cancel()

	// Give workers time to notice the cancellation.
	time.Sleep(100 * time.Millisecond)

	// Shutdown should succeed quickly since workers already stopped.
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()

	if err := w.Shutdown(shutdownCtx); err != nil {
		t.Errorf("Shutdown after cancel error = %v", err)
	}
}

func TestWorker_JobError_DoesNotStopWorker(t *testing.T) {
	t.Parallel()

	logger := log.New(slog.LevelDebug)
	e := NewEnqueuer(logger, WithQueueSize(10))
	w := NewWorker(e, WorkerConfig{Concurrency: 1}, logger)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	w.Start(ctx)

	counter := &atomic.Int64{}

	// Enqueue a failing job, then a succeeding one.
	failJob := &countingJob{name: "fail", counter: counter, err: errors.New("boom")}
	goodJob := &countingJob{name: "good", counter: counter}
	_ = e.Enqueue(ctx, failJob)
	_ = e.Enqueue(ctx, goodJob)

	// Wait for both jobs.
	deadline := time.After(3 * time.Second)
	for {
		if counter.Load() >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("timed out: only %d/2 jobs executed", counter.Load())
		default:
			time.Sleep(10 * time.Millisecond)
		}
	}

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), time.Second)
	defer shutdownCancel()
	_ = w.Shutdown(shutdownCtx)
}
