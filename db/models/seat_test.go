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

func TestGetSeatWithDetails(t *testing.T) {
	tests := []struct {
		name     string // description of this test case
		withUser bool   // true = book the seat for a user, false = free seat
		id       int64
		wantErr  bool
	}{
		{
			name:     "returns a free seat with its hall but no user",
			withUser: false,
			id:       1,
			wantErr:  false,
		},
		{
			name:     "returns a booked seat with its hall and user",
			withUser: true,
			id:       1,
			wantErr:  false,
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
			hall := seedHall(t, pool)

			// Seed the seat (and a user if the seat is booked).
			userId := int64(0)
			if tt.withUser {
				userId = seedUser(t, pool).ID
			}
			if _, err := models.CreateSeat(t.Context(), pool, userId, hall.ID, 1, models.Free, 100); err != nil {
				t.Fatalf("seed seat: %v", err)
			}

			got, gotErr := models.GetSeatWithDetails(t.Context(), pool, tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetSeatWithDetails() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetSeatWithDetails() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetSeatWithDetails() succeeded unexpectedly")
			}

			// The seat's own fields.
			if got.ID != 1 || got.SeatNumber != 1 || got.Status != models.Free || got.Price != 100 {
				t.Errorf("GetSeatWithDetails() seat fields = %+v", got)
			}

			// The joined Hall.
			if got.Hall == nil {
				t.Fatal("GetSeatWithDetails() returned a nil Hall")
			}
			if got.Hall.ID != hall.ID || got.Hall.Name != hall.Name || got.Hall.Capacity != hall.Capacity {
				t.Errorf("GetSeatWithDetails() Hall = %+v, want %+v", got.Hall, hall)
			}

			// The joined User: nil for a free seat, filled for a booked seat.
			if !tt.withUser {
				if got.User != nil {
					t.Errorf("GetSeatWithDetails() User = %+v, want nil for a free seat", got.User)
				}
				if got.UserId != 0 {
					t.Errorf("GetSeatWithDetails() UserId = %d, want 0 for a free seat", got.UserId)
				}
			} else {
				if got.User == nil {
					t.Fatal("GetSeatWithDetails() returned a nil User for a booked seat")
				}
				if got.UserId != userId || got.User.ID != userId ||
					got.User.Username != "ahmed" || got.User.FullName != "Ahmed Al-Bahrawy" {
					t.Errorf("GetSeatWithDetails() User = %+v, want user id %d", got.User, userId)
				}
			}
		})
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

func TestGetAllSeats(t *testing.T) {
	t.Run("returns all seats ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)
		hall := seedHall(t, pool)
		user := seedUser(t, pool)

		freeSeat, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 1, models.Free, 100)
		if err != nil {
			t.Fatalf("seed free seat: %v", err)
		}
		bookedSeat, err := models.CreateSeat(t.Context(), pool, user.ID, hall.ID, 2, models.Booked, 150)
		if err != nil {
			t.Fatalf("seed booked seat: %v", err)
		}

		got, err := models.GetAllSeats(t.Context(), pool)
		if err != nil {
			t.Fatalf("GetAllSeats() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAllSeats() returned %d seats, want 2", len(got))
		}
		if got[0].ID != freeSeat.ID || got[0].SeatNumber != 1 ||
			got[0].Status != models.Free || got[0].Price != 100 || got[0].UserId != 0 {
			t.Errorf("GetAllSeats()[0] = %+v", got[0])
		}
		if got[1].ID != bookedSeat.ID || got[1].SeatNumber != 2 ||
			got[1].Status != models.Booked || got[1].Price != 150 || got[1].UserId != user.ID {
			t.Errorf("GetAllSeats()[1] = %+v", got[1])
		}
		// A plain GetAll does not join, so the relations stay empty.
		if got[0].User != nil || got[0].Hall != nil {
			t.Errorf("GetAllSeats() should not fill relations, got %+v", got[0])
		}
	})

	t.Run("returns an empty list when there are no seats", func(t *testing.T) {
		pool := testdb.Setup(t)

		got, err := models.GetAllSeats(t.Context(), pool)
		if err != nil {
			t.Fatalf("GetAllSeats() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAllSeats() returned %d seats, want 0", len(got))
		}
	})
}

func TestGetAllSeatsWithDetails(t *testing.T) {
	pool := testdb.Setup(t)
	hall := seedHall(t, pool)
	user := seedUser(t, pool)

	freeSeat, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 1, models.Free, 100)
	if err != nil {
		t.Fatalf("seed free seat: %v", err)
	}
	bookedSeat, err := models.CreateSeat(t.Context(), pool, user.ID, hall.ID, 2, models.Booked, 150)
	if err != nil {
		t.Fatalf("seed booked seat: %v", err)
	}

	got, err := models.GetAllSeatsWithDetails(t.Context(), pool)
	if err != nil {
		t.Fatalf("GetAllSeatsWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllSeatsWithDetails() returned %d seats, want 2", len(got))
	}

	// Free seat: Hall filled, User nil.
	if got[0].ID != freeSeat.ID {
		t.Fatalf("GetAllSeatsWithDetails()[0].ID = %d, want %d", got[0].ID, freeSeat.ID)
	}
	if got[0].Hall == nil || got[0].Hall.ID != hall.ID || got[0].Hall.Name != "Hall A" {
		t.Errorf("GetAllSeatsWithDetails()[0].Hall = %+v, want hall %d", got[0].Hall, hall.ID)
	}
	if got[0].User != nil {
		t.Errorf("GetAllSeatsWithDetails()[0].User = %+v, want nil for a free seat", got[0].User)
	}

	// Booked seat: Hall and User filled.
	if got[1].ID != bookedSeat.ID {
		t.Fatalf("GetAllSeatsWithDetails()[1].ID = %d, want %d", got[1].ID, bookedSeat.ID)
	}
	if got[1].Hall == nil || got[1].Hall.ID != hall.ID {
		t.Errorf("GetAllSeatsWithDetails()[1].Hall = %+v, want hall %d", got[1].Hall, hall.ID)
	}
	if got[1].User == nil {
		t.Fatal("GetAllSeatsWithDetails()[1].User is nil, want the booked user")
	}
	if got[1].User.ID != user.ID || got[1].User.Username != "ahmed" || got[1].User.FullName != "Ahmed Al-Bahrawy" {
		t.Errorf("GetAllSeatsWithDetails()[1].User = %+v, want user %d", got[1].User, user.ID)
	}
	if got[1].UserId != user.ID || got[1].Status != models.Booked || got[1].Price != 150 {
		t.Errorf("GetAllSeatsWithDetails()[1] = %+v", got[1])
	}
}
