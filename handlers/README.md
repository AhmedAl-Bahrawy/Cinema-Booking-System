# `handlers` — HTTP Layer

This folder contains the Gin handlers for the Cinema Booking System. A handler is
the thin layer between an HTTP request and a repository: it binds/validates the
request, calls the repository, and turns the result into JSON plus a status code.

Request flow:

```
main.go → db.ConnectDB → models.New*Repository(pool)
        → handlers.NewHandler(...) → router.SetupRouter(h) → r.Run(":8080")
```

---

## Folder layout

| File | What it contains |
|------|------------------|
| `handlers.go` | The `Handler` struct (one repository per resource) and `NewHandler` |
| `user.go` | `CreateUser` handler + `validateUser` helper |

```go
type Handler struct {
    Halls   *models.HallRepository
    Seats   *models.SeatRepository
    Shows   *models.ShowRepository
    Tickets *models.TicketRepository
    Users   *models.UserRepository
}
```

`NewHandler` receives the repositories from `main.go`, so handlers never create
their own database connections and are easy to wire/test.

---

## Current endpoints

| Method | Path | Handler | Status |
|--------|------|---------|--------|
| `POST` | `/user` | `CreateUser` | ✅ Implemented |

Everything else returns `404` until the remaining routes are registered in
[`../router/router.go`](../router/router.go).

### `POST /user`

Creates a new user. The request body is bound to `models.UserReq` (defined in
[`../db/models/user.go`](../db/models/user.go)), which only accepts the fields a
client is allowed to set.

**Request**

```http
POST /user
Content-Type: application/json

{
  "username": "ahmed",
  "full_name": "Ahmed Al-Bahrawy"
}
```

**Success — `201 Created`**

```json
{ "message": "user created successfully" }
```

**Failure — `400 Bad Request`**

```json
{ "error": "JSON can't unmarshal body" }
```

Returned when the body is not valid JSON or fails the `binding` tags.

```json
{ "error": "username length is lower or longer than must." }
{ "error": "fullName length is lower or longer than must." }
```

Returned by `validateUser` when `username` is outside `3..100` characters or
`full_name` is outside `3..255` characters.

```json
{ "error": "new user can not be created!!" }
```

Returned when `UserRepository.Create` fails (for example, a duplicate
`username`, which violates the `users.username UNIQUE` constraint).

**Try it**

```bash
curl -i -X POST http://localhost:8080/user \
  -H 'Content-Type: application/json' \
  -d '{"username":"ahmed","full_name":"Ahmed Al-Bahrawy"}'
```

---

## Handler conventions

When adding a handler, keep it in the file for its resource (`hall.go`,
`seat.go`, ...) and follow the same shape as `CreateUser`:

1. **Method on `*Handler`** — e.g. `func (h *Handler) GetUser(c *gin.Context)`.
2. **Request context** — `ctx := c.Request.Context()` and pass it to the repository.
3. **Bind into a request struct** — use a `models.XReq` struct (not the full
   model) for bodies, and `c.ShouldBindJSON`. Use `c.Param`/`c.Query` for IDs and
   filters.
4. **Validate** — check lengths/values and return `400` before touching the DB.
5. **Call the repository** — handlers do not write SQL.
6. **Map errors to status codes** — not every repository error is a `400`:
   - `pgx.ErrNoRows` → `404 Not Found`
   - unique violation (Postgres `23505`) → `409 Conflict`
   - foreign-key violation (`23503`) → `400`/`422`
   - anything else → `500 Internal Server Error`
7. **Return JSON** — a small `gin.H{...}` body and the right status code.
8. **Register the route** — add it in `router.SetupRouter`.

### Checklist for a new handler

```
[ ] request struct in db/models/<resource>.go (if the body is not a full model)
[ ] func (h *Handler) Xxx(c *gin.Context)
[ ] bind + validate → 400 on bad input
[ ] call repository with c.Request.Context()
[ ] map errors (404 / 409 / 500) instead of returning 400 for everything
[ ] success response with the correct 2xx code
[ ] route registered in router/router.go
[ ] handler test (httptest + a test database)
```

---

## Known issues / next steps

The current `CreateUser` is a first version; these are the things to fix as the
HTTP layer grows:

- **`log.Fatal` exits the server.** Both `validateUser` failures and repository
  failures call `log.Fatal`, which terminates the whole process instead of
  returning an error response. Replace it with `c.JSON(...)` + `return`.
- **`validateUser` writes the response itself.** It takes `*gin.Context`, emits
  the `400`, and also returns an `error`. A validation helper should be pure
  (return an error, let the handler respond) so a response is never written
  twice.
- **Every DB error becomes `400`.** A duplicate username should be `409`, and
  unexpected failures should be `500`. Inspect `pgconn.PgError` codes.
- **No read/update/delete user routes.** Only create exists so far.
- **No handler tests yet.** Model/repository tests live in `db/models`.
- **Route naming is singular** (`/user`). Decide on `/users` (and `/users/:id`)
  before adding more user routes, then keep it consistent.

## To Do

- [x] Create handler for `User` (`CreateUser`)
- [x] Wire `POST /user` into the router
- [ ] Create handlers for `Hall`, `Show`, `Seat`, `Ticket`
- [ ] Add `GetAll` / `GetById` / `Update` / `Delete` handlers
- [ ] Return proper status codes (`404`, `409`, `500`) instead of `400`/`log.Fatal`
- [ ] Implement the seat hold → book flow (`POST /seats/:id/hold`, `.../book`,
      `.../release`) with `409 Conflict` on races
- [ ] Add handler tests
- [ ] Implement that a user can book a ticket end-to-end
