-- +goose Up
ALTER TABLE provider_secrets DROP CONSTRAINT provider_secrets_name_check;
ALTER TABLE provider_secrets ADD CONSTRAINT provider_secrets_name_check CHECK (name IN ('minimax_api_key', 'fal_api_key', 'qwen_api_key'));
CREATE TABLE model_settings (
 singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton),
 settings jsonb NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE model_settings;
DELETE FROM provider_secrets WHERE name='fal_api_key';
ALTER TABLE provider_secrets DROP CONSTRAINT provider_secrets_name_check;
ALTER TABLE provider_secrets ADD CONSTRAINT provider_secrets_name_check CHECK (name IN ('minimax_api_key', 'qwen_api_key'));
