package models_test

import (
	"errors"
	"testing"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
)

func TestCreateSeat(t *testing.T) {
	tests := []struct {
		name       string // description of this test case
		seatNumber uint32
		categoryId int64 // 0 means "use the normal category"
		wantCat    int64
		wantErr    bool
	}{
		{
			name:       "creates a normal seat by default",
			seatNumber: 1,
			categoryId: 0,
			wantCat:    1,
		},
		{
			name:       "creates a premium seat",
			seatNumber: 2,
			categoryId: 2,
			wantCat:    2,
		},
		{
			name:       "creates a VIP seat",
			seatNumber: 3,
			categoryId: 3,
			wantCat:    3,
		},
		{
			// seat_categories has a foreign key, so an unknown category fails.
			name:       "unknown category is rejected",
			seatNumber: 4,
			categoryId: 999,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewSeatRepository(pool)
			hall := seedHall(t, pool)

			got, gotErr := repo.Create(t.Context(), hall.ID, tt.seatNumber, tt.categoryId)
			if gotErr != nil {
				if !tt.wantErr {
					t.Fatalf("Create() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}
			if got.ID != 1 || got.HallId != hall.ID || got.SeatNumber != tt.seatNumber ||
				got.CategoryId != tt.wantCat {
				t.Errorf("Create() = %+v", got)
			}
		})
	}
}

// TestCreateSeat_DuplicateSeatNumberInHall verifies the UNIQUE (hall_id, seat_number)
// index: the same seat number twice in one hall must fail, but the same number in
// a different hall is fine.
func TestCreateSeat_DuplicateSeatNumberInHall(t *testing.T) {
	pool := testdb.Setup(t)
	seatRepo := models.NewSeatRepository(pool)
	hallRepo := models.NewHallRepository(pool)
	hall := seedHall(t, pool)

	first, err := seatRepo.Create(t.Context(), hall.ID, 1, 1)
	if err != nil {
		t.Fatalf("Create() first insert failed: %v", err)
	}

	second, err := seatRepo.Create(t.Context(), hall.ID, 1, 1)
	if err == nil {
		t.Errorf("Create() succeeded unexpectedly for duplicate seat number: %+v", second)
	}

	// The same seat number in another hall must still work.
	otherHall, err := hallRepo.Create(t.Context(), "Hall B", 50)
	if err != nil {
		t.Fatalf("seed second hall: %v", err)
	}
	other, err := seatRepo.Create(t.Context(), otherHall.ID, 1, 1)
	if err != nil {
		t.Errorf("Create() failed for same number in another hall: %v", err)
	}
	if other != nil && other.ID == first.ID {
		t.Errorf("Create() returned the first seat's id: %d", other.ID)
	}
}

func TestGetSeatWithDetails(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		wantErr bool
	}{
		{
			name:    "returns a seat with its hall and category",
			id:      1,
			wantErr: false,
		},
		{
			// No seat was seeded for this id, so no rows are returned.
			name:    "returns error for a non-existent id",
			id:      999,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewSeatRepository(pool)
			hall := seedHall(t, pool)

			if _, err := repo.Create(t.Context(), hall.ID, 1, 2); err != nil {
				t.Fatalf("seed seat: %v", err)
			}

			got, gotErr := repo.GetWithDetails(t.Context(), tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetWithDetails() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetWithDetails() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetWithDetails() succeeded unexpectedly")
			}

			// The seat's own fields.
			if got.ID != 1 || got.SeatNumber != 1 || got.CategoryId != 2 {
				t.Errorf("GetWithDetails() seat fields = %+v", got)
			}

			// The joined category.
			if got.Category == nil {
				t.Fatal("GetWithDetails() returned a nil Category")
			}
			if got.Category.ID != 2 || got.Category.Name != "premium" ||
				got.Category.PriceMultiplierPercent != 150 {
				t.Errorf("GetWithDetails() Category = %+v, want premium (150)", got.Category)
			}

			// The joined hall.
			if got.Hall == nil {
				t.Fatal("GetWithDetails() returned a nil Hall")
			}
			if got.Hall.ID != hall.ID || got.Hall.Name != hall.Name || got.Hall.Capacity != hall.Capacity {
				t.Errorf("GetWithDetails() Hall = %+v, want %+v", got.Hall, hall)
			}
		})
	}
}

