package models_test

import (
	"errors"
	"testing"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
)

func TestCreateHall(t *testing.T) {
	tests := []struct {
		name     string // description of this test case
		hallName string
		capacity uint32
		want     *models.Hall
		wantErr  bool
	}{
		{
			name:     "creates a hall with valid data",
			hallName: "Hall A",
			capacity: 100,
			want:     &models.Hall{ID: 1, Name: "Hall A", Capacity: 100},
			wantErr:  false,
		},
		{
			// capacity has CHECK (capacity >= 0), so 0 is a valid value.
			name:     "zero capacity is accepted",
			hallName: "Zero Hall",
			capacity: 0,
			want:     &models.Hall{ID: 1, Name: "Zero Hall", Capacity: 0},
			wantErr:  false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)

			got, gotErr := models.CreateHall(t.Context(), pool, tt.hallName, tt.capacity)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateHall() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CreateHall() succeeded unexpectedly")
			}
			if *got != *tt.want {
				t.Errorf("CreateHall() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCreateHall_DuplicateName verifies the UNIQUE constraint on name:
// the second insert with the same name must fail.
func TestCreateHall_DuplicateName(t *testing.T) {
	pool := testdb.Setup(t)

	first, err := models.CreateHall(t.Context(), pool, "Main Hall", 50)
	if err != nil {
		t.Fatalf("CreateHall() first insert failed: %v", err)
	}

	second, err := models.CreateHall(t.Context(), pool, "Main Hall", 200)
	if err == nil {
		t.Errorf("CreateHall() succeeded unexpectedly for duplicate name: %+v", second)
	}
	if second != nil {
		t.Errorf("CreateHall() returned a hall on error: %+v", second)
	}

	// The first hall must be untouched by the failed insert.
	var capacity int64
	err = pool.QueryRow(t.Context(), `SELECT capacity FROM halls WHERE id = $1`, first.ID).Scan(&capacity)
	if err != nil {
		t.Fatalf("query first hall: %v", err)
	}
	if capacity != 50 {
		t.Errorf("capacity = %d, want %d", capacity, 50)
	}
}

func TestGetHallById(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		want    *models.Hall
		wantErr bool
	}{
		{
			name:    "returns the hall for an existing id",
			id:      1,
			want:    &models.Hall{ID: 1, Name: "Hall A", Capacity: 100},
			wantErr: false,
		},
		{
			// No hall was seeded for this id, so no rows are returned.
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

			// Seed a known hall so the test is independent of other data.
			if _, err := models.CreateHall(t.Context(), pool, "Hall A", 100); err != nil {
				t.Fatalf("seed hall: %v", err)
			}

			got, gotErr := models.GetHallById(t.Context(), pool, tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetHallById() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetHallById() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetHallById() succeeded unexpectedly")
			}
			if *got != *tt.want {
				t.Errorf("GetHallById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
