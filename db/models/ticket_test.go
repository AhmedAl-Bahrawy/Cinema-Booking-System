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
	show, err := models.NewShowRepository(pool).Create(t.Context(), "Hamlet", 120, "Shakespeare")
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
	seat, err := models.NewSeatRepository(pool).Create(t.Context(), 0, hall.ID, 1, models.Free, 100)
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
			repo := models.NewTicketRepository(pool)
			_, _, _ = seedTicketParents(t, pool)

			got, gotErr := repo.Create(t.Context(), tt.seatId, tt.hallId, tt.showId)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Create() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}
			if *got != *tt.want {
				t.Errorf("Create() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

// TestCreateTicket_DuplicateSeatForShow verifies the UNIQUE (show_id, seat_id)
// index: a seat can only be sold once per show, but the same seat can be sold
// for a different show.
func TestCreateTicket_DuplicateSeatForShow(t *testing.T) {
	pool := testdb.Setup(t)
	ticketRepo := models.NewTicketRepository(pool)
	showRepo := models.NewShowRepository(pool)
	_, seat, show := seedTicketParents(t, pool)

	first, err := ticketRepo.Create(t.Context(), seat.ID, seat.HallId, show.ID)
	if err != nil {
		t.Fatalf("Create() first insert failed: %v", err)
	}

	second, err := ticketRepo.Create(t.Context(), seat.ID, seat.HallId, show.ID)
	if err == nil {
		t.Errorf("Create() succeeded unexpectedly for duplicate seat per show: %+v", second)
	}

	// The same seat for another show must still work.
	otherShow, err := showRepo.Create(t.Context(), "Othello", 90, "Shakespeare")
	if err != nil {
		t.Fatalf("seed second show: %v", err)
	}
	other, err := ticketRepo.Create(t.Context(), seat.ID, seat.HallId, otherShow.ID)
	if err != nil {
		t.Errorf("Create() failed for same seat on another show: %v", err)
	}
	if other != nil && other.ID == first.ID {
		t.Errorf("Create() returned the first ticket's id: %d", other.ID)
	}
}

func TestGetTicketWithDetails(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		wantErr bool
	}{
		{
			name:    "returns the ticket with seat, hall and show filled",
			id:      1,
			wantErr: false,
		},
		{
			// No ticket was seeded for this id, so no rows are returned.
			name:    "returns error for a non-existent id",
			id:      999,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewTicketRepository(pool)

			// Seed known parents and a ticket so the test is independent of other data.
			hall, seat, show := seedTicketParents(t, pool)
			if _, err := repo.Create(t.Context(), seat.ID, hall.ID, show.ID); err != nil {
				t.Fatalf("seed ticket: %v", err)
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

			// The ticket's own fields.
			if got.ID != 1 || got.SeatId != seat.ID || got.HallId != hall.ID || got.ShowId != show.ID {
				t.Errorf("GetWithDetails() ticket fields = %+v", got)
			}

			// The joined Seat (a free seat: no user).
			if got.Seat == nil {
				t.Fatal("GetWithDetails() returned a nil Seat")
			}
			if got.Seat.ID != seat.ID || got.Seat.SeatNumber != 1 || got.Seat.Status != models.Free {
				t.Errorf("GetWithDetails() Seat = %+v, want %+v", got.Seat, seat)
			}
			if got.Seat.User != nil {
				t.Errorf("GetWithDetails() Seat.User = %+v, want nil for a free seat", got.Seat.User)
			}
			if got.Seat.Hall == nil || got.Seat.Hall.ID != hall.ID {
				t.Errorf("GetWithDetails() Seat.Hall = %+v, want hall id %d", got.Seat.Hall, hall.ID)
			}

			// The joined Hall.
			if got.Hall == nil {
				t.Fatal("GetWithDetails() returned a nil Hall")
			}
			if got.Hall.ID != hall.ID || got.Hall.Name != hall.Name || got.Hall.Capacity != hall.Capacity {
				t.Errorf("GetWithDetails() Hall = %+v, want %+v", got.Hall, hall)
			}

			// The joined Show.
			if got.Show == nil {
				t.Fatal("GetWithDetails() returned a nil Show")
			}
			if got.Show.ID != show.ID || got.Show.Name != show.Name ||
				got.Show.DurationMinutes != show.DurationMinutes || got.Show.AuthorName != show.AuthorName {
				t.Errorf("GetWithDetails() Show = %+v, want %+v", got.Show, show)
			}
		})
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
			repo := models.NewTicketRepository(pool)

			// Seed known parents and a ticket so the test is independent of other data.
			hall, seat, show := seedTicketParents(t, pool)
			if _, err := repo.Create(t.Context(), seat.ID, hall.ID, show.ID); err != nil {
				t.Fatalf("seed ticket: %v", err)
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
			if got.ID != tt.want.ID || got.SeatId != seat.ID || got.HallId != hall.ID || got.ShowId != show.ID {
				t.Errorf("GetById() = %+v, want (%d, %d, %d, %d)",
					got, tt.want.ID, seat.ID, hall.ID, show.ID)
			}
		})
	}
}

