-- +goose Up
CREATE TABLE users (
    id       BIGSERIAL PRIMARY KEY,
    username TEXT NOT NULL UNIQUE,
    full_name TEXT NOT NULL
);

-- +goose Down
DROP TABLE IF EXISTS users;
