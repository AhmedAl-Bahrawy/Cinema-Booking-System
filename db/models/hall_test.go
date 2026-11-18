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
			if got.ID != tt.want.ID || got.Name != tt.want.Name || got.Capacity != tt.want.Capacity {
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
			if got.ID != tt.want.ID || got.Name != tt.want.Name || got.Capacity != tt.want.Capacity {
				t.Errorf("GetHallById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetAllHalls(t *testing.T) {
	t.Run("returns all halls ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)

		first, err := models.CreateHall(t.Context(), pool, "Hall A", 100)
		if err != nil {
			t.Fatalf("seed first hall: %v", err)
		}
		second, err := models.CreateHall(t.Context(), pool, "Hall B", 50)
		if err != nil {
			t.Fatalf("seed second hall: %v", err)
		}

		got, err := models.GetAllHalls(t.Context(), pool)
		if err != nil {
			t.Fatalf("GetAllHalls() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAllHalls() returned %d halls, want 2", len(got))
		}
		if got[0].ID != first.ID || got[0].Name != "Hall A" || got[0].Capacity != 100 {
			t.Errorf("GetAllHalls()[0] = %+v, want %+v", got[0], first)
		}
		if got[1].ID != second.ID || got[1].Name != "Hall B" || got[1].Capacity != 50 {
			t.Errorf("GetAllHalls()[1] = %+v, want %+v", got[1], second)
		}
	})

	t.Run("returns an empty list when there are no halls", func(t *testing.T) {
		pool := testdb.Setup(t)

		got, err := models.GetAllHalls(t.Context(), pool)
		if err != nil {
			t.Fatalf("GetAllHalls() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAllHalls() returned %d halls, want 0", len(got))
		}
	})
}

func TestGetAllHallsWithDetails(t *testing.T) {
	pool := testdb.Setup(t)

	hallA, err := models.CreateHall(t.Context(), pool, "Hall A", 100)
	if err != nil {
		t.Fatalf("seed first hall: %v", err)
	}
	hallB, err := models.CreateHall(t.Context(), pool, "Hall B", 50)
	if err != nil {
		t.Fatalf("seed second hall: %v", err)
	}

	// Hall A has two seats; Hall B has none.
	seat1, err := models.CreateSeat(t.Context(), pool, 0, hallA.ID, 1, models.Free, 100)
	if err != nil {
		t.Fatalf("seed first seat: %v", err)
	}
	seat2, err := models.CreateSeat(t.Context(), pool, 0, hallA.ID, 2, models.Holding, 150)
	if err != nil {
		t.Fatalf("seed second seat: %v", err)
	}

	got, err := models.GetAllHallsWithDetails(t.Context(), pool)
	if err != nil {
		t.Fatalf("GetAllHallsWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllHallsWithDetails() returned %d halls, want 2", len(got))
	}

	// Hall A: two seats attached.
	if got[0].ID != hallA.ID {
		t.Fatalf("GetAllHallsWithDetails()[0].ID = %d, want %d", got[0].ID, hallA.ID)
	}
	if len(got[0].Seats) != 2 {
		t.Fatalf("GetAllHallsWithDetails()[0].Seats has %d seats, want 2", len(got[0].Seats))
	}
	if got[0].Seats[0].ID != seat1.ID || got[0].Seats[0].SeatNumber != 1 ||
		got[0].Seats[0].HallId != hallA.ID || got[0].Seats[0].Status != models.Free ||
		got[0].Seats[0].Price != 100 {
		t.Errorf("GetAllHallsWithDetails()[0].Seats[0] = %+v", got[0].Seats[0])
	}
	if got[0].Seats[1].ID != seat2.ID || got[0].Seats[1].Status != models.Holding || got[0].Seats[1].Price != 150 {
		t.Errorf("GetAllHallsWithDetails()[0].Seats[1] = %+v", got[0].Seats[1])
	}

	// Hall B: no seats.
	if got[1].ID != hallB.ID {
		t.Fatalf("GetAllHallsWithDetails()[1].ID = %d, want %d", got[1].ID, hallB.ID)
	}
	if len(got[1].Seats) != 0 {
		t.Errorf("GetAllHallsWithDetails()[1].Seats has %d seats, want 0", len(got[1].Seats))
	}
}
