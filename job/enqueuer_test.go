package job

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/source-maker/broth/log"
)

// testJob is a minimal Job implementation for testing.
type testJob struct {
	name string
	err  error
}

func (j *testJob) JobName() string              { return j.name }
func (j *testJob) Execute(_ context.Context) error { return j.err }

// mockBackend records calls to Enqueue for assertion.
type mockBackend struct {
	enqueuedJobs []Job
	enqueueErr   error
}

func (b *mockBackend) Enqueue(_ context.Context, j Job, _ jobOptions) error {
	if b.enqueueErr != nil {
		return b.enqueueErr
	}
	b.enqueuedJobs = append(b.enqueuedJobs, j)
	return nil
}

func (b *mockBackend) Fetch(_ context.Context, _ []string, _ string) (*PersistedJob, error) {
	return nil, nil
}
func (b *mockBackend) Complete(_ context.Context, _ int64) error          { return nil }
func (b *mockBackend) Fail(_ context.Context, _ int64, _ error) error     { return nil }
func (b *mockBackend) Dead(_ context.Context, _ int64, _ error) error     { return nil }
func (b *mockBackend) RetryDead(_ context.Context, _ int64) error         { return nil }
func (b *mockBackend) Stats(_ context.Context) (*QueueStats, error)       { return nil, nil }
func (b *mockBackend) Cleanup(_ context.Context, _ time.Time) (int64, error) { return 0, nil }

func newTestLogger() *log.Logger {
	return log.New(slog.LevelDebug)
}

func TestNewEnqueuer_Defaults(t *testing.T) {
	t.Parallel()

	e := NewEnqueuer(newTestLogger())
	if e == nil {
		t.Fatal("NewEnqueuer returned nil")
	}
	// Default queue buffer should be 1000.
	if cap(e.memQueue) != 1000 {
		t.Errorf("default memQueue cap = %d, want 1000", cap(e.memQueue))
	}
	if e.backend != nil {
		t.Error("default backend should be nil")
	}
}

func TestNewEnqueuer_WithQueueSize(t *testing.T) {
	t.Parallel()

	e := NewEnqueuer(newTestLogger(), WithQueueSize(50))
	if cap(e.memQueue) != 50 {
		t.Errorf("memQueue cap = %d, want 50", cap(e.memQueue))
	}
}

func TestNewEnqueuer_WithBackend(t *testing.T) {
	t.Parallel()

	b := &mockBackend{}
	e := NewEnqueuer(newTestLogger(), WithBackend(b))
	if e.backend == nil {
		t.Fatal("backend should not be nil after WithBackend")
	}
}

func TestEnqueue_InMemory(t *testing.T) {
	t.Parallel()

	e := NewEnqueuer(newTestLogger(), WithQueueSize(10))
	j := &testJob{name: "send-email"}

	err := e.Enqueue(context.Background(), j)
	if err != nil {
		t.Fatalf("Enqueue() error = %v, want nil", err)
	}

	// Read from the queue channel.
	select {
	case env := <-e.Queue():
		if env.job.JobName() != "send-email" {
			t.Errorf("dequeued job name = %q, want %q", env.job.JobName(), "send-email")
		}
		if env.enqueuedAt.IsZero() {
			t.Error("enqueuedAt should not be zero")
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for job on queue")
	}
}

func TestEnqueue_InMemory_ContextCanceled(t *testing.T) {
	t.Parallel()

	// Buffer of 1, fill it up.
	e := NewEnqueuer(newTestLogger(), WithQueueSize(1))
	_ = e.Enqueue(context.Background(), &testJob{name: "first"})

	// Now the buffer is full. Use a canceled context.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := e.Enqueue(ctx, &testJob{name: "second"})
	if err == nil {
		t.Fatal("Enqueue with full buffer and canceled context should return error")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
}

func TestEnqueue_Persist_NoBackend(t *testing.T) {
	t.Parallel()

	e := NewEnqueuer(newTestLogger()) // no backend
	j := &testJob{name: "important-job"}

	err := e.Enqueue(context.Background(), j, WithPersist())
	if err == nil {
		t.Fatal("Enqueue with WithPersist and no backend should return error")
	}
	if got := err.Error(); got != "job: no persistent backend configured for job important-job" {
		t.Errorf("error message = %q", got)
	}
}

func TestEnqueue_Persist_WithBackend(t *testing.T) {
	t.Parallel()

	b := &mockBackend{}
	e := NewEnqueuer(newTestLogger(), WithBackend(b))
	j := &testJob{name: "persist-me"}

	err := e.Enqueue(context.Background(), j, WithPersist())
	if err != nil {
		t.Fatalf("Enqueue(persist) error = %v, want nil", err)
	}
	if len(b.enqueuedJobs) != 1 {
		t.Fatalf("backend received %d jobs, want 1", len(b.enqueuedJobs))
	}
	if b.enqueuedJobs[0].JobName() != "persist-me" {
		t.Errorf("backend job name = %q, want %q", b.enqueuedJobs[0].JobName(), "persist-me")
	}
}

func TestEnqueue_Persist_BackendError(t *testing.T) {
	t.Parallel()

	b := &mockBackend{enqueueErr: errors.New("db connection lost")}
	e := NewEnqueuer(newTestLogger(), WithBackend(b))

	err := e.Enqueue(context.Background(), &testJob{name: "fail-job"}, WithPersist())
	if err == nil {
		t.Fatal("expected backend error to propagate")
	}
	if err.Error() != "db connection lost" {
		t.Errorf("error = %q, want %q", err.Error(), "db connection lost")
	}
}

func TestQueue_ReturnsReadOnlyChannel(t *testing.T) {
	t.Parallel()

	e := NewEnqueuer(newTestLogger())
	ch := e.Queue()
	if ch == nil {
		t.Fatal("Queue() returned nil channel")
	}
}
