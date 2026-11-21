# `db/models` — Database Models & Queries

This folder contains **all SQL for the Cinema Booking System** plus the Go structs
that represent database rows. There is no ORM: every function writes explicit SQL
and scans the result with [`pgx`](https://github.com/jackc/pgx) v5.

If you come back to this code later, this file explains:

- [Folder layout](#folder-layout)
- [Database schema & relations](#database-schema--relations)
- [Go structs and tags](#go-structs-and-tags)
- [Request structs](#request-structs)
- [Repository naming convention](#repository-naming-convention)
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
| `screening.go` | `Screening` | `screenings` | A show playing in a hall during a time slot |
| `seat.go` | `Seat` | `seats` | Physical seats inside a hall (no user, no status) |
| `seat_category.go` | `SeatCategory` | `seat_categories` | Price multiplier per seat level (normal/premium/VIP) |
| `ticket.go` | `Ticket` | `tickets` | A user's seat for one screening (the booking) |
| `*_test.go` | — | — | Integration tests against a throwaway PostgreSQL database |

Migrations live in [`../migrations`](../migrations) and are the source of truth for
the schema. Go structs only mirror them.

`user.go` also defines `UserReq`, a small **request payload** struct used by the
HTTP layer for `POST /user`. It is not a table model and is never scanned from the
database. See [Request structs](#request-structs) below.

---

## Database schema & relations

```
users                          halls
  id (PK)                        id (PK)
  username (UNIQUE)              name (UNIQUE)
  full_name                      capacity >= 0
    │                              │
    │                              │ 1
    │                              ▼
    │                           seats
    │                            id (PK)
    │                            hall_id → halls.id (NOT NULL, ON DELETE CASCADE)
    │                            seat_number > 0
    │                            category_id → seat_categories.id
    │                            UNIQUE (hall_id, seat_number)
    │
    │                          screenings
    │                            id (PK)
    │                            hall_id → halls.id (NOT NULL, ON DELETE CASCADE)
    │                            show_id → shows.id (NOT NULL, ON DELETE CASCADE)
    │                            starts_at, ends_at (TIMESTAMPTZ)
    │                            CHECK (ends_at > starts_at)
    │                            EXCLUDE (hall_id =, [starts_at, ends_at) &&)
    │                              │
    │                              │ 1
    │                              ▼
    └──────────────────────►   tickets
                               id (PK)
                               screening_id → screenings.id (NOT NULL)
                               seat_id      → seats.id      (NOT NULL)
                               user_id      → users.id      (NOT NULL)
                               number > 0   -- = seats.seat_number
                               price >= 0   -- snapshot computed at sale
                               status 1|2|3
                               UNIQUE (screening_id, seat_id)
                               UNIQUE (screening_id, number)

seat_categories
  id (PK)
  name (UNIQUE)
  price_multiplier_percent > 0   -- normal 100, premium 150, vip 200

shows
  id (PK)
  name
  duration_minutes > 0
  author_name
  price >= 0   -- base price for a normal seat
    │
    └──< screenings >── halls
```

Key rules enforced by the database (and verified by tests):

- `users.username` is unique.
- `halls.name` is unique.
- A seat number is unique **per hall** (`UNIQUE (hall_id, seat_number)`).
- A seat is **physical**: it has no `user_id` and no `status`. Its category
  (`category_id`) decides the price multiplier.
- A ticket's `number` is the hall seat number and is unique per screening
  (`UNIQUE (screening_id, number)`), also enforced by the
  `tickets_seat_number_check` trigger.
- A ticket's `price` is computed at sale time:
  `ROUND(show.price × seat_category.price_multiplier_percent / 100)`.
- A **screening** is a show in a hall at a time. The exclusion constraint
  (`EXCLUDE USING gist (hall_id WITH =, tstzrange(starts_at, ends_at) WITH &&)`)
  guarantees a hall never plays two shows at the same time. It can play a
  different show later on the same day.
- A ticket is unique **per screening and seat** (`UNIQUE (screening_id, seat_id)`):
  a seat is sold once per screening, but is free again for the next screening.
- A ticket's seat must belong to the hall of the screening. The repository's
  `INSERT ... SELECT` checks it, and the `tickets_seat_hall_check` trigger
  enforces it in the database.
- Deleting a hall deletes its seats and screenings (`ON DELETE CASCADE`).
- Deleting a user deletes that user's tickets (`ON DELETE CASCADE`).

### Ticket status (`ticket.go`)

```go
type TicketStatus int16

const (
    Held      TicketStatus = 1 // temporarily reserved
    Booked    TicketStatus = 2 // paid / confirmed
    Cancelled TicketStatus = 3 // released again
)
```

Stored as `SMALLINT` with `CHECK (status IN (1, 2, 3))`.

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
Category *SeatCategory `db:"-" json:"category,omitempty" binding:"-"`
Hall     *Hall         `db:"-" json:"hall,omitempty"     binding:"-"`

// Screening has:
Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
Show *Show `db:"-" json:"show,omitempty" binding:"-"`

// Ticket has:
Seat      *Seat      `db:"-" json:"seat,omitempty"      binding:"-"`
User      *User      `db:"-" json:"user,omitempty"      binding:"-"`
Screening *Screening `db:"-" json:"screening,omitempty" binding:"-"`

// User has:        Tickets []*Ticket
// Hall has:        Seats []*Seat, Screenings []*Screening
// Show has:        Screenings []*Screening
```

`db:"-"` means "don't map this to a column". `binding:"-"` means Gin ignores the
field when validating request bodies.

### Request structs

Models are shaped for **database rows and responses**. For incoming JSON bodies
we use a separate request struct so clients cannot set fields they do not own
(`id`, relations, `status`, ...).

```go
type UserReq struct {
    Username string `db:"username"  json:"username"  binding:"required,min=3,max=100"`
    FullName string `db:"full_name" json:"full_name" binding:"required,min=3,max=255"`
}
```

- Used by `handlers.CreateUser` with `c.ShouldBindJSON(&newUser)`.
- `binding` tags are the first line of validation; the handler also re-checks the
  lengths in `validateUser`.
- The `db` tags are informational only — this struct is never passed to `pgx`.
- Add a `*Req` struct next to the model when a handler needs a different payload
  from the full model (e.g. a future `SeatBookReq`).

### Helper row structs

`UserTicket`, `HallSeat`, `HallScreening` and `ShowScreening` are **not models** —
they exist only to hold one raw row of a one-to-many JOIN before the rows are
grouped into `User.Tickets`, `Hall.Seats`/`Hall.Screenings` and
`Show.Screenings`. They are intentionally unexported-ish (exported only for
`pgx.CollectRows`), have no JSON/binding tags, and must not be used outside their
file.

---

## Repository naming convention

Each model has its own repository and constructor:

```go
type UserRepository struct {
    db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
    return &UserRepository{db: db}
}
```

Repositories share a predictable set of methods:

| Method | Returns | SQL | Present on |
|--------|---------|-----|------------|
| `Create(...)` | `*X` | `INSERT ... RETURNING ...` | all models |
| `GetById(...)` | `*X` | `SELECT ... WHERE id = $1` | all models |
| `GetAll(...)` | `[]*X` | `SELECT ... ORDER BY ...` | all models |
| `GetWithDetails(...)` | `*X` with relations | `SELECT ... JOIN ... WHERE id = $1` | `Screening`, `Seat`, `Ticket` |
| `GetAllWithDetails(...)` | `[]*X` with relations | `SELECT ... JOIN ... ORDER BY ...` | all models |
| `GetByHall(...)` | `[]*X` | `WHERE hall_id = $1` | `Seat`, `Screening` |
| `GetByShow(...)` | `[]*Screening` | `WHERE show_id = $1` | `Screening` |
| `GetByScreening(...)` | `[]*Ticket` | `WHERE screening_id = $1` | `Ticket` |
| `GetByUser(...)` | `[]*Ticket` | `WHERE user_id = $1` | `Ticket` |
| `CreateAt(...)` | `*Screening` | computes `ends_at` from the show duration | `Screening` |

`User`, `Hall` and `Show` only have the list variant (`GetAllWithDetails`)
because they are the "parent" side of a one-to-many relation; `Screening`,
`Seat` and `Ticket` also have a single-row variant.

"Details" means the relation fields are filled:

| Model | Relations filled by `GetAllWithDetails` (and `GetWithDetails` where present) |
|-------|--------------------------------------|
| `User` | `Tickets` (each with `Seat` → `Hall`, and `Screening` → `Hall` + `Show`) |
| `Hall` | `Seats`, `Screenings` (each with `Show`) |
| `Show` | `Screenings` (each with `Hall`) |
| `Screening` | `Hall`, `Show` |
| `Seat` | `Hall` |
| `Ticket` | `Seat` (with `Seat.Hall`), `User`, `Screening` (with `Screening.Hall` + `Screening.Show`) |

"Plain" methods (`GetById`, `GetAll`) never JOIN; relation fields stay `nil`.
That keeps the common, cheap queries fast.

---

## The 3 query patterns used here

### 1. Create — `INSERT ... RETURNING` + `QueryRow`

```go
func (r *UserRepository) Create(ctx context.Context, username, fullName string) (*User, error) {
    query := `INSERT INTO users (username, full_name) VALUES ($1, $2) RETURNING id, username, full_name`

    var user User
    err := r.db.QueryRow(ctx, query, username, fullName).Scan(&user.ID, &user.Username, &user.FullName)
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
err := r.db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.FullName)
```

If nothing matches, `pgx` returns `pgx.ErrNoRows` — tests assert exactly that.

Single row **with joins** uses `pgx.CollectOneRow`, which shares its scan logic
with the list version:

```go
rows, err := r.db.Query(ctx, query, id)
if err != nil {
    return nil, fmt.Errorf("query seat with details: %w", err)
}

seat, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (*Seat, error) {
    return scanSeatWithDetails(row) // same helper used by GetAllWithDetails
})
```

### 3. Many rows — `pgx.CollectRows` (no manual `rows.Next()` loops)

Simple list:

```go
rows, err := r.db.Query(ctx, query)
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

A LEFT JOIN parent with no children produces NULL child columns, and ticket
relations are optional in one-to-many queries. Scanning a NULL into `int64`
fails, so we scan into `*int64` / `*string`, which stay `nil` for NULL:

```go
var dbSeatID *int64
// ...
Scan(&userID, &dbSeatID, ...)
if dbSeatID != nil {
    // attach the child
}
```

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

`tickets.status` (`SMALLINT`) is scanned into `int16` in helper structs and cast
to `TicketStatus`.

### `JOIN` vs `LEFT JOIN`

- `JOIN` when the relation **must** exist: every seat has a hall, every ticket has
  a seat, user and screening, every screening has a hall and a show.
- `LEFT JOIN` when the relation is **optional**: a user, hall or show may have no
  tickets/seats/screenings. Those parents must still appear in the result, with
  `NULL` child columns.

### Avoiding infinite JSON cycles

`Hall` contains `Seats`, and `Seat` has a `Hall *Hall` pointer. If both were
filled, JSON encoding would loop forever:

```
Hall → Seats → Seat.Hall → Seats → Seat.Hall → ... 💥
```

The same problem exists for `Hall → Screenings → Screening.Hall`,
`Show → Screenings → Screening.Show`, `User → Tickets → Ticket.User`,
`Screening → Tickets → Ticket.Screening`, and so on.

So when filling a parent's child list, the **back-reference is deliberately left
nil**:

- `HallRepository.GetAllWithDetails` leaves `seat.Hall` and `screening.Hall`
  nil, and `screening.Show.Screenings` nil
- `UserRepository.GetAllWithDetails` leaves `ticket.User` nil
- `ShowRepository.GetAllWithDetails` leaves `screening.Show` nil
- `TicketRepository` leaves every back-reference (`Seat.Tickets`, `User.Tickets`,
  `Screening.Tickets`, child lists) nil

This is noted with a comment at each place. Do not "fix" it.

At the top level the other direction is safe: `SeatRepository.GetWithDetails` sets
`seat.Hall`, `ScreeningRepository.GetWithDetails` sets `Hall` + `Show`, and
`TicketRepository.GetWithDetails` sets `Seat`/`User`/`Screening`, because the
parent child-lists are left nil there.

---

## Usage examples

```go
ctx := context.Background()

users := models.NewUserRepository(pool)
halls := models.NewHallRepository(pool)
shows := models.NewShowRepository(pool)
screenings := models.NewScreeningRepository(pool)
seats := models.NewSeatRepository(pool)
tickets := models.NewTicketRepository(pool)

// Create the catalogue and the room.
user, err := users.Create(ctx, "ahmed", "Ahmed Al-Bahrawy")
hall, err := halls.Create(ctx, "Hall A", 100)
show, err := shows.Create(ctx, "Hamlet", 120, "Shakespeare")

// Physical seats belong to the hall, not to a user.
seat, err := seats.Create(ctx, hall.ID, 1, 100)

// Schedule Hamlet in Hall A. EndsAt is calculated from the show duration
// (120 minutes), so this screening is 10:00–12:00.
screening, err := screenings.CreateAt(ctx, hall.ID, show.ID, startsAt)

// A later show in the same hall is allowed because it does not overlap.
another, err := screenings.CreateAt(ctx, hall.ID, show.ID, startsAt.Add(3*time.Hour))

// Sell the seat to the user for the first screening.
ticket, err := tickets.Create(ctx, screening.ID, seat.ID, user.ID, models.Booked)

// The same physical seat can be sold again for the later screening, even to
// the same user.
ticket2, err := tickets.Create(ctx, another.ID, seat.ID, user.ID, models.Booked)

// Plain reads (no JOINs)
allUsers, err := users.GetAll(ctx)

// Reads with relations
seat, err = seats.GetWithDetails(ctx, seat.ID)             // seat.Hall
ticket, err = tickets.GetWithDetails(ctx, ticket.ID)       // ticket.Seat/User/Screening (with Hall + Show)
screening, err = screenings.GetWithDetails(ctx, screening.ID) // screening.Hall/Show
allHalls, err := halls.GetAllWithDetails(ctx)              // hall.Seats + hall.Screenings
```

Checking for a missing row:

```go
_, err := users.GetById(ctx, 999)
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
        repo := models.NewUserRepository(pool)
        // seed parents ...
        got, gotErr := repo.GetById(t.Context(), tt.id)
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
| `TestCreateSeatCategory_DuplicateName` | `seat_categories.name UNIQUE` |
| `TestCreateSeatCategory` (zero multiplier) | `price_multiplier_percent > 0` |
| `TestCreateScreening_Overlap` | `screenings_hall_no_overlap` EXCLUDE constraint |
| `TestCreateScreening` (bad times) | `screenings_time_valid` CHECK |
| `TestCreateScreening_SameHallDifferentShowsSameDay` | a hall can change show later in the day |
| `TestCreateTicket_DoubleBooking` | `UNIQUE (screening_id, seat_id)` |
| `TestCreateTicket_PricingBySeatCategory` | ticket number + price formula per category |
| `TestCreateTicket_SeatReusedAcrossScreenings` | a seat is reusable after a screening |
| `TestCreateTicket_SeatHallMustMatchScreening` | ticket seat/hall consistency |
| `TestCreateTicket` (bad FK ids) | foreign keys |
| `TestGetXById` (id `0`, `999`) | `pgx.ErrNoRows` |

Shared seed helpers (defined once, reused by other test files in the same
`models_test` package, in `helpers_test.go`):

- `seedHall`, `seedUser`, `seedShow` — create one known parent.
- `seedSeatCategory` — creates a custom seat category.
- `seedSeat` — creates a physical seat in a hall.
- `seedScreening` — schedules a show in a hall via `CreateAt`.
- `seedTicketParents` — creates a hall + seat + show + screening for ticket tests.

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

Follow the same repository pattern so the rest of the codebase stays predictable:

1. **Migration** — add `db/migrations/<timestamp>_<name>.sql` with Up/Down.
2. **Struct** in `db/models/<name>.go` with `db`/`json`/`binding` tags.
   Add relation fields only if useful, always with `db:"-"`.
3. **Repository**:
   - `type XRepository struct { db *pgxpool.Pool }`
   - `func NewXRepository(db *pgxpool.Pool) *XRepository`
   - `Create` → `INSERT ... RETURNING` + `QueryRow`.
   - `GetById` → `QueryRow`; let `pgx.ErrNoRows` bubble up.
   - `GetAll` → `r.db.Query` + `pgx.CollectRows`.
   - `GetWithDetails` → JOIN + `pgx.CollectOneRow` + shared scanner.
   - `GetAllWithDetails` → JOIN + `pgx.CollectRows` (+ grouping map for
     one-to-many).
4. **Tests** in `<name>_test.go` using `testdb.Setup(t)`:
   - create happy path + constraint failure,
   - get existing / non-existent (`pgx.ErrNoRows`) / `0`,
   - `GetAll` with rows **and** empty table,
   - `GetAll...WithDetails` with a parent that has children **and** one with none.
5. **`internal/testdb/testdb.go`** — add the new table to the `TRUNCATE` cleanup:
   ```go
   TRUNCATE users, halls, shows, seats, screenings, tickets, <new_table> RESTART IDENTITY CASCADE
   ```

### Quick reference checklist

```
[ ] migration Up + Down
[ ] struct + tags
[ ] XRepository + NewXRepository
[ ] Create
[ ] GetById
[ ] GetAll
[ ] GetWithDetails (if relations exist)
[ ] GetAllWithDetails (if relations exist)
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