func TestGetSeatById(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		wantErr bool
	}{
		{
			name:    "returns the seat for an existing id",
			id:      1,
			wantErr: false,
		},
		{
			// No seat was seeded for this id, so no rows are returned.
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
			repo := models.NewSeatRepository(pool)
			hall := seedHall(t, pool)

			if _, err := repo.Create(t.Context(), hall.ID, 1, 1); err != nil {
				t.Fatalf("seed seat: %v", err)
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
			if got.ID != 1 || got.HallId != hall.ID || got.SeatNumber != 1 || got.CategoryId != 1 {
				t.Errorf("GetById() = %+v", got)
			}
		})
	}
}

func TestGetAllSeats(t *testing.T) {
	t.Run("returns all seats ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewSeatRepository(pool)
		hall := seedHall(t, pool)

		first, err := repo.Create(t.Context(), hall.ID, 1, 1)
		if err != nil {
			t.Fatalf("seed first seat: %v", err)
		}
		second, err := repo.Create(t.Context(), hall.ID, 2, 2)
		if err != nil {
			t.Fatalf("seed second seat: %v", err)
		}

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAll() returned %d seats, want 2", len(got))
		}
		if got[0].ID != first.ID || got[0].SeatNumber != 1 || got[0].CategoryId != 1 {
			t.Errorf("GetAll()[0] = %+v", got[0])
		}
		if got[1].ID != second.ID || got[1].SeatNumber != 2 || got[1].CategoryId != 2 {
			t.Errorf("GetAll()[1] = %+v", got[1])
		}
		// A plain GetAll does not join, so the relations stay empty.
		if got[0].Hall != nil || got[0].Category != nil {
			t.Errorf("GetAll() should not fill relations, got %+v", got[0])
		}
	})

	t.Run("returns an empty list when there are no seats", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewSeatRepository(pool)

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAll() returned %d seats, want 0", len(got))
		}
	})
}

func TestGetAllSeatsWithDetails(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewSeatRepository(pool)
	hall := seedHall(t, pool)

	first, err := repo.Create(t.Context(), hall.ID, 1, 1)
	if err != nil {
		t.Fatalf("seed first seat: %v", err)
	}
	second, err := repo.Create(t.Context(), hall.ID, 2, 3)
	if err != nil {
		t.Fatalf("seed second seat: %v", err)
	}

	got, err := repo.GetAllWithDetails(t.Context())
	if err != nil {
		t.Fatalf("GetAllWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllWithDetails() returned %d seats, want 2", len(got))
	}

	for i, want := range []*models.Seat{first, second} {
		if got[i].ID != want.ID {
			t.Fatalf("GetAllWithDetails()[%d].ID = %d, want %d", i, got[i].ID, want.ID)
		}
		if got[i].Hall == nil || got[i].Hall.ID != hall.ID || got[i].Hall.Name != "Hall A" {
			t.Errorf("GetAllWithDetails()[%d].Hall = %+v, want hall %d", i, got[i].Hall, hall.ID)
		}
		if got[i].Category == nil || got[i].Category.ID != want.CategoryId {
			t.Errorf("GetAllWithDetails()[%d].Category = %+v, want category %d", i, got[i].Category, want.CategoryId)
		}
	}
	// The second seat is VIP.
	if got[1].Category.Name != "vip" || got[1].Category.PriceMultiplierPercent != 200 {
		t.Errorf("GetAllWithDetails()[1].Category = %+v, want vip (200)", got[1].Category)
	}
}

func TestGetSeatsByHall(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewSeatRepository(pool)
	hallA := seedHall(t, pool)
	hallB, err := models.NewHallRepository(pool).Create(t.Context(), "Hall B", 50)
	if err != nil {
		t.Fatalf("seed second hall: %v", err)
	}

	if _, err := repo.Create(t.Context(), hallA.ID, 1, 1); err != nil {
		t.Fatalf("seed hall A seat: %v", err)
	}
	if _, err := repo.Create(t.Context(), hallA.ID, 2, 1); err != nil {
		t.Fatalf("seed hall A second seat: %v", err)
	}
	if _, err := repo.Create(t.Context(), hallB.ID, 1, 1); err != nil {
		t.Fatalf("seed hall B seat: %v", err)
	}

	got, err := repo.GetByHall(t.Context(), hallA.ID)
	if err != nil {
		t.Fatalf("GetByHall() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetByHall() returned %d seats, want 2", len(got))
	}
	for _, seat := range got {
		if seat.HallId != hallA.ID {
			t.Errorf("GetByHall() returned a seat from hall %d", seat.HallId)
		}
	}
}
