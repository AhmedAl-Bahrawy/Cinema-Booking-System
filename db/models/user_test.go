package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
)

func TestCreateUser(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		username string
		fullName string
		want     *models.User
		wantErr  bool
	}{
		{
			name:     "creates a user with valid data",
			username: "ahmed",
			fullName: "Ahmed Al-Bahrawy",
			want:     &models.User{ID: 1, Username: "ahmed", FullName: "Ahmed Al-Bahrawy"},
			wantErr:  false,
		},
		{
			// Postgres only rejects actual NULLs for NOT NULL columns;
			// an empty string is a valid value, so this succeeds.
			name:     "empty username is accepted (not NULL)",
			username: "",
			fullName: "Some FullName",
			want:     &models.User{ID: 1, Username: "", FullName: "Some FullName"},
			wantErr:  false,
		},
		{
			name:     "empty full_name is accepted (not NULL)",
			username: "no_full_name",
			fullName: "",
			want:     &models.User{ID: 1, Username: "no_full_name", FullName: ""},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewUserRepository(pool)

			got, gotErr := repo.Create(t.Context(), tt.username, tt.fullName)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Create() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID || got.Username != tt.want.Username || got.FullName != tt.want.FullName {
				t.Errorf("Create() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCreateUser_DuplicateUsername verifies the UNIQUE constraint on username:
// the second insert with the same username must fail.
func TestCreateUser_DuplicateUsername(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewUserRepository(pool)

	first, err := repo.Create(t.Context(), "dupe_user", "First User")
	if err != nil {
		t.Fatalf("Create() first insert failed: %v", err)
	}

	second, err := repo.Create(t.Context(), "dupe_user", "Second User")
	if err == nil {
		t.Errorf("Create() succeeded unexpectedly for duplicate username: %+v", second)
	}
	if second != nil {
		t.Errorf("Create() returned a user on error: %+v", second)
	}

	// The first user must be untouched by the failed insert.
	var fullName string
	err = pool.QueryRow(t.Context(), `SELECT full_name FROM users WHERE id = $1`, first.ID).Scan(&fullName)
	if err != nil {
		t.Fatalf("query first user: %v", err)
	}
	if fullName != "First User" {
		t.Errorf("full_name = %q, want %q", fullName, "First User")
	}
}

// TestCreateUser_PersistsRow verifies that Create actually wrote the row
// to the database, not just returned a struct.
func TestCreateUser_PersistsRow(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewUserRepository(pool)

	created, err := repo.Create(t.Context(), "persist_me", "Persisted User")
	if err != nil {
		t.Fatalf("Create() failed: %v", err)
	}

	if created.ID <= 0 {
		t.Fatalf("Create() returned non-positive ID: %d", created.ID)
	}

	var username, fullName string
	err = pool.QueryRow(t.Context(),
		`SELECT username, full_name FROM users WHERE id = $1`, created.ID,
	).Scan(&username, &fullName)
	if err != nil {
		t.Fatalf("query created user: %v", err)
	}

	if username != "persist_me" || fullName != "Persisted User" {
		t.Errorf("stored user = (%q, %q), want (%q, %q)",
			username, fullName, "persist_me", "Persisted User")
	}
}

func TestGetUserById(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		id      int64
		want    *models.User
		wantErr bool
	}{
		{
			name:    "returns the user for an existing id",
			id:      1,
			want:    &models.User{ID: 1, Username: "ahmed", FullName: "Ahmed Al-Bahrawy"},
			wantErr: false,
		},
		{
			// No user was seeded for this id, so no rows are returned.
			name:    "returns error for a non-existent id",
			id:      999,
			wantErr: true,
		},
		{
			// id is a BIGSERIAL starting at 1, so 0 can never exist.
			name:    "returns error for id 0",
			id:      0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewUserRepository(pool)

			// Seed a known user so the test is independent of other data.
			if _, err := repo.Create(t.Context(), "ahmed", "Ahmed Al-Bahrawy"); err != nil {
				t.Fatalf("seed user: %v", err)
			}

			got, gotErr := repo.GetById(t.Context(), tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetById() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetById() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetById() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID || got.Username != tt.want.Username || got.FullName != tt.want.FullName {
				t.Errorf("GetById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetAllUsers(t *testing.T) {
	t.Run("returns all users ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewUserRepository(pool)

		first, err := repo.Create(t.Context(), "ahmed", "Ahmed Al-Bahrawy")
		if err != nil {
			t.Fatalf("seed first user: %v", err)
		}
		second, err := repo.Create(t.Context(), "sara", "Sara Ali")
		if err != nil {
			t.Fatalf("seed second user: %v", err)
		}

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAll() returned %d users, want 2", len(got))
		}
		if got[0].ID != first.ID || got[0].Username != "ahmed" || got[0].FullName != "Ahmed Al-Bahrawy" {
			t.Errorf("GetAll()[0] = %+v, want %+v", got[0], first)
		}
		if got[1].ID != second.ID || got[1].Username != "sara" || got[1].FullName != "Sara Ali" {
			t.Errorf("GetAll()[1] = %+v, want %+v", got[1], second)
		}
	})

	t.Run("returns an empty list when there are no users", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewUserRepository(pool)

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAll() returned %d users, want 0", len(got))
		}
	})
}

func TestGetAllUsersWithDetails(t *testing.T) {
	pool := testdb.Setup(t)
	userRepo := models.NewUserRepository(pool)
	ticketRepo := models.NewTicketRepository(pool)

	first, err := userRepo.Create(t.Context(), "ahmed", "Ahmed Al-Bahrawy")
	if err != nil {
		t.Fatalf("seed first user: %v", err)
	}
	second, err := userRepo.Create(t.Context(), "sara", "Sara Ali")
	if err != nil {
		t.Fatalf("seed second user: %v", err)
	}
	hall := seedHall(t, pool)
	show := seedShow(t, pool)

	seat1 := seedSeat(t, pool, hall.ID, 1, 100)

	// The first user books the same physical seat for two shows on the same
	// day; the second user has no tickets.
	screening1 := seedScreening(t, pool, hall.ID, show.ID, screeningBase)
	screening2 := seedScreening(t, pool, hall.ID, show.ID, screeningBase.Add(2*time.Hour))

	ticket1, err := ticketRepo.Create(t.Context(), screening1.ID, seat1.ID, first.ID, models.Booked)
	if err != nil {
		t.Fatalf("seed first ticket: %v", err)
	}
	ticket2, err := ticketRepo.Create(t.Context(), screening2.ID, seat1.ID, first.ID, models.Booked)
	if err != nil {
		t.Fatalf("seed second ticket: %v", err)
	}

	got, err := userRepo.GetAllWithDetails(t.Context())
	if err != nil {
		t.Fatalf("GetAllWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllWithDetails() returned %d users, want 2", len(got))
	}

	// First user: two tickets attached.
	if got[0].ID != first.ID {
		t.Fatalf("GetAllWithDetails()[0].ID = %d, want %d", got[0].ID, first.ID)
	}
	if len(got[0].Tickets) != 2 {
		t.Fatalf("GetAllWithDetails()[0].Tickets has %d tickets, want 2", len(got[0].Tickets))
	}
	if got[0].Tickets[0].ID != ticket1.ID || got[0].Tickets[1].ID != ticket2.ID {
		t.Errorf("GetAllWithDetails()[0].Tickets = %+v", got[0].Tickets)
	}
	for i, ticket := range got[0].Tickets {
		if ticket.Seat == nil || ticket.Seat.ID != seat1.ID || ticket.Seat.SeatNumber != 1 {
			t.Errorf("GetAllWithDetails()[0].Tickets[%d].Seat = %+v", i, ticket.Seat)
		}
		if ticket.Seat.Hall == nil || ticket.Seat.Hall.ID != hall.ID {
			t.Errorf("GetAllWithDetails()[0].Tickets[%d].Seat.Hall = %+v", i, ticket.Seat.Hall)
		}
		if ticket.Screening == nil || ticket.Screening.Show == nil || ticket.Screening.Show.ID != show.ID {
			t.Errorf("GetAllWithDetails()[0].Tickets[%d].Screening = %+v", i, ticket.Screening)
		}
		if ticket.User != nil {
			t.Errorf("GetAllWithDetails()[0].Tickets[%d].User should be nil to avoid a cycle", i)
		}
	}

	// Second user: no tickets.
	if got[1].ID != second.ID {
		t.Fatalf("GetAllWithDetails()[1].ID = %d, want %d", got[1].ID, second.ID)
	}
	if len(got[1].Tickets) != 0 {
		t.Errorf("GetAllWithDetails()[1].Tickets has %d tickets, want 0", len(got[1].Tickets))
	}
}
