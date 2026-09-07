-- +goose Up
ALTER TABLE assets ADD COLUMN observed jsonb NOT NULL DEFAULT '{}'::jsonb;
CREATE TABLE visual_reviews (
 attempt_id uuid PRIMARY KEY REFERENCES generation_attempts(id),
 review jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE visual_reviews;
ALTER TABLE assets DROP COLUMN observed;
