-- +goose Up
CREATE TABLE halls (
    id       BIGSERIAL PRIMARY KEY,
    name     TEXT NOT NULL UNIQUE,
    capacity INTEGER NOT NULL CHECK (capacity >= 0)
);

-- +goose Down
DROP TABLE IF EXISTS halls;
