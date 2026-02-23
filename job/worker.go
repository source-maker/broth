package job

import (
	"context"
	"sync"
	"time"

	"github.com/source-maker/broth/log"
)

// WorkerConfig holds worker pool configuration.
type WorkerConfig struct {
	Concurrency     int
	ShutdownTimeout time.Duration
}

// Worker processes jobs from the in-memory queue.
type Worker struct {
	enqueuer    *Enqueuer
	concurrency int
	logger      *log.Logger
	wg          sync.WaitGroup
	stopCh      chan struct{}
}

// NewWorker creates a Worker that consumes jobs from the given Enqueuer.
func NewWorker(enqueuer *Enqueuer, cfg WorkerConfig, logger *log.Logger) *Worker {
	conc := cfg.Concurrency
	if conc <= 0 {
		conc = 1
	}
	return &Worker{
		enqueuer:    enqueuer,
		concurrency: conc,
		logger:      logger,
		stopCh:      make(chan struct{}),
	}
}

// Start launches the worker goroutines. It blocks until Shutdown is called.
func (w *Worker) Start(ctx context.Context) {
	for i := range w.concurrency {
		w.wg.Add(1)
		go w.run(ctx, i)
	}
}

// Shutdown signals all workers to stop and waits for them to finish.
func (w *Worker) Shutdown(ctx context.Context) error {
	close(w.stopCh)

	done := make(chan struct{})
	go func() {
		w.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (w *Worker) run(ctx context.Context, id int) {
	defer w.wg.Done()
	queue := w.enqueuer.Queue()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ctx.Done():
			return
		case env := <-queue:
			w.execute(ctx, id, env)
		}
	}
}

func (w *Worker) execute(ctx context.Context, workerID int, env envelope) {
	w.logger.Info(ctx, "job: executing",
		"job", env.job.JobName(),
		"worker", workerID,
		"queue", env.opts.queue,
	)

	if err := env.job.Execute(ctx); err != nil {
		w.logger.Error(ctx, "job: failed",
			"job", env.job.JobName(),
			"worker", workerID,
			"error", err,
		)
	}
}
