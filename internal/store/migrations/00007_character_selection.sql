-- +goose Up
CREATE TABLE character_selection (singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), version_id text NOT NULL REFERENCES character_versions(id));
-- +goose Down
DROP TABLE character_selection;
