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
| Database migrations | ✅ Done | `users`, `halls`, `shows`, `seats`, `tickets` |
| Models + repositories | ✅ Done | Explicit SQL with `pgx` v5 (no ORM) |
| Repository tests | ✅ Done | Integration tests against a real PostgreSQL DB |
| Handlers | 🚧 In progress | `CreateUser` is implemented; hall/show/seat/ticket handlers are pending |
| Router | 🚧 In progress | `POST /user` is registered; the other routes are pending |
| Booking logic (hold/book) | ⬜ Not started | The main goal of the project |
| UI | ⬜ Not started | Only if there is time |

The project currently **starts**, connects to PostgreSQL, builds all repositories,
wires them into the handlers and router, and exposes one endpoint: `POST /user`.
The next step is the remaining handlers and the booking/seat-hold logic.

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
│       ├── seat.go
│       ├── ticket.go
│       ├── *_test.go           # Integration tests for every repository
│       └── README.md           # Deep dive into the models/repository layer
├── handlers/                   # Gin handlers (HTTP layer)
│   ├── handlers.go             # Handler struct holding all repositories
│   ├── user.go                 # CreateUser handler + request validation
│   └── README.md               # Endpoint reference and handler conventions
├── router/
│   └── router.go               # Registers routes on the Gin engine (`POST /user`)
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

The server listens on **`http://localhost:8080`**. Only `POST /user` is registered
so far, so every other path returns `404` until the router is filled in.

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
shows (id, name, duration_minutes > 0, author_name)

seats (id, user_id → users.id NULLABLE, hall_id → halls.id,
       seat_number > 0, status 0|1|2, price >= 0)
       UNIQUE (hall_id, seat_number)

tickets (id, seat_id → seats.id, hall_id → halls.id, show_id → shows.id)
       UNIQUE (show_id, seat_id)   -- a seat is sold only once per show
```

### Seat status values

| Value | Name | Meaning |
|-------|------|---------|
| `0` | `Free` | Available |
| `1` | `Holding` | Temporarily reserved by a user (the "hold seat" color) |
| `2` | `Booked` | Paid/sold |

Relations: `User` → `Seats`, `Hall` → `Seats`, `Show` → `Tickets`,
`Seat` → `User` + `Hall`, `Ticket` → `Seat` + `Hall` + `Show`.

> ⚠️ **Schema note:** `seats.status` is stored per *physical seat*, not per
> *seat + show*. That is fine while the project has a single show per hall, but a
> seat held/booked for one show would look taken for another show. If multiple
> shows per hall are needed, move the status to a `show_seats` (show_id, seat_id,
> status) table and keep `tickets` as the proof of purchase.

---

## Booking Flow Design (Concurrency Notes)

These are the design targets for the core feature — nothing is implemented yet.

1. **No double booking.** Two users must never end up with `Booked` on the same
   seat for the same show. Enforced in **two places**:
   - The atomic `UPDATE seats SET user_id = $1, status = 2
     WHERE id = $2 AND status = 0` — the `WHERE status = 0` guard means only one
     writer can win.
   - The `UNIQUE (show_id, seat_id)` index on `tickets` as a last line of defense.
2. **Hold → Book lifecycle.** Booking is two phases:
   - *Hold*: atomically transition `Free (0) → Holding (1)` and record the user.
     The seat shows up as held (e.g. yellow) for everyone else.
   - *Book*: transition `Holding (1) → Booked (2)` and insert the ticket.
   - *Release*: if the hold times out or the user cancels, back to `Free (0)`.
3. **Row locking when needed.** For flows that read then write (e.g. choosing
   several seats at once, or checking a hold before confirming), use a
   transaction with `SELECT ... FOR UPDATE` so concurrent requests serialize
   instead of racing.
4. **Check the result, not just the error.** An `UPDATE ... WHERE status = 0`
   that matches zero rows is a conflict, not a database error — the handler must
   return `409 Conflict` in that case.
5. **Hold expiry.** Add an `expires_at` column (or a background sweeper) so
   abandoned holds do not stay colored forever.
6. **Live updates between users.** When a seat changes state, broadcast the new
   state (SSE / WebSocket) to the other ticket windows so every user sees the
   hold/book color immediately.
7. **Two parallel windows.** Model each booking window as an independent
   request path (goroutine/channel or simply concurrent HTTP requests). The
   database is the single source of truth, so the windows never need to share
   in-memory state.

### Suggested handler checklist

```
[ ] POST   /seats/:id/hold     -> Free → Holding (409 on conflict)
[ ] POST   /seats/:id/release  -> Holding → Free (owner only)
[ ] POST   /seats/:id/book     -> Holding → Booked + create ticket
[ ] GET    /halls/:id/seats    -> seats with status + holder for the UI
[ ] GET    /shows/:id/tickets  -> sold seats for a show
[ ] SSE /  /events             -> live seat status updates
```

---

## Roadmap

- [x] Connect to PostgreSQL with `pgxpool`
- [x] Write the schema as goose migrations
- [x] Implement models and repositories
- [x] Cover repositories with integration tests
- [x] Wire repositories into handlers and router
- [x] Implement `POST /user` (`CreateUser`)
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
