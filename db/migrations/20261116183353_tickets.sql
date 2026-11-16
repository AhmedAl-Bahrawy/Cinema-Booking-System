-- +goose Up
CREATE TABLE tickets (
    id      BIGSERIAL PRIMARY KEY,
    seat_id BIGINT NOT NULL REFERENCES seats (id) ON DELETE CASCADE,
    hall_id BIGINT NOT NULL REFERENCES halls (id) ON DELETE CASCADE,
    show_id BIGINT NOT NULL REFERENCES shows (id) ON DELETE CASCADE
);

-- A seat can only be sold once per show.
CREATE UNIQUE INDEX tickets_show_id_seat_id_idx ON tickets (show_id, seat_id);
-- Speed up listing tickets for a show / hall.
CREATE INDEX tickets_hall_id_idx ON tickets (hall_id);

-- +goose Down
DROP TABLE IF EXISTS tickets;
