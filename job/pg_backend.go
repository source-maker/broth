package job

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PgBackend implements Backend using a PostgreSQL broth_jobs table.
type PgBackend struct {
	db *sql.DB
}

// NewPgBackend creates a PgBackend backed by the given *sql.DB.
func NewPgBackend(db *sql.DB) *PgBackend {
	return &PgBackend{db: db}
}

// Enqueue inserts a new job row into broth_jobs with status 'pending'.
func (b *PgBackend) Enqueue(ctx context.Context, j Job, opts jobOptions) error {
	payload, err := json.Marshal(j)
	if err != nil {
		return fmt.Errorf("job: pg backend: enqueue: marshal payload: %w", err)
	}

	const query = `
		INSERT INTO broth_jobs (name, queue, payload, priority, status, max_attempts)
		VALUES ($1, $2, $3, $4, 'pending', $5)`

	maxAttempts := 3
	_, err = b.db.ExecContext(ctx, query,
		j.JobName(),
		opts.queue,
		payload,
		opts.priority,
		maxAttempts,
	)
	if err != nil {
		return fmt.Errorf("job: pg backend: enqueue: %w", err)
	}
	return nil
}

// Fetch retrieves the next available pending job from the specified queues
// using SELECT ... FOR UPDATE SKIP LOCKED for concurrent-worker safety.
// It atomically transitions the job to 'running' status.
func (b *PgBackend) Fetch(ctx context.Context, queues []string, workerID string) (*PersistedJob, error) {
	if len(queues) == 0 {
		return nil, nil
	}

	// Build numbered placeholders for the IN clause: $1, $2, ...
	placeholders := make([]string, len(queues))
	args := make([]interface{}, len(queues)+1)
	for i, q := range queues {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = q
	}
	workerPlaceholder := fmt.Sprintf("$%d", len(queues)+1)
	args[len(queues)] = workerID

	query := fmt.Sprintf(`
		UPDATE broth_jobs
		SET status     = 'running',
		    attempts   = attempts + 1,
		    started_at = NOW(),
		    locked_by  = %s
		WHERE id = (
			SELECT id FROM broth_jobs
			WHERE status = 'pending'
			  AND queue IN (%s)
			ORDER BY priority DESC, created_at ASC
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING id, name, queue, priority, payload, attempts, max_attempts, locked_by, created_at`,
		workerPlaceholder,
		strings.Join(placeholders, ", "),
	)

	pj := &PersistedJob{}
	err := b.db.QueryRowContext(ctx, query, args...).Scan(
		&pj.ID,
		&pj.Name,
		&pj.Queue,
		&pj.Priority,
		&pj.Payload,
		&pj.Attempts,
		&pj.MaxAttempts,
		&pj.LockedBy,
		&pj.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("job: pg backend: fetch: %w", err)
	}
	return pj, nil
}

// Complete marks a job as successfully completed.
func (b *PgBackend) Complete(ctx context.Context, jobID int64) error {
	const query = `
		UPDATE broth_jobs
		SET status       = 'completed',
		    completed_at = NOW(),
		    locked_by    = ''
		WHERE id = $1`

	res, err := b.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("job: pg backend: complete: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job: pg backend: complete: job %d not found", jobID)
	}
	return nil
}

// Fail records a job failure. If the job has remaining attempts it is returned
// to 'pending' status for retry; otherwise it is moved to 'dead'.
func (b *PgBackend) Fail(ctx context.Context, jobID int64, jobErr error) error {
	errMsg := ""
	if jobErr != nil {
		errMsg = jobErr.Error()
	}

	const query = `
		UPDATE broth_jobs
		SET status     = CASE
		        WHEN attempts < max_attempts THEN 'pending'
		        ELSE 'dead'
		    END,
		    last_error = $2,
		    failed_at  = NOW(),
		    locked_by  = ''
		WHERE id = $1`

	res, err := b.db.ExecContext(ctx, query, jobID, errMsg)
	if err != nil {
		return fmt.Errorf("job: pg backend: fail: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job: pg backend: fail: job %d not found", jobID)
	}
	return nil
}

// Dead moves a job directly to the dead-letter state.
func (b *PgBackend) Dead(ctx context.Context, jobID int64, jobErr error) error {
	errMsg := ""
	if jobErr != nil {
		errMsg = jobErr.Error()
	}

	const query = `
		UPDATE broth_jobs
		SET status     = 'dead',
		    last_error = $2,
		    failed_at  = NOW(),
		    locked_by  = ''
		WHERE id = $1`

	res, err := b.db.ExecContext(ctx, query, jobID, errMsg)
	if err != nil {
		return fmt.Errorf("job: pg backend: dead: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job: pg backend: dead: job %d not found", jobID)
	}
	return nil
}

// RetryDead re-enqueues a dead job by resetting its attempts and status.
func (b *PgBackend) RetryDead(ctx context.Context, jobID int64) error {
	const query = `
		UPDATE broth_jobs
		SET status     = 'pending',
		    attempts   = 0,
		    last_error = '',
		    failed_at  = NULL,
		    started_at = NULL,
		    locked_by  = ''
		WHERE id = $1
		  AND status = 'dead'`

	res, err := b.db.ExecContext(ctx, query, jobID)
	if err != nil {
		return fmt.Errorf("job: pg backend: retry dead: %w", err)
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("job: pg backend: retry dead: job %d not found or not dead", jobID)
	}
	return nil
}

// Stats returns aggregate queue metrics across all queues.
// The valid job statuses are: pending, running, completed, dead.
func (b *PgBackend) Stats(ctx context.Context) (*QueueStats, error) {
	const query = `
		SELECT queue, status, COUNT(*)
		FROM broth_jobs
		GROUP BY queue, status`

	rows, err := b.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("job: pg backend: stats: %w", err)
	}
	defer rows.Close()

	stats := &QueueStats{
		Queues: make(map[string]QueueDetail),
	}

	for rows.Next() {
		var queue, status string
		var count int64
		if err := rows.Scan(&queue, &status, &count); err != nil {
			return nil, fmt.Errorf("job: pg backend: stats: scan: %w", err)
		}

		// Aggregate totals.
		switch status {
		case "pending":
			stats.Pending += count
		case "running":
			stats.Running += count
		case "completed":
			stats.Completed += count
		case "dead":
			stats.Dead += count
		}

		// Per-queue detail (pending and running only).
		detail := stats.Queues[queue]
		detail.Name = queue
		switch status {
		case "pending":
			detail.Pending += count
		case "running":
			detail.Running += count
		}
		stats.Queues[queue] = detail
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("job: pg backend: stats: rows: %w", err)
	}

	return stats, nil
}

// Cleanup deletes completed jobs whose completed_at is before the given time.
// It returns the number of deleted rows.
func (b *PgBackend) Cleanup(ctx context.Context, completedBefore time.Time) (int64, error) {
	const query = `
		DELETE FROM broth_jobs
		WHERE status = 'completed'
		  AND completed_at < $1`

	res, err := b.db.ExecContext(ctx, query, completedBefore)
	if err != nil {
		return 0, fmt.Errorf("job: pg backend: cleanup: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("job: pg backend: cleanup: rows affected: %w", err)
	}
	return n, nil
}

// compile-time interface check
var _ Backend = (*PgBackend)(nil)
