package models_test

import (
	"errors"
	"testing"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedHall creates one hall to use as the seat's parent.
func seedHall(t *testing.T, pool *pgxpool.Pool) *models.Hall {
	t.Helper()
	hall, err := models.CreateHall(t.Context(), pool, "Hall A", 100)
	if err != nil {
		t.Fatalf("seed hall: %v", err)
	}
	return hall
}

// seedUser creates one user to attach to held/booked seats.
func seedUser(t *testing.T, pool *pgxpool.Pool) *models.User {
	t.Helper()
	user, err := models.CreateUser(t.Context(), pool, "ahmed", "Ahmed Al-Bahrawy")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func TestCreateSeat(t *testing.T) {
	tests := []struct {
		name       string // description of this test case
		userId     int64  // 0 means a free seat (user_id is NULL)
		seatNumber uint32
		status     models.BookingStatus
		price      uint32
		want       *models.Seat
		wantErr    bool
	}{
		{
			name:       "creates a free seat with no user",
			userId:     0,
			seatNumber: 1,
			status:     models.Free,
			price:      100,
			want:       &models.Seat{ID: 1, UserId: 0, HallId: 1, SeatNumber: 1, Status: models.Free, Price: 100},
			wantErr:    false,
		},
		{
			name:       "creates a booked seat linked to a user",
			userId:     1,
			seatNumber: 2,
			status:     models.Booked,
			price:      150,
			want:       &models.Seat{ID: 1, UserId: 1, HallId: 1, SeatNumber: 2, Status: models.Booked, Price: 150},
			wantErr:    false,
		},
		{
			// price has CHECK (price >= 0), so 0 is a valid value.
			name:       "zero price is accepted",
			userId:     0,
			seatNumber: 3,
			status:     models.Free,
			price:      0,
			want:       &models.Seat{ID: 1, UserId: 0, HallId: 1, SeatNumber: 3, Status: models.Free, Price: 0},
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			hall := seedHall(t, pool)
			if tt.userId != 0 {
				seedUser(t, pool)
			}

			got, gotErr := models.CreateSeat(t.Context(), pool, tt.userId, hall.ID, tt.seatNumber, tt.status, tt.price)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateSeat() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CreateSeat() succeeded unexpectedly")
			}
			tt.want.HallId = hall.ID
			if *got != *tt.want {
				t.Errorf("CreateSeat() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCreateSeat_DuplicateSeatNumberInHall verifies the UNIQUE (hall_id, seat_number)
// index: the same seat number twice in one hall must fail, but the same number in
// a different hall is fine.
func TestCreateSeat_DuplicateSeatNumberInHall(t *testing.T) {
	pool := testdb.Setup(t)
	hall := seedHall(t, pool)

	first, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 1, models.Free, 100)
	if err != nil {
		t.Fatalf("CreateSeat() first insert failed: %v", err)
	}

	second, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 1, models.Free, 100)
	if err == nil {
		t.Errorf("CreateSeat() succeeded unexpectedly for duplicate seat number: %+v", second)
	}

	// The same seat number in another hall must still work.
	otherHall, err := models.CreateHall(t.Context(), pool, "Hall B", 50)
	if err != nil {
		t.Fatalf("seed second hall: %v", err)
	}
	other, err := models.CreateSeat(t.Context(), pool, 0, otherHall.ID, 1, models.Free, 100)
	if err != nil {
		t.Errorf("CreateSeat() failed for same number in another hall: %v", err)
	}
	if other != nil && other.ID == first.ID {
		t.Errorf("CreateSeat() returned the first seat's id: %d", other.ID)
	}
}

func TestGetSeatById(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		want    *models.Seat
		wantErr bool
	}{
		{
			name:    "returns the seat for an existing id",
			id:      1,
			want:    &models.Seat{ID: 1, UserId: 0, HallId: 1, SeatNumber: 1, Status: models.Free, Price: 100},
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
			hall := seedHall(t, pool)

			// Seed a known seat so the test is independent of other data.
			if _, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 1, models.Free, 100); err != nil {
				t.Fatalf("seed seat: %v", err)
			}

			got, gotErr := models.GetSeatById(t.Context(), pool, tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetSeatById() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetSeatById() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetSeatById() succeeded unexpectedly")
			}
			tt.want.HallId = hall.ID
			if *got != *tt.want {
				t.Errorf("GetSeatById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
