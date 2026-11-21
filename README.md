# Cinema-Booking-System
 Building a Concurrent Cinema Booking System in Golang 

## Rules
- No AI Writes Code
- You Can Ask AI but not copy
- MD files only AI can write it and not for today btw

### Create Ticket System
- System must hold tickets
- Some Cinema Seats
- If one trying to book a seat it will shows for other users
- Do not get in stuck with race conditions
- We Will have two paralel channels (window) at least to book the ticket
- If have time we can Design a good UI for the App


### Focus
- 1st. double booking
- 2nd. handle when "A" user starts booking a seat while "B" user wants to book this seat also (we can use a color to say that is a hold seat)


---

## Progress

> Updated: 2026-11-19

| Layer | Status | Notes |
|-------|--------|-------|
| Database migrations | ✅ Done | `users`, `halls`, `shows`, `seats`, `screenings`, `tickets` |
| Models + repositories | ✅ Done | Explicit SQL with `pgx` v5 (no ORM) |
| Repository tests | ✅ Done | Integration tests against a real PostgreSQL DB |
| Handlers | 🚧 In progress | `CreateUser`, `CreateHall`, `CreateSeat`, `CreateScreening`, `CreateTicket` are implemented |
| Router | 🚧 In progress | User, hall, seat, screening and ticket routes are registered |
| Booking logic (hold/book) | ⬜ Not started | The main goal of the project |
| UI | ⬜ Not started | Only if there is time |

The project currently **starts**, connects to PostgreSQL, builds all repositories,
wires them into the handlers and router, and exposes create endpoints for users,
halls, seats, screenings and tickets. The relations are fully normalised: a
physical seat is reusable, a hall can schedule different shows during the day,
and a ticket is the only place where a user, a seat and a screening meet.

---

## Tech Stack

