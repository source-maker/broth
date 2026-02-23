// Package schedule provides cron-like job scheduling with leader election.
package schedule

import (
	"context"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/source-maker/broth/job"
	"github.com/source-maker/broth/log"
)

// Definition describes a scheduled job.
type Definition struct {
	// Name is a unique identifier for this scheduled job.
	Name string

	// Cron is the cron expression (e.g., "0 */5 * * *").
	Cron string

	// Job is the job to execute on schedule.
	Job job.Job

	// Options are the job options applied when enqueuing.
	Options []job.JobOption

	// Overlap allows concurrent executions if true.
	Overlap bool

	// Timezone is the timezone for cron evaluation. Defaults to UTC.
	Timezone *time.Location
}

// LeaderElector determines which instance runs scheduled jobs in a multi-instance deployment.
type LeaderElector interface {
	TryAcquire(ctx context.Context, key string) (bool, error)
	Release(ctx context.Context, key string) error
}

// leaderReleaseTimeout is the maximum time allowed for releasing a leader lock.
const leaderReleaseTimeout = 5 * time.Second

// ErrorFunc is called when a scheduled job fails to enqueue or encounters a
// leader election error. Use it to integrate with alerting or health-check
// systems.
type ErrorFunc func(defName string, err error)

// Scheduler manages periodic job execution.
type Scheduler struct {
	definitions []Definition
	enqueuer    *job.Enqueuer
	logger      *log.Logger
	leader      LeaderElector
	onError     ErrorFunc
	cron        *cron.Cron
	cronLogger  *cronLogger
	wg          sync.WaitGroup
	stopCh      chan struct{}
	startOnce   sync.Once
	stopOnce    sync.Once
}

// SchedulerOption configures a Scheduler.
type SchedulerOption func(*Scheduler)

// WithOnError sets a callback invoked when enqueue or leader election fails.
func WithOnError(fn ErrorFunc) SchedulerOption {
	return func(s *Scheduler) { s.onError = fn }
}

// NewScheduler creates a Scheduler. If leader is nil, all instances will run schedules.
// Register definitions before calling Start. Register must not be called after Start.
func NewScheduler(enqueuer *job.Enqueuer, logger *log.Logger, leader LeaderElector, opts ...SchedulerOption) *Scheduler {
	cl := &cronLogger{logger: logger}
	s := &Scheduler{
		enqueuer:   enqueuer,
		logger:     logger,
		leader:     leader,
		cronLogger: cl,
		cron: cron.New(
			cron.WithLocation(time.UTC),
			cron.WithLogger(cl),
		),
		stopCh: make(chan struct{}),
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

// Register adds schedule definitions. Must be called before Start.
func (s *Scheduler) Register(defs ...Definition) {
	s.definitions = append(s.definitions, defs...)
}

// Start begins evaluating cron schedules and blocks until Shutdown is called
// or the context is cancelled. Calling Start more than once is a no-op.
func (s *Scheduler) Start(ctx context.Context) {
	s.startOnce.Do(func() {
		s.start(ctx)
	})
}

func (s *Scheduler) start(ctx context.Context) {
	for _, def := range s.definitions {
		if err := s.addDefinition(def); err != nil {
			s.logger.Error(ctx, "schedule: failed to add definition",
				"name", def.Name,
				"cron", def.Cron,
				"error", err,
			)
			continue
		}
		s.logger.Info(ctx, "schedule: registered",
			"name", def.Name,
			"cron", def.Cron,
		)
	}

	s.cron.Start()
	s.logger.Info(ctx, "schedule: started", "definitions", len(s.definitions))

	// Block until stopCh is closed or context is cancelled.
	select {
	case <-s.stopCh:
	case <-ctx.Done():
	}
}

// addDefinition registers a single Definition with the underlying cron scheduler.
func (s *Scheduler) addDefinition(def Definition) error {
	// Build the cron job function that enqueues the job.
	var cronJob cron.Job = cron.FuncJob(func() {
		s.wg.Add(1)
		defer s.wg.Done()

		ctx := context.Background()

		// Check leader election at each tick, not just at start time.
		if s.leader != nil {
			acquired, err := s.leader.TryAcquire(ctx, "schedule:"+def.Name)
			if err != nil {
				s.logger.Error(ctx, "schedule: leader election failed",
					"name", def.Name,
					"error", err,
				)
				s.notifyError(def.Name, err)
				return
			}
			if !acquired {
				s.logger.Debug(ctx, "schedule: skipped (not leader)",
					"name", def.Name,
				)
				return
			}
			defer func() {
				releaseCtx, cancel := context.WithTimeout(context.Background(), leaderReleaseTimeout)
				defer cancel()
				if err := s.leader.Release(releaseCtx, "schedule:"+def.Name); err != nil {
					s.logger.Error(releaseCtx, "schedule: leader release failed",
						"name", def.Name,
						"error", err,
					)
				}
			}()
		}

		if err := s.enqueuer.Enqueue(ctx, def.Job, def.Options...); err != nil {
			s.logger.Error(ctx, "schedule: enqueue failed",
				"name", def.Name,
				"error", err,
			)
			s.notifyError(def.Name, err)
		}
	})

	// Wrap with SkipIfStillRunning when overlap is not allowed.
	if !def.Overlap {
		cronJob = cron.SkipIfStillRunning(s.cronLogger)(cronJob)
	}

	// Determine the schedule spec. If a per-definition timezone is set,
	// prepend the CRON_TZ directive so robfig evaluates the expression in
	// that timezone instead of the scheduler-wide default (UTC).
	spec := def.Cron
	if def.Timezone != nil {
		spec = "CRON_TZ=" + def.Timezone.String() + " " + def.Cron
	}

	_, err := s.cron.AddJob(spec, cronJob)
	return err
}

// Shutdown stops the scheduler and waits for in-flight jobs to complete.
// Calling Shutdown more than once is safe; subsequent calls are no-ops that
// wait for in-flight work to finish.
func (s *Scheduler) Shutdown(ctx context.Context) error {
	s.stopOnce.Do(func() { close(s.stopCh) })

	// Stop the cron scheduler. The returned context is cancelled when all
	// running cron jobs (managed by robfig) have finished.
	cronCtx := s.cron.Stop()
	select {
	case <-cronCtx.Done():
	case <-ctx.Done():
		return ctx.Err()
	}

	// Wait for our own tracked goroutines (wg) with timeout.
	done := make(chan struct{})
	go func() {
		s.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// notifyError calls the onError callback if configured. It never panics.
func (s *Scheduler) notifyError(defName string, err error) {
	if s.onError != nil {
		s.onError(defName, err)
	}
}

// cronLogger adapts Broth's log.Logger to the cron.Logger interface required
// by robfig/cron/v3.
type cronLogger struct {
	logger *log.Logger
}

// Info implements cron.Logger.
func (cl *cronLogger) Info(msg string, keysAndValues ...interface{}) {
	cl.logger.Info(context.Background(), "schedule: "+msg, keysAndValues...)
}

// Error implements cron.Logger.
func (cl *cronLogger) Error(err error, msg string, keysAndValues ...interface{}) {
	args := make([]interface{}, 0, len(keysAndValues)+2)
	args = append(args, "error", err)
	args = append(args, keysAndValues...)
	cl.logger.Error(context.Background(), "schedule: "+msg, args...)
}
