package models_test

import (
	"errors"
	"testing"

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

			got, gotErr := models.CreateUser(t.Context(), pool, tt.username, tt.fullName)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateUser() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CreateUser() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID || got.Username != tt.want.Username || got.FullName != tt.want.FullName {
				t.Errorf("CreateUser() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCreateUser_DuplicateUsername verifies the UNIQUE constraint on username:
// the second insert with the same username must fail.
func TestCreateUser_DuplicateUsername(t *testing.T) {
	pool := testdb.Setup(t)

	first, err := models.CreateUser(t.Context(), pool, "dupe_user", "First User")
	if err != nil {
		t.Fatalf("CreateUser() first insert failed: %v", err)
	}

	second, err := models.CreateUser(t.Context(), pool, "dupe_user", "Second User")
	if err == nil {
		t.Errorf("CreateUser() succeeded unexpectedly for duplicate username: %+v", second)
	}
	if second != nil {
		t.Errorf("CreateUser() returned a user on error: %+v", second)
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

// TestCreateUser_PersistsRow verifies that CreateUser actually wrote the row
// to the database, not just returned a struct.
func TestCreateUser_PersistsRow(t *testing.T) {
	pool := testdb.Setup(t)

	created, err := models.CreateUser(t.Context(), pool, "persist_me", "Persisted User")
	if err != nil {
		t.Fatalf("CreateUser() failed: %v", err)
	}

	if created.ID <= 0 {
		t.Fatalf("CreateUser() returned non-positive ID: %d", created.ID)
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

			// Seed a known user so the test is independent of other data.
			if _, err := models.CreateUser(t.Context(), pool, "ahmed", "Ahmed Al-Bahrawy"); err != nil {
				t.Fatalf("seed user: %v", err)
			}

			got, gotErr := models.GetUserById(t.Context(), pool, tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetUserById() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetUserById() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetUserById() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID || got.Username != tt.want.Username || got.FullName != tt.want.FullName {
				t.Errorf("GetUserById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