- **Language:** Go `1.27.3`
- **HTTP framework:** [Gin](https://github.com/gin-gonic/gin)
- **Database:** PostgreSQL
- **DB driver:** [`pgx`](https://github.com/jackc/pgx) v5 (`pgxpool`)
- **Migrations:** [goose](https://github.com/pressly/goose)
- **Env loading:** `godotenv` (`.env` at the repo root)
- **Tests:** standard `go test`, real database, no mocks

---

## Project Structure

```
Cinema-Booking-System/
├── main.go                     # Loads .env, connects the pool, wires repos → handlers → router
├── db/
│   ├── postgres.go             # pgxpool connection helper
│   ├── migrations/             # goose SQL migrations (source of truth for the schema)
│   └── models/                 # Structs + repositories (all SQL lives here)
│       ├── user.go
│       ├── hall.go
│       ├── show.go
│       ├── screening.go        # A show playing in a hall during a time slot
│       ├── seat.go
│       ├── seat_category.go    # Price multiplier per seat level
│       ├── ticket.go
│       ├── *_test.go           # Integration tests for every repository
│       └── README.md           # Deep dive into the models/repository layer
├── handlers/                   # Gin handlers (HTTP layer)
│   ├── handlers.go             # Handler struct holding all repositories
│   ├── user.go                 # CreateUser handler + request validation
│   ├── hall.go                 # CreateHall
│   ├── show.go                 # CreateShow / GetShow
│   ├── seat.go                 # CreateSeat / GetSeat
│   ├── seat_category.go        # Seat category CRUD helpers
│   ├── screening.go            # CreateScreening / GetScreening
│   ├── ticket.go               # CreateTicket / GetTicket
│   └── README.md               # Endpoint reference and handler conventions
├── router/
│   └── router.go               # Registers routes on the Gin engine
├── internal/
│   └── testdb/testdb.go        # Creates a throwaway DB, applies migrations, cleans up
├── go.mod / go.sum
├── .env                        # Local only (not committed)
└── README.md
```

**Request flow:** `main.go` → `db.ConnectDB` → `models.New*Repository(pool)`
→ `handlers.NewHandler(...)` → `router.SetupRouter(h)` → `r.Run(":8080")`.

**Where to read more:** [`db/models/README.md`](db/models/README.md) documents the
schema, the struct/tag conventions, the repository naming pattern, the three query
patterns used in the code, and the testing setup in detail.
[`handlers/README.md`](handlers/README.md) documents the HTTP layer, the current
endpoint, and the handler conventions.

---

## Getting Started

### Prerequisites

- Go 1.27+
- PostgreSQL (running locally or in Docker)
- [goose](https://github.com/pressly/goose) CLI

### 1. Create the `.env` file

Create a `.env` file in the repository root (it is git-ignored):

```env
GOOSE_DRIVER=postgres
GOOSE_DBSTRING=postgres://postgres:postgres@localhost:5432/cinema_booking?sslmode=disable
GOOSE_MIGRATION_DIR=./db/migrations
```

Create the database if it does not exist:

```bash
createdb cinema_booking
```

### 2. Apply migrations

```bash
goose up
```

Rollback the last migration (or all of them):

```bash
goose down
goose down-to 0
```

### 3. Run the server

```bash
go run .
```

The server listens on **`http://localhost:8080`**. The create/read routes for
users, halls, seats, screenings and tickets are registered (see
[`handlers/README.md`](handlers/README.md)).

### 4. Try the current endpoint

```bash
curl -i -X POST http://localhost:8080/user \
  -H 'Content-Type: application/json' \
  -d '{"username":"ahmed","full_name":"Ahmed Al-Bahrawy"}'
```

A successful request returns `201 Created`:

```json
{ "message": "user created successfully" }
```

Invalid JSON, a username/full name outside the allowed length, or a duplicate
username currently returns `400 Bad Request` — but note that the handler also
calls `log.Println` on validation/repository errors, which stops the server. See
[`handlers/README.md`](handlers/README.md) for the full endpoint reference and
known issues.

### 5. Run the tests

Tests use `TEST_DATABASE_URL`, falling back to `GOOSE_DBSTRING` from `.env`.
If neither is set, model tests are **skipped** (so `go test ./...` still passes
without a database).

```bash
go test ./...                          # everything
go test ./db/models -v                 # model/repository tests only
go test ./db/models -run TestCreateUser
go build ./... && go vet ./...
```

> Tests recreate a throwaway database named `cinema_booking_test`. **Point these
> variables at a test/local database, never at production data.**

---

## Database Schema (Quick View)

```
users (id, username UNIQUE, full_name)
halls (id, name UNIQUE, capacity >= 0)
shows (id, name, duration_minutes > 0, author_name, price >= 0)

seat_categories (id, name UNIQUE, price_multiplier_percent > 0)
                 -- seeded: normal 100, premium 150, vip 200

seats (id, hall_id → halls.id, seat_number > 0,
       category_id → seat_categories.id)
       UNIQUE (hall_id, seat_number)
       -- physical seats only: no user, no booking status, no price

screenings (id, hall_id → halls.id, show_id → shows.id,
            starts_at, ends_at)
            CHECK (ends_at > starts_at)
            EXCLUDE (hall_id =, [starts_at, ends_at) &&)
            -- one hall can never play two shows at the same time

tickets (id, screening_id → screenings.id, seat_id → seats.id,
         user_id → users.id, number > 0, price >= 0, status 1|2|3)
         UNIQUE (screening_id, seat_id)   -- a seat is sold once per screening
         UNIQUE (screening_id, number)    -- ticket number = hall seat number
```

### Ticket number and price

- `tickets.number` is copied from `seats.seat_number` (the seat number inside
  the hall) and kept in sync by the `tickets_seat_number_check` trigger.
- `tickets.price` is a snapshot computed when the ticket is sold:

  ```
  ticket.price = ROUND(show.price × seat_category.price_multiplier_percent / 100)
  ```

  | Category | Multiplier | Show price | Ticket price |
  |----------|-----------|------------|--------------|
  | normal   | 100%      | 80         | 80           |
  | premium  | 150%      | 80         | 120          |
  | vip      | 200%      | 80         | 160          |
  | box      | 125%      | 80         | 100          |

  Custom categories can be added with `POST /seat-category`.

### Relations

```
user ──< tickets >── seat ──> hall ──< screenings >── show
                     │                    │
                     └────────────────────┘
                        (same hall)
```

- A **seat** is physical and reusable. It belongs to exactly one hall, has no
  user or status, and has a category (normal / premium / VIP) instead of a
  fixed price.
- A **seat category** sets the price multiplier over the show's base price.
- A **screening** is "this show plays in this hall at this time". A hall can
  show different films during the same day, but never two at the same time.
- A **ticket** is the booking: it names the seat, the user, and the screening,
  and stores the hall seat number and the computed price. The same seat can
  therefore be sold to the same user (or another user) for a later screening on
  the same day.

Relations: `User` → `Tickets`, `Hall` → `Seats` + `Screenings`,
`Show` → `Screenings`, `Seat` → `Hall` + `SeatCategory`,
`Ticket` → `Seat` + `User` + `Screening`.

### Ticket status values

| Value | Name | Meaning |
|-------|------|---------|
| `1` | `Held` | Temporarily reserved by a user (the "hold seat" color) |
| `2` | `Booked` | Paid/sold |
| `3` | `Cancelled` | Released again |

Relations: `User` → `Tickets`, `Hall` → `Seats` + `Screenings`,
`Show` → `Screenings`, `Seat` → `Hall`, `Ticket` → `Seat` + `User` + `Screening`.

---

## Booking Flow Design (Concurrency Notes)

These are the design targets for the core feature — nothing is implemented yet.

1. **No double booking.** Two users must never end up with a ticket for the same
   seat and screening. Enforced by:
   - `UNIQUE (screening_id, seat_id)` on `tickets`.
   - `INSERT ... SELECT` that only succeeds when the seat belongs to the hall of
     the screening.
   - A trigger (`tickets_seat_hall_check`) as a last line of defence.
2. **Hold → Book lifecycle.** Booking is two phases on the *ticket*, not on the
   physical seat:
   - *Hold*: insert a ticket with `status = 1` (`Held`) and `expires_at` (to be
     added). It shows up as held for everyone else.
   - *Book*: `UPDATE tickets SET status = 2 WHERE id = $1 AND status = 1`.
   - *Release*: `UPDATE tickets SET status = 3 WHERE id = $1 AND status = 1`, or
     delete the held ticket after it expires.
3. **Row locking when needed.** For flows that read then write (e.g. choosing
   several seats at once), use a transaction with `SELECT ... FOR UPDATE` on the
   screening/ticket rows so concurrent requests serialize instead of racing.
4. **Check the result, not just the error.** An `UPDATE ... WHERE status = 1`
   that matches zero rows is a conflict, not a database error — the handler must
   return `409 Conflict`.
5. **Hold expiry.** Add an `expires_at` column on `tickets` (or a background
   sweeper) so abandoned holds do not stay coloured forever.
6. **Hall scheduling.** The `screenings` exclusion constraint already prevents
   two overlapping shows in one hall. A `POST /screening` that overlaps returns
   `409 Conflict`.
7. **Live updates between users.** When a ticket changes state, broadcast the new
   state (SSE / WebSocket) to the other ticket windows so every user sees the
   hold/book colour immediately.
8. **Two parallel windows.** Model each booking window as an independent
   request path (goroutine/channel or simply concurrent HTTP requests). The
   database is the single source of truth, so the windows never need to share
   in-memory state.

### Suggested handler checklist

```
[ ] POST   /tickets/:id/hold     -> create Held ticket (409 on conflict)
[ ] POST   /tickets/:id/release  -> Held → Cancelled (owner only)
[ ] POST   /tickets/:id/book     -> Held → Booked
[ ] GET    /screenings/:id/seats -> seats with ticket status + holder for the UI
[ ] GET    /shows/:id/screenings -> schedule for a show
[ ] SSE /  /events               -> live seat status updates
```

---

## Roadmap

- [x] Connect to PostgreSQL with `pgxpool`
- [x] Write the schema as goose migrations
- [x] Implement models and repositories
- [x] Cover repositories with integration tests
- [x] Wire repositories into handlers and router
- [x] Implement `POST /user` (`CreateUser`)
- [x] Model true relations (`seats` ↔ `screenings` ↔ `tickets`)
- [x] Implement `CreateSeat`, `CreateScreening`, `CreateTicket`
- [ ] Implement the remaining user routes (`GET /users`, `GET /user/:id`, ...)
- [ ] Implement hall/show/seat/ticket handlers and routes
- [ ] Harden handler error handling (no `log.Println`; return `400`/`404`/`409`/`500`)
- [ ] Implement the hold → book lifecycle with transactions
- [ ] Prevent double booking (atomic updates + unique constraints)
- [ ] Handle concurrent users on the same seat (`409` + live status)
- [ ] Add hold expiry
- [ ] Build a simple UI (seat colors: free / holding / booked)
- [ ] Optionally run the two booking windows in parallel

---

## Rules Reminder

- **No AI writes code** — AI may explain and review, but the code is written by hand.
- **Markdown files are the exception** (this README, `db/models/README.md` and
  `handlers/README.md`).
