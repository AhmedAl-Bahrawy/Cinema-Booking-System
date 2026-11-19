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
	hall, err := models.NewHallRepository(pool).Create(t.Context(), "Hall A", 100)
	if err != nil {
		t.Fatalf("seed hall: %v", err)
	}
	return hall
}

// seedUser creates one user to attach to held/booked seats.
func seedUser(t *testing.T, pool *pgxpool.Pool) *models.User {
	t.Helper()
	user, err := models.NewUserRepository(pool).Create(t.Context(), "ahmed", "Ahmed Al-Bahrawy")
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
			repo := models.NewSeatRepository(pool)
			hall := seedHall(t, pool)
			if tt.userId != 0 {
				seedUser(t, pool)
			}

			got, gotErr := repo.Create(t.Context(), tt.userId, hall.ID, tt.seatNumber, tt.status, tt.price)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Create() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}
			tt.want.HallId = hall.ID
			if *got != *tt.want {
				t.Errorf("Create() = %+v, want %+v", got, tt.want)
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

	first, err := seatRepo.Create(t.Context(), 0, hall.ID, 1, models.Free, 100)
	if err != nil {
		t.Fatalf("Create() first insert failed: %v", err)
	}

	second, err := seatRepo.Create(t.Context(), 0, hall.ID, 1, models.Free, 100)
	if err == nil {
		t.Errorf("Create() succeeded unexpectedly for duplicate seat number: %+v", second)
	}

	// The same seat number in another hall must still work.
	otherHall, err := hallRepo.Create(t.Context(), "Hall B", 50)
	if err != nil {
		t.Fatalf("seed second hall: %v", err)
	}
	other, err := seatRepo.Create(t.Context(), 0, otherHall.ID, 1, models.Free, 100)
	if err != nil {
		t.Errorf("Create() failed for same number in another hall: %v", err)
	}
	if other != nil && other.ID == first.ID {
		t.Errorf("Create() returned the first seat's id: %d", other.ID)
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
			repo := models.NewSeatRepository(pool)
			hall := seedHall(t, pool)

			// Seed the seat (and a user if the seat is booked).
			userId := int64(0)
			if tt.withUser {
				userId = seedUser(t, pool).ID
			}
			if _, err := repo.Create(t.Context(), userId, hall.ID, 1, models.Free, 100); err != nil {
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
			if got.ID != 1 || got.SeatNumber != 1 || got.Status != models.Free || got.Price != 100 {
				t.Errorf("GetWithDetails() seat fields = %+v", got)
			}

			// The joined Hall.
			if got.Hall == nil {
				t.Fatal("GetWithDetails() returned a nil Hall")
			}
			if got.Hall.ID != hall.ID || got.Hall.Name != hall.Name || got.Hall.Capacity != hall.Capacity {
				t.Errorf("GetWithDetails() Hall = %+v, want %+v", got.Hall, hall)
			}

			// The joined User: nil for a free seat, filled for a booked seat.
			if !tt.withUser {
				if got.User != nil {
					t.Errorf("GetWithDetails() User = %+v, want nil for a free seat", got.User)
				}
				if got.UserId != 0 {
					t.Errorf("GetWithDetails() UserId = %d, want 0 for a free seat", got.UserId)
				}
			} else {
				if got.User == nil {
					t.Fatal("GetWithDetails() returned a nil User for a booked seat")
				}
				if got.UserId != userId || got.User.ID != userId ||
					got.User.Username != "ahmed" || got.User.FullName != "Ahmed Al-Bahrawy" {
					t.Errorf("GetWithDetails() User = %+v, want user id %d", got.User, userId)
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
			repo := models.NewSeatRepository(pool)
			hall := seedHall(t, pool)

			// Seed a known seat so the test is independent of other data.
			if _, err := repo.Create(t.Context(), 0, hall.ID, 1, models.Free, 100); err != nil {
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
			tt.want.HallId = hall.ID
			if *got != *tt.want {
				t.Errorf("GetById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetAllSeats(t *testing.T) {
	t.Run("returns all seats ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewSeatRepository(pool)
		hall := seedHall(t, pool)
		user := seedUser(t, pool)

		freeSeat, err := repo.Create(t.Context(), 0, hall.ID, 1, models.Free, 100)
		if err != nil {
			t.Fatalf("seed free seat: %v", err)
		}
		bookedSeat, err := repo.Create(t.Context(), user.ID, hall.ID, 2, models.Booked, 150)
		if err != nil {
			t.Fatalf("seed booked seat: %v", err)
		}

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAll() returned %d seats, want 2", len(got))
		}
		if got[0].ID != freeSeat.ID || got[0].SeatNumber != 1 ||
			got[0].Status != models.Free || got[0].Price != 100 || got[0].UserId != 0 {
			t.Errorf("GetAll()[0] = %+v", got[0])
		}
		if got[1].ID != bookedSeat.ID || got[1].SeatNumber != 2 ||
			got[1].Status != models.Booked || got[1].Price != 150 || got[1].UserId != user.ID {
			t.Errorf("GetAll()[1] = %+v", got[1])
		}
		// A plain GetAll does not join, so the relations stay empty.
		if got[0].User != nil || got[0].Hall != nil {
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
	user := seedUser(t, pool)

	freeSeat, err := repo.Create(t.Context(), 0, hall.ID, 1, models.Free, 100)
	if err != nil {
		t.Fatalf("seed free seat: %v", err)
	}
	bookedSeat, err := repo.Create(t.Context(), user.ID, hall.ID, 2, models.Booked, 150)
	if err != nil {
		t.Fatalf("seed booked seat: %v", err)
	}

	got, err := repo.GetAllWithDetails(t.Context())
	if err != nil {
		t.Fatalf("GetAllWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllWithDetails() returned %d seats, want 2", len(got))
	}

	// Free seat: Hall filled, User nil.
	if got[0].ID != freeSeat.ID {
		t.Fatalf("GetAllWithDetails()[0].ID = %d, want %d", got[0].ID, freeSeat.ID)
	}
	if got[0].Hall == nil || got[0].Hall.ID != hall.ID || got[0].Hall.Name != "Hall A" {
		t.Errorf("GetAllWithDetails()[0].Hall = %+v, want hall %d", got[0].Hall, hall.ID)
	}
	if got[0].User != nil {
		t.Errorf("GetAllWithDetails()[0].User = %+v, want nil for a free seat", got[0].User)
	}

	// Booked seat: Hall and User filled.
	if got[1].ID != bookedSeat.ID {
		t.Fatalf("GetAllWithDetails()[1].ID = %d, want %d", got[1].ID, bookedSeat.ID)
	}
	if got[1].Hall == nil || got[1].Hall.ID != hall.ID {
		t.Errorf("GetAllWithDetails()[1].Hall = %+v, want hall %d", got[1].Hall, hall.ID)
	}
	if got[1].User == nil {
		t.Fatal("GetAllWithDetails()[1].User is nil, want the booked user")
	}
	if got[1].User.ID != user.ID || got[1].User.Username != "ahmed" || got[1].User.FullName != "Ahmed Al-Bahrawy" {
		t.Errorf("GetAllWithDetails()[1].User = %+v, want user %d", got[1].User, user.ID)
	}
	if got[1].UserId != user.ID || got[1].Status != models.Booked || got[1].Price != 150 {
		t.Errorf("GetAllWithDetails()[1] = %+v", got[1])
	}
}