func TestGetAllTickets(t *testing.T) {
	t.Run("returns all tickets ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewTicketRepository(pool)
		seatRepo := models.NewSeatRepository(pool)
		hall := seedHall(t, pool)
		show := seedShow(t, pool)

		seat1, err := seatRepo.Create(t.Context(), 0, hall.ID, 1, models.Free, 100)
		if err != nil {
			t.Fatalf("seed first seat: %v", err)
		}
		seat2, err := seatRepo.Create(t.Context(), 0, hall.ID, 2, models.Free, 100)
		if err != nil {
			t.Fatalf("seed second seat: %v", err)
		}

		ticket1, err := repo.Create(t.Context(), seat1.ID, hall.ID, show.ID)
		if err != nil {
			t.Fatalf("seed first ticket: %v", err)
		}
		ticket2, err := repo.Create(t.Context(), seat2.ID, hall.ID, show.ID)
		if err != nil {
			t.Fatalf("seed second ticket: %v", err)
		}

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAll() returned %d tickets, want 2", len(got))
		}
		if got[0].ID != ticket1.ID || got[0].SeatId != seat1.ID ||
			got[0].HallId != hall.ID || got[0].ShowId != show.ID {
			t.Errorf("GetAll()[0] = %+v", got[0])
		}
		if got[1].ID != ticket2.ID || got[1].SeatId != seat2.ID {
			t.Errorf("GetAll()[1] = %+v", got[1])
		}
	})

	t.Run("returns an empty list when there are no tickets", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewTicketRepository(pool)

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAll() returned %d tickets, want 0", len(got))
		}
	})
}

func TestGetAllTicketsWithDetails(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewTicketRepository(pool)
	seatRepo := models.NewSeatRepository(pool)
	hall, seat1, show := seedTicketParents(t, pool)

	seat2, err := seatRepo.Create(t.Context(), 0, hall.ID, 2, models.Free, 100)
	if err != nil {
		t.Fatalf("seed second seat: %v", err)
	}
	ticket1, err := repo.Create(t.Context(), seat1.ID, hall.ID, show.ID)
	if err != nil {
		t.Fatalf("seed first ticket: %v", err)
	}
	ticket2, err := repo.Create(t.Context(), seat2.ID, hall.ID, show.ID)
	if err != nil {
		t.Fatalf("seed second ticket: %v", err)
	}

	got, err := repo.GetAllWithDetails(t.Context())
	if err != nil {
		t.Fatalf("GetAllWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllWithDetails() returned %d tickets, want 2", len(got))
	}

	for i, want := range []*models.Ticket{ticket1, ticket2} {
		ticket := got[i]

		if ticket.ID != want.ID || ticket.SeatId != want.SeatId ||
			ticket.HallId != hall.ID || ticket.ShowId != show.ID {
			t.Errorf("GetAllWithDetails()[%d] = %+v", i, ticket)
		}
		if ticket.Seat == nil {
			t.Fatalf("GetAllWithDetails()[%d].Seat is nil", i)
		}
		if ticket.Seat.ID != want.SeatId {
			t.Errorf("GetAllWithDetails()[%d].Seat.ID = %d, want %d", i, ticket.Seat.ID, want.SeatId)
		}
		if ticket.Seat.Hall == nil || ticket.Seat.Hall.ID != hall.ID {
			t.Errorf("GetAllWithDetails()[%d].Seat.Hall = %+v, want hall %d", i, ticket.Seat.Hall, hall.ID)
		}
		if ticket.Hall == nil || ticket.Hall.ID != hall.ID || ticket.Hall.Name != hall.Name {
			t.Errorf("GetAllWithDetails()[%d].Hall = %+v, want %+v", i, ticket.Hall, hall)
		}
		if ticket.Show == nil || ticket.Show.ID != show.ID || ticket.Show.Name != show.Name {
			t.Errorf("GetAllWithDetails()[%d].Show = %+v, want %+v", i, ticket.Show, show)
		}
	}
}
