package job

import (
	"context"
	"time"
)

// Backend defines the interface for persistent job storage.
type Backend interface {
	// Enqueue stores a job for later execution.
	Enqueue(ctx context.Context, j Job, opts jobOptions) error

	// Fetch retrieves the next available job from the given queues.
	Fetch(ctx context.Context, queues []string, workerID string) (*PersistedJob, error)

	// Complete marks a job as successfully completed.
	Complete(ctx context.Context, jobID int64) error

	// Fail records a job failure for retry.
	Fail(ctx context.Context, jobID int64, err error) error

	// Dead moves a job to the dead-letter queue after max retries.
	Dead(ctx context.Context, jobID int64, err error) error

	// RetryDead re-enqueues a dead job for another attempt.
	RetryDead(ctx context.Context, jobID int64) error

	// Stats returns queue statistics.
	Stats(ctx context.Context) (*QueueStats, error)

	// Cleanup removes completed jobs older than the given time.
	Cleanup(ctx context.Context, completedBefore time.Time) (int64, error)
}

// PersistedJob represents a job stored in the persistent backend.
type PersistedJob struct {
	ID          int64
	Name        string
	Queue       string
	Priority    int
	Payload     []byte
	Attempts    int
	MaxAttempts int
	LockedBy    string
	CreatedAt   time.Time
}

// QueueStats holds aggregate queue metrics.
// Valid statuses in the job lifecycle: pending → running → completed | dead.
type QueueStats struct {
	Pending   int64
	Running   int64
	Completed int64
	Dead      int64
	Queues    map[string]QueueDetail
}

// QueueDetail holds per-queue metrics.
type QueueDetail struct {
	Name    string
	Pending int64
	Running int64
}
