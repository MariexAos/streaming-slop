-- +goose Up
CREATE TABLE provider_secrets (
    name text PRIMARY KEY,
    value text NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT provider_secrets_name_check CHECK (name IN ('minimax_api_key', 'qwen_api_key'))
);

-- +goose Down
DROP TABLE provider_secrets;
