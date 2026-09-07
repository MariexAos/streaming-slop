-- +goose Up
CREATE TABLE observer_runs (
    id bigserial PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
    audience_revision bigint NOT NULL,
    messages jsonb NOT NULL,
    observation jsonb NOT NULL,
    error_message text,
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, audience_revision)
);

-- +goose Down
DROP TABLE observer_runs;
