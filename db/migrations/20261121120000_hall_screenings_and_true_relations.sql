-- +goose Up
-- A hall can show different shows during the same day, but only one show at a
-- time. A screening is the "show happens in this hall during this time slot".
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE screenings (
    id        BIGSERIAL PRIMARY KEY,
    hall_id   BIGINT NOT NULL REFERENCES halls (id) ON DELETE CASCADE,
    show_id   BIGINT NOT NULL REFERENCES shows (id) ON DELETE CASCADE,
    starts_at TIMESTAMPTZ NOT NULL,
    ends_at   TIMESTAMPTZ NOT NULL,

    CONSTRAINT screenings_time_valid CHECK (ends_at > starts_at),

    -- The real rule: two screenings in the same hall may never overlap.
    -- The same hall can still play a different show later in the day.
    CONSTRAINT screenings_hall_no_overlap EXCLUDE USING gist (
        hall_id WITH =,
        tstzrange(starts_at, ends_at) WITH &&
    )
);

CREATE INDEX screenings_hall_id_starts_at_idx ON screenings (hall_id, starts_at);
CREATE INDEX screenings_show_id_idx ON screenings (show_id);

-- ---------------------------------------------------------------------------
-- Move booking data off the physical seat and onto the ticket.
-- ---------------------------------------------------------------------------

-- Give every legacy (hall_id, show_id) pair a screening so old tickets still
-- point at something. Legacy tickets had no start time, so put each pair on
-- its own day; they can therefore never overlap.
INSERT INTO screenings (hall_id, show_id, starts_at, ends_at)
SELECT
    legacy.hall_id,
    legacy.show_id,
    date_trunc('day', now()) + (legacy.position - 1) * INTERVAL '1 day',
    date_trunc('day', now()) + (legacy.position - 1) * INTERVAL '1 day'
        + sh.duration_minutes * INTERVAL '1 minute'
FROM (
    SELECT
        hall_id,
        show_id,
        ROW_NUMBER() OVER (ORDER BY hall_id, show_id) AS position
    FROM tickets
    GROUP BY hall_id, show_id
) AS legacy
JOIN shows sh ON sh.id = legacy.show_id;

ALTER TABLE tickets ADD COLUMN screening_id BIGINT;
ALTER TABLE tickets ADD COLUMN user_id BIGINT;
ALTER TABLE tickets ADD COLUMN status SMALLINT NOT NULL DEFAULT 2;

-- Link each legacy ticket to the screening created for its hall/show pair.
UPDATE tickets t
SET screening_id = s.id
FROM screenings s
WHERE s.hall_id = t.hall_id
  AND s.show_id = t.show_id;

-- A ticket now says who owns it. Recover the holder from the old seat row.
UPDATE tickets t
SET user_id = s.user_id
FROM seats s
WHERE s.id = t.seat_id
  AND s.user_id IS NOT NULL;

-- Extremely old tickets may have no recoverable user (the old seat was free).
-- Keep the migration safe by attaching them to a single legacy account.
INSERT INTO users (username, full_name)
SELECT '__legacy_ticket_holder__', 'Legacy Ticket Holder'
WHERE EXISTS (SELECT 1 FROM tickets WHERE user_id IS NULL)
ON CONFLICT (username) DO NOTHING;

UPDATE tickets
SET user_id = (SELECT id FROM users WHERE username = '__legacy_ticket_holder__')
WHERE user_id IS NULL;

ALTER TABLE tickets ALTER COLUMN screening_id SET NOT NULL;
ALTER TABLE tickets ALTER COLUMN user_id SET NOT NULL;

-- Replace the old direct hall/show links with the screening link.
ALTER TABLE tickets DROP CONSTRAINT tickets_hall_id_fkey;
ALTER TABLE tickets DROP CONSTRAINT tickets_show_id_fkey;
DROP INDEX IF EXISTS tickets_show_id_seat_id_idx;
DROP INDEX IF EXISTS tickets_hall_id_idx;

ALTER TABLE tickets DROP COLUMN hall_id;
ALTER TABLE tickets DROP COLUMN show_id;

