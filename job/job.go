// Package job provides a two-layer job system:
// in-memory goroutine queue for fast jobs and a persistent backend for durable jobs.
package job

import "context"

// Job is the interface that all background jobs must implement.
type Job interface {
	// JobName returns a unique identifier for this job type.
	JobName() string

	// Execute runs the job logic.
	Execute(ctx context.Context) error
}

// Definition describes a registered job type for module discovery.
type Definition struct {
	Name    string
	Factory func() Job
}

// JobOption configures how a job is enqueued.
type JobOption func(*jobOptions)

type jobOptions struct {
	queue    string
	persist  bool
	priority int
}

func defaultJobOptions() jobOptions {
	return jobOptions{
		queue:    "default",
		persist:  false,
		priority: 0,
	}
}

// WithQueue sets the queue name for the job.
func WithQueue(name string) JobOption {
	return func(o *jobOptions) { o.queue = name }
}

// WithPersist marks the job for persistent (database-backed) execution.
func WithPersist() JobOption {
	return func(o *jobOptions) { o.persist = true }
}

// WithPriority sets the job priority (higher values = higher priority).
func WithPriority(p int) JobOption {
	return func(o *jobOptions) { o.priority = p }
}
