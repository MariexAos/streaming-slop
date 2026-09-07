-- +goose Up
CREATE TABLE characters (id text PRIMARY KEY, current_version text);
CREATE TABLE reference_assets (
 id text PRIMARY KEY, path text NOT NULL, mime text NOT NULL,
 width integer NOT NULL CHECK(width > 0), height integer NOT NULL CHECK(height > 0),
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE character_versions (
 id text PRIMARY KEY, character_id text NOT NULL REFERENCES characters(id),
 profile jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE characters ADD FOREIGN KEY(current_version) REFERENCES character_versions(id);
CREATE TABLE character_memories (
 character_id text NOT NULL REFERENCES characters(id), session_id uuid NOT NULL REFERENCES live_sessions(id),
 facts jsonb NOT NULL, updated_at timestamptz NOT NULL DEFAULT now(), PRIMARY KEY(character_id, session_id)
);
CREATE TABLE spending_limits (id text PRIMARY KEY, limit_micros bigint NOT NULL CHECK(limit_micros >= 0));
CREATE TABLE spending (
 id text PRIMARY KEY, budget_id text NOT NULL REFERENCES spending_limits(id),
 reserved_micros bigint NOT NULL CHECK(reserved_micros >= 0), charged_micros bigint CHECK(charged_micros >= 0),
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE spending;
DROP TABLE spending_limits;
DROP TABLE character_memories;
ALTER TABLE characters DROP CONSTRAINT characters_current_version_fkey;
DROP TABLE character_versions;
DROP TABLE reference_assets;
DROP TABLE characters;
