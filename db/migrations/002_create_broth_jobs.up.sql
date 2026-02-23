-- +goose Up
-- +goose StatementBegin

CREATE TABLE broth_jobs (
    id           BIGSERIAL    PRIMARY KEY,
    name         VARCHAR(255) NOT NULL,
    queue        VARCHAR(100) NOT NULL DEFAULT 'default',
    payload      JSONB        NOT NULL DEFAULT '{}',
    priority     INT          NOT NULL DEFAULT 0,
    status       VARCHAR(20)  NOT NULL DEFAULT 'pending',
    attempts     INT          NOT NULL DEFAULT 0,
    max_attempts INT          NOT NULL DEFAULT 3,
    locked_by    VARCHAR(255) NOT NULL DEFAULT '',
    last_error   TEXT         NOT NULL DEFAULT '',
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    scheduled_at TIMESTAMPTZ,
    started_at   TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    failed_at    TIMESTAMPTZ
);

CREATE INDEX idx_broth_jobs_fetch ON broth_jobs (queue, status, priority DESC, created_at ASC)
    WHERE status = 'pending';
CREATE INDEX idx_broth_jobs_status ON broth_jobs (status);
CREATE INDEX idx_broth_jobs_cleanup ON broth_jobs (status, completed_at)
    WHERE status = 'completed';

-- +goose StatementEnd
