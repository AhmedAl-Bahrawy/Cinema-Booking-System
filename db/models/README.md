# `db/models` — Database Models & Queries

This folder contains **all SQL for the Cinema Booking System** plus the Go structs
that represent database rows. There is no ORM: every function writes explicit SQL
and scans the result with [`pgx`](https://github.com/jackc/pgx) v5.

If you come back to this code later, this file explains:

- [Folder layout](#folder-layout)
- [Database schema & relations](#database-schema--relations)
- [Go structs and tags](#go-structs-and-tags)
- [Function naming convention](#function-naming-convention)
- [The 3 query patterns used here](#the-3-query-patterns-used-here)
- [Important details (NULLs, uint32, JSON cycles)](#important-details)
- [Usage examples](#usage-examples)
- [Testing guide](#testing-guide)
- [How to add a new model](#how-to-add-a-new-model)

---

## Folder layout

| File | Model | Table | What it contains |
|------|-------|-------|------------------|
| `user.go` | `User` | `users` | Users of the cinema |
| `hall.go` | `Hall` | `halls` | Cinema halls (rooms) |
| `show.go` | `Show` | `shows` | Plays/movies that are shown |
| `seat.go` | `Seat` | `seats` | Seats inside a hall; optionally held/booked by a user |
| `ticket.go` | `Ticket` | `tickets` | Sold seat for a specific show |
| `*_test.go` | — | — | Integration tests against a throwaway PostgreSQL database |

Migrations live in [`../migrations`](../migrations) and are the source of truth for
the schema. Go structs only mirror them.

---

## Database schema & relations

```
users                      halls
  id (PK)                    id (PK)
  username (UNIQUE)          name (UNIQUE)
  full_name                  capacity >= 0
    │                          │
    │                          │ 1
    │                          ▼
    │                       seats
    │                        id (PK)
    └──────────────►         user_id  → users.id  (NULLABLE, ON DELETE SET NULL)
       0..n                  hall_id  → halls.id  (NOT NULL, ON DELETE CASCADE)
                             seat_number > 0
                             status 0|1|2
                             price >= 0
                             UNIQUE (hall_id, seat_number)
                               │
                               │ 1
                               ▼
shows                       tickets
  id (PK)                    id (PK)
  name                       seat_id → seats.id  (NOT NULL)
  duration_minutes > 0       hall_id → halls.id  (NOT NULL)
  author_name                show_id → shows.id  (NOT NULL)
                             UNIQUE (show_id, seat_id)
```

Key rules enforced by the database (and verified by tests):

- `users.username` is unique.
- `halls.name` is unique.
- A seat number is unique **per hall** (`UNIQUE (hall_id, seat_number)`).
- A seat can be sold **once per show** (`UNIQUE (show_id, seat_id)`).
- `seats.user_id` is **nullable** — a seat is free until someone holds/books it.
- Deleting a hall deletes its seats and tickets (`ON DELETE CASCADE`).
- Deleting a user keeps the seat but sets `user_id = NULL`.

### Booking status (`seat.go`)

```go
type BookingStatus int8

const (
    Free    BookingStatus = iota // 0
    Holding                      // 1
    Booked                       // 2
)
```

Stored as `SMALLINT` with `CHECK (status IN (0, 1, 2))`.

---

## Go structs and tags

Every struct has **three tag families**:

| Tag | Purpose |
|-----|---------|
| `db:"..."` | Column name (used to keep struct ↔ column mapping readable) |
| `json:"..."` | Field name in JSON responses (Gin) |
| `binding:"..."` | Gin validator rules for incoming request bodies |

Example:

```go
type Hall struct {
    ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
    Name     string `db:"name"     json:"name"     binding:"required,min=1,max=255"`
    Capacity uint32 `db:"capacity" json:"capacity" binding:"required,min=1"`
}
```

### Relation fields (`db:"-"`)

Relations are **never scanned directly**. They are filled by JOIN queries:

```go
// Seat has:
User *User `db:"-" json:"user,omitempty" binding:"-"`
Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`

// Ticket has:
Seat *Seat `db:"-" json:"seat,omitempty" binding:"-"`
Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
Show *Show `db:"-" json:"show,omitempty" binding:"-"`

// User has:   Seats []*Seat
// Hall has:   Seats []*Seat
// Show has:   Tickets []*Ticket
```

`db:"-"` means "don't map this to a column". `binding:"-"` means Gin ignores the
field when validating request bodies.

### Helper row structs

`UserSeat`, `HallSeat` and `ShowTicket` are **not models** — they exist only to hold
one raw row of a one-to-many JOIN before the rows are grouped into
`User.Seats`, `Hall.Seats` and `Show.Tickets`. They are intentionally unexported-ish
(exported only for `pgx.CollectRows`), have no JSON/binding tags, and must not be
used outside their file.

---

## Function naming convention

Each model exposes the same predictable set of functions:

| Function | Returns | SQL |
|----------|---------|-----|
| `CreateX(...)` | `*X` | `INSERT ... RETURNING ...` |
| `GetXById(...)` | `*X` | `SELECT ... WHERE id = $1` |
| `GetAllX(...)` | `[]*X` | `SELECT ... ORDER BY id` |
| `GetXWithDetails(...)` | `*X` with relations | `SELECT ... JOIN ... WHERE id = $1` |
| `GetAllXWithDetails(...)` | `[]*X` with relations | `SELECT ... JOIN ... ORDER BY id` |

"Details" means the relation fields are filled:

| Model | Relations filled by `...WithDetails` |
|-------|--------------------------------------|
| `User` | `Seats` |
| `Hall` | `Seats` |
| `Show` | `Tickets` |
| `Seat` | `User` (nil when free), `Hall` |
| `Ticket` | `Seat` (with `Seat.Hall`), `Hall`, `Show` |

"Plain" functions (`GetXById`, `GetAllX`) never JOIN; relation fields stay `nil`.
That keeps the common, cheap queries fast.

---

## The 3 query patterns used here

### 1. Create — `INSERT ... RETURNING` + `QueryRow`

```go
func CreateUser(ctx context.Context, pool *pgxpool.Pool, username, fullName string) (*User, error) {
    query := `INSERT INTO users (username, full_name) VALUES ($1, $2) RETURNING id, username, full_name`

    var user User
    err := pool.QueryRow(ctx, query, username, fullName).Scan(&user.ID, &user.Username, &user.FullName)
    if err != nil {
        return nil, err
    }

    return &user, nil
}
```

`RETURNING` gives back the generated `id`, so the returned struct is complete
without a second SELECT.

### 2. Single row — `QueryRow` (plain) or `CollectOneRow` (with JOINs)

Plain by-id lookup:

```go
var user User
err := pool.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.FullName)
```

If nothing matches, `pgx` returns `pgx.ErrNoRows` — tests assert exactly that.

Single row **with joins** uses `pgx.CollectOneRow`, which shares its scan logic
with the list version:

```go
rows, err := pool.Query(ctx, query, id)
if err != nil {
    return nil, fmt.Errorf("query seat with details: %w", err)
}

seat, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (*Seat, error) {
    return scanSeatWithDetails(row) // same helper used by GetAllSeatsWithDetails
})
```

### 3. Many rows — `pgx.CollectRows` (no manual `rows.Next()` loops)

Simple list:

```go
rows, err := pool.Query(ctx, query)
if err != nil {
    return nil, fmt.Errorf("query users: %w", err)
}

users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*User, error) {
    var user User
    err := row.Scan(&user.ID, &user.Username, &user.FullName)
    return &user, err
})
if err != nil {
    return nil, fmt.Errorf("collect users: %w", err)
}
```

`CollectRows` closes the rows automatically, so **no `defer rows.Close()`** is
needed. Errors are wrapped with `%w` and a message that says whether the
failure happened while querying or while collecting.

### 3b. Many rows **with a one-to-many relation**

A JOIN returns one row per child, so the parent repeats:

```
u.id | username | s.id
-----+----------+-----
   1 | ahmed    |    1
   1 | ahmed    |    2
   2 | sara     | NULL   ← user with no seats (LEFT JOIN)
```

Step 1 — collect the flat rows into the helper struct (`UserSeat`):

```go
raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (UserSeat, error) {
    var r UserSeat
    err := row.Scan(&r.userID, &r.username, &r.fullName, &r.seatID, &r.hallId, ...)
    return r, err
})
```

Step 2 — group them by parent id with a map:

```go
users := make([]*User, 0, len(raw))
byID := make(map[int64]*User)
for _, r := range raw {
    user, ok := byID[r.userID]
    if !ok {
        user = &User{ID: r.userID, Username: r.username, FullName: r.fullName}
        byID[r.userID] = user
        users = append(users, user)
    }
    if r.seatID != nil { // nil for a parent with no children
        user.Seats = append(user.Seats, &Seat{...})
    }
}
```

This is still **one SQL query** — the map just assembles the result. Order is
guaranteed by `ORDER BY parent_id, child_id`.

---

## Important details

### NULL columns are scanned into pointers

`seats.user_id` is nullable. Scanning a NULL into `int64` fails, so we scan into
`*int64`, which stays `nil` for NULL:

```go
var dbUserId *int64
// ...
Scan(&seat.ID, &dbUserId, ...)
if dbUserId != nil {
    seat.UserId = *dbUserId
}
```

The public struct still uses `int64`; `0` means "no user". `CreateSeat` does the
reverse: `userId == 0` is sent to Postgres as `NULL`.

### Why `uint32` fields are scanned via `int64`

Postgres `INTEGER` is signed 32-bit; the structs use `uint32` for `capacity`,
`seat_number` and `price` because the schema forbids negatives. Scanning `INTEGER`
directly into `uint32` is awkward in pgx, so we scan into a temporary `int64` and
convert:

```go
var dbCapacity int64
row.Scan(&hall.ID, &hall.Name, &dbCapacity)
hall.Capacity = uint32(dbCapacity)
```

`seats.status` (`SMALLINT`) is scanned into `int16` in helper structs and cast to
`BookingStatus`.

### `JOIN` vs `LEFT JOIN`

- `JOIN` when the relation **must** exist: every seat has a hall, every ticket has
  a seat/hall/show, every seat belongs to a hall.
- `LEFT JOIN` when the relation is **optional**: a seat may have no user; a user,
  hall or show may have no seats/tickets. Those parents must still appear in the
  result, with `NULL` child columns.

### Avoiding infinite JSON cycles

`Hall` contains `Seats`, and `Seat` has a `Hall *Hall` pointer. If both were
filled, JSON encoding would loop forever:

```
Hall → Seats → Seat.Hall → Seats → Seat.Hall → ... 💥
```

So when filling a parent's child list, the **back-reference is deliberately left
nil**:

- `GetAllHallsWithDetails` leaves `seat.Hall == nil`
- `GetAllUsersWithDetails` leaves `seat.User == nil`
- `GetAllShowsWithDetails` leaves `ticket.Show == nil`

This is noted with a comment at each place. Do not "fix" it.

At the top level the other direction is safe: `GetAllSeatsWithDetails` sets
`seat.Hall` and `seat.User`, because `Hall.Seats`/`User.Seats` are left nil there.

---

## Usage examples

```go
ctx := context.Background()

// Create
user, err := models.CreateUser(ctx, pool, "ahmed", "Ahmed Al-Bahrawy")
hall, err := models.CreateHall(ctx, pool, "Hall A", 100)
show, err := models.CreateShow(ctx, pool, "Hamlet", 120, "Shakespeare")

// A free seat has no user yet (userId = 0 → NULL in the database).
seat, err := models.CreateSeat(ctx, pool, 0, hall.ID, 1, models.Free, 100)

// Hold/book the seat for the user.
seat, err = models.CreateSeat(ctx, pool, user.ID, hall.ID, 2, models.Booked, 150)

// Sell the seat for a show.
ticket, err := models.CreateTicket(ctx, pool, seat.ID, hall.ID, show.ID)

// Plain reads (no JOINs)
users, err := models.GetAllUsers(ctx, pool)

// Reads with relations
seat, err = models.GetSeatWithDetails(ctx, pool, seat.ID)   // seat.User, seat.Hall
ticket, err = models.GetTicketWithDetails(ctx, pool, ticket.ID) // ticket.Seat/Hall/Show
hall, err = models.GetAllHallsWithDetails(ctx, pool)        // hall.Seats (per hall)
```

Checking for a missing row:

```go
_, err := models.GetUserById(ctx, pool, 999)
if errors.Is(err, pgx.ErrNoRows) {
    // not found
}
```

---

## Testing guide

Tests live next to the code (`user_test.go`, `hall_test.go`, ...) and are
**integration tests** against a real PostgreSQL database — no mocks.

### How `internal/testdb` works

```go
pool := testdb.Setup(t)
```

`Setup` (see [`../../internal/testdb/testdb.go`](../../internal/testdb/testdb.go)):

1. Reads `TEST_DATABASE_URL`, or `GOOSE_DBSTRING` from the repo-root `.env`.
   If neither exists, the test is **skipped** (so `go test ./...` still works
   without a database).
2. Drops and recreates the throwaway database `cinema_booking_test`.
3. Applies every Up migration from `db/migrations`.
4. Registers cleanups: truncate all tables, close the pool, drop the database.

Because the database is recreated per test, seeding inside a test is safe and
isolated.

### Test style

Every function has a table-driven test:

```go
tests := []struct {
    name    string
    id      int64
    want    *models.User
    wantErr bool
}{ ... }

for _, tt := range tests {
    t.Run(tt.name, func(t *testing.T) {
        pool := testdb.Setup(t)
        // seed parents ...
        got, gotErr := models.GetUserById(t.Context(), pool, tt.id)
        // assert ...
    })
}
```

The tests also cover database constraints beyond the happy path:

| Test | Constraint covered |
|------|--------------------|
| `TestCreateUser_DuplicateUsername` | `users.username UNIQUE` |
| `TestCreateHall_DuplicateName` | `halls.name UNIQUE` |
| `TestCreateShow` (zero duration) | `duration_minutes > 0` |
| `TestCreateSeat_DuplicateSeatNumberInHall` | `UNIQUE (hall_id, seat_number)` |
| `TestCreateTicket_DuplicateSeatForShow` | `UNIQUE (show_id, seat_id)` |
| `TestCreateTicket` (bad FK ids) | foreign keys |
| `TestGetXById` (id `0`, `999`) | `pgx.ErrNoRows` |

Shared seed helpers (defined once, reused by other test files in the same
`models_test` package):

- `seedHall`, `seedUser`, `seedShow` — create one known parent.
- `seedTicketParents` — creates a hall + seat + show for ticket tests.

> Note: since `User`, `Hall` and `Show` now contain slices, they are not
> comparable with `*got != *tt.want`. Compare their fields individually in tests.

### Run the tests

```bash
# all tests
go test ./...

# only the models package, verbose
go test ./db/models -v

# one test / one subtest
go test ./db/models -run 'TestGetAllSeatsWithDetails'
```

---

## How to add a new model

Follow the same 5-function pattern so the rest of the codebase stays predictable:

1. **Migration** — add `db/migrations/<timestamp>_<name>.sql` with Up/Down.
2. **Struct** in `db/models/<name>.go` with `db`/`json`/`binding` tags.
   Add relation fields only if useful, always with `db:"-"`.
3. **Functions**:
   - `CreateX` → `INSERT ... RETURNING` + `QueryRow`.
   - `GetXById` → `QueryRow`; let `pgx.ErrNoRows` bubble up.
   - `GetAllX` → `pool.Query` + `pgx.CollectRows`.
   - `GetXWithDetails` → JOIN + `pgx.CollectOneRow` + shared scanner.
   - `GetAllXWithDetails` → JOIN + `pgx.CollectRows` (+ grouping map for
     one-to-many).
4. **Tests** in `<name>_test.go` using `testdb.Setup(t)`:
   - create happy path + constraint failure,
   - get existing / non-existent (`pgx.ErrNoRows`) / `0`,
   - `GetAll` with rows **and** empty table,
   - `GetAll...WithDetails` with a parent that has children **and** one with none.
5. **`internal/testdb/testdb.go`** — add the new table to the `TRUNCATE` cleanup:
   ```go
   TRUNCATE users, halls, shows, seats, tickets, <new_table> RESTART IDENTITY CASCADE
   ```

### Quick reference checklist

```
[ ] migration Up + Down
[ ] struct + tags
[ ] CreateX
[ ] GetXById
[ ] GetAllX
[ ] GetXWithDetails (if relations exist)
[ ] GetAllXWithDetails (if relations exist)
[ ] tests: create / get / get-by-bad-id / getAll / details
[ ] add table to TRUNCATE list
[ ] go build ./... && go vet ./... && go test ./...
```

---

## Mental model in one sentence

> Each file owns one table; every function is one explicit SQL statement, scanned
> either with `QueryRow` (single row), `CollectOneRow` (single row + JOINs) or
> `CollectRows` (lists), and "Details" functions only add the JOINs + relation
> assembly needed to fill the `db:"-"` fields.
