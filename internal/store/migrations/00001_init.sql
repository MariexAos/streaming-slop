-- +goose Up
CREATE TABLE live_sessions (
    id uuid PRIMARY KEY,
    status text NOT NULL CHECK (status IN ('STARTING', 'BUFFERING', 'RUNNING', 'STOPPING', 'STOPPED', 'RECOVERING', 'FAILED')),
    playhead_ms bigint NOT NULL DEFAULT 0 CHECK (playhead_ms >= 0),
    world_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    runtime_config jsonb NOT NULL DEFAULT '{}'::jsonb,
    stream_state jsonb NOT NULL DEFAULT '{}'::jsonb,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL
);

CREATE TABLE segments (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
    sequence integer NOT NULL CHECK (sequence >= 0),
    start_ms bigint NOT NULL CHECK (start_ms >= 0),
    end_ms bigint NOT NULL CHECK (end_ms > start_ms),
    direction jsonb NOT NULL,
    status text NOT NULL CHECK (status IN ('PLANNED', 'GENERATING', 'READY', 'COMMITTED', 'PLAYING', 'PLAYED')),
    current_asset_id uuid,
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (session_id, sequence),
    UNIQUE (session_id, start_ms),
    UNIQUE (session_id, id)
);

CREATE TABLE generation_attempts (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL,
    segment_id uuid NOT NULL,
    attempt_no integer NOT NULL CHECK (attempt_no BETWEEN 1 AND 2),
    provider text NOT NULL,
    idempotency_key text NOT NULL UNIQUE,
    provider_job_id text,
    status text NOT NULL CHECK (status IN ('PENDING_SUBMIT', 'SUBMITTED', 'RUNNING', 'PREPARING', 'SUCCEEDED', 'FAILED', 'CANCELLED')),
    error_code text,
    error_message text,
    latency_ms bigint,
    cost_usd numeric(16, 8),
    version bigint NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL,
    updated_at timestamptz NOT NULL,
    submitted_at timestamptz,
    started_at timestamptz,
    finished_at timestamptz,
    UNIQUE (segment_id, attempt_no),
    FOREIGN KEY (session_id, segment_id) REFERENCES segments(session_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX generation_attempts_one_active_per_segment
    ON generation_attempts (segment_id)
    WHERE status NOT IN ('SUCCEEDED', 'FAILED', 'CANCELLED');

CREATE INDEX generation_attempts_pending
    ON generation_attempts (session_id, updated_at)
    WHERE status NOT IN ('SUCCEEDED', 'FAILED', 'CANCELLED');

CREATE TABLE assets (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL,
    segment_id uuid NOT NULL,
    attempt_id uuid REFERENCES generation_attempts(id),
    source text NOT NULL CHECK (source IN ('generated', 'fallback')),
    uri text NOT NULL,
    normalized_uri text NOT NULL,
    duration_ms bigint NOT NULL CHECK (duration_ms > 0),
    width integer NOT NULL,
    height integer NOT NULL,
    fps integer NOT NULL,
    checksum text,
    verified_at timestamptz NOT NULL,
    is_current boolean NOT NULL DEFAULT true,
    created_at timestamptz NOT NULL DEFAULT now(),
    FOREIGN KEY (session_id, segment_id) REFERENCES segments(session_id, id) ON DELETE CASCADE
);

CREATE UNIQUE INDEX assets_one_current_per_segment
    ON assets (segment_id)
    WHERE is_current;

ALTER TABLE segments
    ADD CONSTRAINT segments_current_asset_fk
    FOREIGN KEY (current_asset_id) REFERENCES assets(id);

CREATE TABLE stream_runs (
    id uuid PRIMARY KEY,
    session_id uuid NOT NULL REFERENCES live_sessions(id) ON DELETE CASCADE,
    status text NOT NULL,
    started_at timestamptz NOT NULL,
    ended_at timestamptz,
    bitrate_kbps double precision,
    dropped_frames bigint NOT NULL DEFAULT 0,
    gap_total bigint NOT NULL DEFAULT 0,
    last_error text
);

-- +goose Down
DROP TABLE stream_runs;
ALTER TABLE segments DROP CONSTRAINT segments_current_asset_fk;
DROP TABLE assets;
DROP TABLE generation_attempts;
DROP TABLE segments;
DROP TABLE live_sessions;

