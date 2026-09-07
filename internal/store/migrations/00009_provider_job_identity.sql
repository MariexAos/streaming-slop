-- +goose Up
CREATE UNIQUE INDEX generation_attempts_provider_job ON generation_attempts(provider,provider_job_id) WHERE provider_job_id IS NOT NULL;
-- +goose Down
DROP INDEX generation_attempts_provider_job;
