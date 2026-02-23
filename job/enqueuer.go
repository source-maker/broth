package job

import (
	"context"
	"fmt"
	"time"

	"github.com/source-maker/broth/log"
)

// envelope wraps a job with its metadata for the in-memory queue.
type envelope struct {
	job        Job
	opts       jobOptions
	enqueuedAt time.Time
}

// Enqueuer dispatches jobs to in-memory or persistent backends.
type Enqueuer struct {
	memQueue chan envelope
	backend  Backend
	logger   *log.Logger
}

// EnqueuerOption configures an Enqueuer.
type EnqueuerOption func(*Enqueuer)

// WithBackend sets the persistent backend for the enqueuer.
func WithBackend(b Backend) EnqueuerOption {
	return func(e *Enqueuer) { e.backend = b }
}

// WithQueueSize sets the in-memory queue buffer size.
func WithQueueSize(size int) EnqueuerOption {
	return func(e *Enqueuer) { e.memQueue = make(chan envelope, size) }
}

// NewEnqueuer creates an Enqueuer with the given options.
func NewEnqueuer(logger *log.Logger, opts ...EnqueuerOption) *Enqueuer {
	e := &Enqueuer{
		memQueue: make(chan envelope, 1000),
		logger:   logger,
	}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Enqueue dispatches a job. If WithPersist is set and a backend is configured,
// the job is sent to the persistent backend; otherwise it goes to the in-memory queue.
func (e *Enqueuer) Enqueue(ctx context.Context, j Job, opts ...JobOption) error {
	o := defaultJobOptions()
	for _, fn := range opts {
		fn(&o)
	}

	if o.persist {
		if e.backend == nil {
			return fmt.Errorf("job: no persistent backend configured for job %s", j.JobName())
		}
		return e.backend.Enqueue(ctx, j, o)
	}

	env := envelope{
		job:        j,
		opts:       o,
		enqueuedAt: time.Now(),
	}

	select {
	case e.memQueue <- env:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Queue returns the in-memory job channel for workers to consume.
func (e *Enqueuer) Queue() <-chan envelope {
	return e.memQueue
}
