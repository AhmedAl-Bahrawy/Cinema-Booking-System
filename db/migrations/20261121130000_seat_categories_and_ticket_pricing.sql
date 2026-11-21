-- +goose Up
-- Seat categories are the "seat status" levels (normal, premium, VIP, ...).
-- The percentage is a multiplier over the show price: 100 = 1.00x, 150 = 1.50x.
CREATE TABLE seat_categories (
    id                       BIGSERIAL PRIMARY KEY,
    name                     TEXT NOT NULL UNIQUE,
    price_multiplier_percent INTEGER NOT NULL CHECK (price_multiplier_percent > 0)
);

INSERT INTO seat_categories (name, price_multiplier_percent) VALUES
    ('normal', 100),
    ('premium', 150),
    ('vip', 200);

-- Every seat has a category. Existing seats become normal.
ALTER TABLE seats
    ADD COLUMN category_id BIGINT NOT NULL DEFAULT 1 REFERENCES seat_categories (id) ON DELETE RESTRICT;
CREATE INDEX seats_category_id_idx ON seats (category_id);

-- A show now has the base price of a normal seat.
ALTER TABLE shows ADD COLUMN price INTEGER NOT NULL DEFAULT 0 CHECK (price >= 0);

-- A seat no longer has a fixed price: the ticket price is computed from the
-- show price and the seat category.
ALTER TABLE seats DROP COLUMN price;

-- A ticket now carries the hall seat number and the price snapshot.
ALTER TABLE tickets ADD COLUMN number INTEGER;
ALTER TABLE tickets ADD COLUMN price INTEGER;

UPDATE tickets t
SET number = s.seat_number,
    price = ROUND(sh.price * cat.price_multiplier_percent / 100.0)::INTEGER
FROM seats s, screenings sc, shows sh, seat_categories cat
WHERE s.id = t.seat_id
  AND sc.id = t.screening_id
  AND sh.id = sc.show_id
  AND cat.id = s.category_id;

ALTER TABLE tickets ALTER COLUMN number SET NOT NULL;
ALTER TABLE tickets ALTER COLUMN price SET NOT NULL;

ALTER TABLE tickets
    ADD CONSTRAINT tickets_number_positive CHECK (number > 0),
    ADD CONSTRAINT tickets_price_non_negative CHECK (price >= 0);

-- The ticket number is the seat number inside the hall, so one screening can
-- only have one ticket with each number.
CREATE UNIQUE INDEX tickets_screening_id_number_idx ON tickets (screening_id, number);

-- Keep ticket.number honest even for direct SQL inserts.
-- +goose StatementBegin
CREATE FUNCTION enforce_ticket_number() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.number <> (SELECT seat_number FROM seats WHERE id = NEW.seat_id) THEN
        RAISE EXCEPTION 'ticket number % must equal the seat number', NEW.number
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER tickets_seat_number_check
BEFORE INSERT OR UPDATE OF number, seat_id ON tickets
FOR EACH ROW EXECUTE FUNCTION enforce_ticket_number();

-- +goose Down
DROP TRIGGER IF EXISTS tickets_seat_number_check ON tickets;
DROP FUNCTION IF EXISTS enforce_ticket_number();

DROP INDEX IF EXISTS tickets_screening_id_number_idx;

ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_number_positive;
ALTER TABLE tickets DROP CONSTRAINT IF EXISTS tickets_price_non_negative;
ALTER TABLE tickets DROP COLUMN number;
ALTER TABLE tickets DROP COLUMN price;

-- Restore the old fixed seat price. The original values were dropped, so the
-- restored column starts at 0.
ALTER TABLE seats ADD COLUMN price INTEGER NOT NULL DEFAULT 0 CHECK (price >= 0);

ALTER TABLE shows DROP COLUMN price;

DROP INDEX IF EXISTS seats_category_id_idx;
ALTER TABLE seats DROP COLUMN category_id;

DROP TABLE IF EXISTS seat_categories;
