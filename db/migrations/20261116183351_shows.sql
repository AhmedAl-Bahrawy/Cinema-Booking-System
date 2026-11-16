-- +goose Up
-- Show.Duration is time.Duration stored as whole minutes.
CREATE TABLE shows (
    id          BIGSERIAL PRIMARY KEY,
    name        TEXT NOT NULL,
    duration_minutes INTEGER NOT NULL CHECK (duration_minutes > 0),
    author_name TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS shows;
