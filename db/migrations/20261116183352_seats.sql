-- +goose Up
CREATE TABLE seats (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT REFERENCES users (id) ON DELETE SET NULL,
    hall_id     BIGINT NOT NULL REFERENCES halls (id) ON DELETE CASCADE,
    seat_number INTEGER NOT NULL CHECK (seat_number > 0),
    status      SMALLINT NOT NULL DEFAULT 0 CHECK (status IN (0, 1, 2)), -- Free (0) | Holding (1) | Booked (2)
    price       INTEGER NOT NULL CHECK (price >= 0)
);

-- One seat number can only exist once per hall.
CREATE UNIQUE INDEX seats_hall_id_seat_number_idx ON seats (hall_id, seat_number);
-- Speed up finding the seats a user is holding / has booked.
CREATE INDEX seats_user_id_idx ON seats (user_id);

-- +goose Down
DROP TABLE IF EXISTS seats;
