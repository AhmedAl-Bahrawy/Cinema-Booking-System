package models_test

import (
	"errors"
	"testing"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// seedShow creates one show to use for tickets.
func seedShow(t *testing.T, pool *pgxpool.Pool) *models.Show {
	t.Helper()
	show, err := models.CreateShow(t.Context(), pool, "Hamlet", 120, "Shakespeare")
	if err != nil {
		t.Fatalf("seed show: %v", err)
	}
	return show
}

// seedTicketParents creates a hall, a seat in that hall, and a show —
// everything a ticket needs to reference.
func seedTicketParents(t *testing.T, pool *pgxpool.Pool) (*models.Hall, *models.Seat, *models.Show) {
	t.Helper()
	hall := seedHall(t, pool)
	seat, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 1, models.Free, 100)
	if err != nil {
		t.Fatalf("seed seat: %v", err)
	}
	show := seedShow(t, pool)
	return hall, seat, show
}

func TestCreateTicket(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		seatId  int64
		hallId  int64
		showId  int64
		want    *models.Ticket
		wantErr bool
	}{
		{
			name:    "creates a ticket with valid data",
			seatId:  1,
			hallId:  1,
			showId:  1,
			want:    &models.Ticket{ID: 1, SeatId: 1, HallId: 1, ShowId: 1},
			wantErr: false,
		},
		{
			// show_id has a foreign key to shows, so 999 must be rejected.
			name:    "non-existent show id is rejected",
			seatId:  1,
			hallId:  1,
			showId:  999,
			wantErr: true,
		},
		{
			// seat_id has a foreign key to seats, so 999 must be rejected.
			name:    "non-existent seat id is rejected",
			seatId:  999,
			hallId:  1,
			showId:  1,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			_, _, _ = seedTicketParents(t, pool)

			got, gotErr := models.CreateTicket(t.Context(), pool, tt.seatId, tt.hallId, tt.showId)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateTicket() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CreateTicket() succeeded unexpectedly")
			}
			if *got != *tt.want {
				t.Errorf("CreateTicket() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCreateTicket_DuplicateSeatForShow verifies the UNIQUE (show_id, seat_id)
// index: a seat can only be sold once per show, but the same seat can be sold
// for a different show.
func TestCreateTicket_DuplicateSeatForShow(t *testing.T) {
	pool := testdb.Setup(t)
	_, seat, show := seedTicketParents(t, pool)

	first, err := models.CreateTicket(t.Context(), pool, seat.ID, seat.HallId, show.ID)
	if err != nil {
		t.Fatalf("CreateTicket() first insert failed: %v", err)
	}

	second, err := models.CreateTicket(t.Context(), pool, seat.ID, seat.HallId, show.ID)
	if err == nil {
		t.Errorf("CreateTicket() succeeded unexpectedly for duplicate seat per show: %+v", second)
	}

	// The same seat for another show must still work.
	otherShow, err := models.CreateShow(t.Context(), pool, "Othello", 90, "Shakespeare")
	if err != nil {
		t.Fatalf("seed second show: %v", err)
	}
	other, err := models.CreateTicket(t.Context(), pool, seat.ID, seat.HallId, otherShow.ID)
	if err != nil {
		t.Errorf("CreateTicket() failed for same seat on another show: %v", err)
	}
	if other != nil && other.ID == first.ID {
		t.Errorf("CreateTicket() returned the first ticket's id: %d", other.ID)
	}
}

func TestGetTicketById(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		want    *models.Ticket
		wantErr bool
	}{
		{
			name:    "returns the ticket for an existing id",
			id:      1,
			want:    &models.Ticket{ID: 1, SeatId: 1, HallId: 1, ShowId: 1},
			wantErr: false,
		},
		{
			// No ticket was seeded for this id, so no rows are returned.
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

			// Seed known parents and a ticket so the test is independent of other data.
			hall, seat, show := seedTicketParents(t, pool)
			if _, err := models.CreateTicket(t.Context(), pool, seat.ID, hall.ID, show.ID); err != nil {
				t.Fatalf("seed ticket: %v", err)
			}

			got, gotErr := models.GetTicketById(t.Context(), pool, tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetTicketById() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetTicketById() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetTicketById() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID || got.SeatId != seat.ID || got.HallId != hall.ID || got.ShowId != show.ID {
				t.Errorf("GetTicketById() = %+v, want (%d, %d, %d, %d)",
					got, tt.want.ID, seat.ID, hall.ID, show.ID)
			}
		})
	}
}
