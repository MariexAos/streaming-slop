-- +goose Up
ALTER TABLE segments ADD COLUMN plan_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE generation_attempts ADD COLUMN plan_revision bigint NOT NULL DEFAULT 0;
ALTER TABLE generation_attempts ADD COLUMN request jsonb NOT NULL DEFAULT '{}'::jsonb;
ALTER TABLE generation_attempts DROP CONSTRAINT generation_attempts_segment_id_attempt_no_key;
ALTER TABLE generation_attempts ADD UNIQUE (segment_id, plan_revision, attempt_no);
DROP INDEX generation_attempts_one_active_per_segment;
CREATE UNIQUE INDEX generation_attempts_one_active_per_segment
    ON generation_attempts (segment_id, plan_revision)
    WHERE status NOT IN ('SUCCEEDED', 'FAILED', 'CANCELLED');

-- +goose Down
-- Older code cannot safely interpret multiple revisions of the same segment.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Generation revision migration requires a forward migration to revert'; END $$;
-- +goose StatementEnd
