-- +goose Up
ALTER TABLE generation_attempts ADD COLUMN cost_cny numeric(16, 8);

-- +goose Down
ALTER TABLE generation_attempts DROP COLUMN cost_cny;