ALTER TABLE tickets
    ADD CONSTRAINT tickets_screening_id_fkey
        FOREIGN KEY (screening_id) REFERENCES screenings (id) ON DELETE CASCADE,
    ADD CONSTRAINT tickets_user_id_fkey
        FOREIGN KEY (user_id) REFERENCES users (id) ON DELETE CASCADE,
    ADD CONSTRAINT tickets_status_check
        CHECK (status IN (1, 2, 3)); -- Held (1) | Booked (2) | Cancelled (3)

-- A physical seat can be sold only once per screening. It is free again for
-- the next screening, even to the same user.
CREATE UNIQUE INDEX tickets_screening_id_seat_id_idx ON tickets (screening_id, seat_id);
CREATE INDEX tickets_user_id_idx ON tickets (user_id);

-- Seats are physical: they no longer know a user or a booking status.
DROP INDEX IF EXISTS seats_user_id_idx;
ALTER TABLE seats DROP COLUMN user_id;
ALTER TABLE seats DROP COLUMN status;

-- Defense in depth: a ticket's seat must belong to the hall where its
-- screening is playing. The repository also checks this, but the database
-- should never allow an inconsistent row.
-- +goose StatementBegin
CREATE FUNCTION enforce_ticket_seat_hall() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM screenings sc
        JOIN seats s ON s.id = NEW.seat_id
        WHERE sc.id = NEW.screening_id
          AND s.hall_id = sc.hall_id
    ) THEN
        RAISE EXCEPTION 'seat % is not in the hall of screening %',
            NEW.seat_id, NEW.screening_id
            USING ERRCODE = '23514';
    END IF;
    RETURN NEW;
END;
$$;
-- +goose StatementEnd

CREATE TRIGGER tickets_seat_hall_check
BEFORE INSERT OR UPDATE OF screening_id, seat_id ON tickets
FOR EACH ROW EXECUTE FUNCTION enforce_ticket_seat_hall();

-- +goose Down
DROP TRIGGER IF EXISTS tickets_seat_hall_check ON tickets;
DROP FUNCTION IF EXISTS enforce_ticket_seat_hall();

-- Rebuild the old ticket columns from the screening link.
ALTER TABLE tickets ADD COLUMN hall_id BIGINT;
ALTER TABLE tickets ADD COLUMN show_id BIGINT;

UPDATE tickets t
SET hall_id = s.hall_id,
    show_id = s.show_id
FROM screenings s
WHERE s.id = t.screening_id;

ALTER TABLE tickets ALTER COLUMN hall_id SET NOT NULL;
ALTER TABLE tickets ALTER COLUMN show_id SET NOT NULL;

ALTER TABLE tickets
    ADD CONSTRAINT tickets_hall_id_fkey
        FOREIGN KEY (hall_id) REFERENCES halls (id) ON DELETE CASCADE,
    ADD CONSTRAINT tickets_show_id_fkey
        FOREIGN KEY (show_id) REFERENCES shows (id) ON DELETE CASCADE;

-- Restore the old seat columns before dropping tickets.user_id. A seat can
-- only hold one user again, so pick the owner from its most recent ticket
-- (or stay NULL when it has none).
ALTER TABLE seats ADD COLUMN user_id BIGINT REFERENCES users (id) ON DELETE SET NULL;
ALTER TABLE seats ADD COLUMN status SMALLINT NOT NULL DEFAULT 0 CHECK (status IN (0, 1, 2));
CREATE INDEX seats_user_id_idx ON seats (user_id);

UPDATE seats s
SET user_id = latest.user_id,
    status = CASE WHEN latest.user_id IS NULL THEN 0 ELSE 2 END
FROM (
    SELECT DISTINCT ON (t.seat_id) t.seat_id, t.user_id
    FROM tickets t
    ORDER BY t.seat_id, t.id DESC
) AS latest
WHERE latest.seat_id = s.id;

DROP INDEX IF EXISTS tickets_screening_id_seat_id_idx;
DROP INDEX IF EXISTS tickets_user_id_idx;

ALTER TABLE tickets DROP COLUMN screening_id;
ALTER TABLE tickets DROP COLUMN user_id;
ALTER TABLE tickets DROP COLUMN status;

CREATE UNIQUE INDEX tickets_show_id_seat_id_idx ON tickets (show_id, seat_id);
CREATE INDEX tickets_hall_id_idx ON tickets (hall_id);

DROP TABLE IF EXISTS screenings;
