package models_test

import (
	"testing"
	"time"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/jackc/pgx/v5/pgxpool"
)

// screeningBase is a fixed time so screening tests are deterministic.
var screeningBase = time.Date(2026, 11, 21, 10, 0, 0, 0, time.UTC)

func seedHall(t *testing.T, pool *pgxpool.Pool) *models.Hall {
	t.Helper()
	hall, err := models.NewHallRepository(pool).Create(t.Context(), "Hall A", 100)
	if err != nil {
		t.Fatalf("seed hall: %v", err)
	}
	return hall
}

func seedUser(t *testing.T, pool *pgxpool.Pool) *models.User {
	t.Helper()
	user, err := models.NewUserRepository(pool).Create(t.Context(), "ahmed", "Ahmed Al-Bahrawy")
	if err != nil {
		t.Fatalf("seed user: %v", err)
	}
	return user
}

func seedShow(t *testing.T, pool *pgxpool.Pool) *models.Show {
	t.Helper()
	show, err := models.NewShowRepository(pool).Create(t.Context(), "Hamlet", 120, "Shakespeare")
	if err != nil {
		t.Fatalf("seed show: %v", err)
	}
	return show
}

func seedSeat(t *testing.T, pool *pgxpool.Pool, hallId int64, seatNumber uint32, price uint32) *models.Seat {
	t.Helper()
	seat, err := models.NewSeatRepository(pool).Create(t.Context(), hallId, seatNumber, price)
	if err != nil {
		t.Fatalf("seed seat: %v", err)
	}
	return seat
}

// seedScreening schedules a show in a hall starting at the given time. EndsAt
// is calculated from the show's duration by the repository.
func seedScreening(t *testing.T, pool *pgxpool.Pool, hallId, showId int64, startsAt time.Time) *models.Screening {
	t.Helper()
	screening, err := models.NewScreeningRepository(pool).CreateAt(t.Context(), hallId, showId, startsAt)
	if err != nil {
		t.Fatalf("seed screening: %v", err)
	}
	return screening
}

// seedTicketParents creates a hall, a seat in that hall, a show, and a
// screening of that show in that hall — everything a ticket needs.
func seedTicketParents(t *testing.T, pool *pgxpool.Pool) (*models.Hall, *models.Seat, *models.Show, *models.Screening) {
	t.Helper()
	hall := seedHall(t, pool)
	seat := seedSeat(t, pool, hall.ID, 1, 100)
	show := seedShow(t, pool)
	screening := seedScreening(t, pool, hall.ID, show.ID, screeningBase)
	return hall, seat, show, screening
}
