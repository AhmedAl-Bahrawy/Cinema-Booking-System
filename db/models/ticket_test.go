package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
)

func TestCreateTicket(t *testing.T) {
	tests := []struct {
		name       string // description of this test case
		seatFrom   string // "same hall" or "other hall" or "missing"
		screening  string // "existing" or "missing"
		user       string // "existing" or "missing"
		status     models.TicketStatus
		wantErr    bool
		wantErrRow bool // true when the INSERT ... SELECT simply matches no row
	}{
		{
			name:      "creates a booked ticket",
			seatFrom:  "same hall",
			screening: "existing",
			user:      "existing",
			status:    models.Booked,
		},
		{
			name:      "creates a held ticket",
			seatFrom:  "same hall",
			screening: "existing",
			user:      "existing",
			status:    models.Held,
		},
		{
			// The seat exists but belongs to another hall, so the INSERT ...
			// SELECT must not match.
			name:       "seat from another hall is rejected",
			seatFrom:   "other hall",
			screening:  "existing",
			user:       "existing",
			status:     models.Booked,
			wantErr:    true,
			wantErrRow: true,
		},
		{
			name:       "non-existent screening is rejected",
			seatFrom:   "same hall",
			screening:  "missing",
			user:       "existing",
			status:     models.Booked,
			wantErr:    true,
			wantErrRow: true,
		},
		{
			name:      "non-existent user is rejected by the foreign key",
			seatFrom:  "same hall",
			screening: "existing",
			user:      "missing",
			status:    models.Booked,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewTicketRepository(pool)
			hall, seat, _, screening := seedTicketParents(t, pool)
			user := seedUser(t, pool)

			seatID := seat.ID
			switch tt.seatFrom {
			case "other hall":
				otherHall, err := models.NewHallRepository(pool).Create(t.Context(), "Hall B", 50)
				if err != nil {
					t.Fatalf("seed other hall: %v", err)
				}
				otherSeat := seedSeat(t, pool, otherHall.ID, 1, 1)
				seatID = otherSeat.ID
			case "missing":
				seatID = 999
			}

			screeningID := screening.ID
			if tt.screening == "missing" {
				screeningID = 999
			}

			userID := user.ID
			if tt.user == "missing" {
				userID = 999
			}

			got, gotErr := repo.Create(t.Context(), screeningID, seatID, userID, tt.status)
			if gotErr != nil {
				if !tt.wantErr {
					t.Fatalf("Create() failed: %v", gotErr)
				}
				if tt.wantErrRow && !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("Create() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}

			if got.ID != 1 || got.ScreeningId != screening.ID || got.SeatId != seat.ID ||
				got.UserId != user.ID || got.Status != tt.status {
				t.Errorf("Create() = %+v", got)
			}
			// The normal seat number becomes the ticket number, and the price is
			// show.price (100) × normal multiplier (100%) = 100.
			if got.Number != 1 || got.Price != 100 {
				t.Errorf("Create() number/price = %d/%d, want 1/100", got.Number, got.Price)
			}
			if got.SeatId != seat.ID || seat.HallId != hall.ID {
				t.Errorf("ticket seat %d does not belong to hall %d", got.SeatId, hall.ID)
			}
		})
	}
}

// TestCreateTicket_DoubleBooking verifies UNIQUE (screening_id, seat_id): the
// same seat cannot be sold twice for one screening.
func TestCreateTicket_DoubleBooking(t *testing.T) {
	pool := testdb.Setup(t)
	ticketRepo := models.NewTicketRepository(pool)
	_, seat, _, screening := seedTicketParents(t, pool)
	user := seedUser(t, pool)

	first, err := ticketRepo.Create(t.Context(), screening.ID, seat.ID, user.ID, models.Booked)
	if err != nil {
		t.Fatalf("Create() first insert failed: %v", err)
	}

	second, err := ticketRepo.Create(t.Context(), screening.ID, seat.ID, user.ID, models.Booked)
	if err == nil {
		t.Errorf("Create() succeeded unexpectedly for double booking: %+v", second)
	}
	if second != nil {
		t.Errorf("Create() returned a ticket on error: %+v", second)
	}
	if first.ID == 0 {
		t.Fatal("first ticket has no id")
	}
}

// TestCreateTicket_SeatReusedAcrossScreenings is the core relation test: a
// physical seat is reusable. The same seat can be sold to the same user (or
// another user) for a different screening on the same day.
func TestCreateTicket_SeatReusedAcrossScreenings(t *testing.T) {
	pool := testdb.Setup(t)
	ticketRepo := models.NewTicketRepository(pool)
	hall, seat, show, firstScreening := seedTicketParents(t, pool)
	user := seedUser(t, pool)

	secondScreening := seedScreening(t, pool, hall.ID, show.ID, screeningBase.Add(2*time.Hour))

	first, err := ticketRepo.Create(t.Context(), firstScreening.ID, seat.ID, user.ID, models.Booked)
	if err != nil {
		t.Fatalf("Create() first screening ticket failed: %v", err)
	}
	second, err := ticketRepo.Create(t.Context(), secondScreening.ID, seat.ID, user.ID, models.Booked)
	if err != nil {
		t.Fatalf("Create() second screening ticket failed: %v", err)
	}

	if first.SeatId != second.SeatId {
		t.Errorf("tickets use different seats: %d and %d", first.SeatId, second.SeatId)
	}
	if first.ScreeningId == second.ScreeningId {
		t.Errorf("both tickets point at the same screening: %d", first.ScreeningId)
	}
}

// TestCreateTicket_SeatHallMustMatchScreening verifies that the ticket cannot
// link a seat from hall A to a screening in hall B.
func TestCreateTicket_SeatHallMustMatchScreening(t *testing.T) {
	pool := testdb.Setup(t)
	ticketRepo := models.NewTicketRepository(pool)
	user := seedUser(t, pool)

	hallA := seedHall(t, pool)
	hallB, err := models.NewHallRepository(pool).Create(t.Context(), "Hall B", 50)
	if err != nil {
		t.Fatalf("seed hall B: %v", err)
	}
	show := seedShow(t, pool)
	seatInA := seedSeat(t, pool, hallA.ID, 1, 1)
	screeningInB := seedScreening(t, pool, hallB.ID, show.ID, screeningBase)

	if _, err := ticketRepo.Create(t.Context(), screeningInB.ID, seatInA.ID, user.ID, models.Booked); err == nil {
		t.Fatal("Create() succeeded for a seat from another hall")
	}
}

func TestGetTicketWithDetails(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		wantErr bool
	}{
		{
			name:    "returns the ticket with seat, user, screening, hall and show",
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

			hall, seat, show, screening := seedTicketParents(t, pool)
			user := seedUser(t, pool)
			if _, err := repo.Create(t.Context(), screening.ID, seat.ID, user.ID, models.Booked); err != nil {
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
			if got.ID != 1 || got.ScreeningId != screening.ID || got.SeatId != seat.ID ||
				got.UserId != user.ID || got.Status != models.Booked {
				t.Errorf("GetWithDetails() ticket fields = %+v", got)
			}
			if got.Number != 1 || got.Price != 100 {
				t.Errorf("GetWithDetails() number/price = %d/%d, want 1/100", got.Number, got.Price)
			}

			// Ticket → Seat → Category → Hall.
			if got.Seat == nil {
				t.Fatal("GetWithDetails() returned a nil Seat")
			}
			if got.Seat.ID != seat.ID || got.Seat.SeatNumber != 1 {
				t.Errorf("GetWithDetails() Seat = %+v", got.Seat)
			}
			if got.Seat.Category == nil || got.Seat.Category.Name != "normal" ||
				got.Seat.Category.PriceMultiplierPercent != 100 {
				t.Errorf("GetWithDetails() Seat.Category = %+v, want normal (100)", got.Seat.Category)
			}
			if got.Seat.Hall == nil || got.Seat.Hall.ID != hall.ID {
				t.Errorf("GetWithDetails() Seat.Hall = %+v, want hall %d", got.Seat.Hall, hall.ID)
			}

			// Ticket → User.
			if got.User == nil || got.User.ID != user.ID || got.User.Username != "ahmed" {
				t.Errorf("GetWithDetails() User = %+v, want user %d", got.User, user.ID)
			}

			// Ticket → Screening → Hall + Show.
			if got.Screening == nil {
				t.Fatal("GetWithDetails() returned a nil Screening")
			}
			if got.Screening.ID != screening.ID || !got.Screening.StartsAt.Equal(screening.StartsAt) {
				t.Errorf("GetWithDetails() Screening = %+v", got.Screening)
			}
			if got.Screening.Hall == nil || got.Screening.Hall.ID != hall.ID {
				t.Errorf("GetWithDetails() Screening.Hall = %+v, want hall %d", got.Screening.Hall, hall.ID)
			}
			if got.Screening.Show == nil || got.Screening.Show.ID != show.ID || got.Screening.Show.Name != "Hamlet" {
				t.Errorf("GetWithDetails() Screening.Show = %+v, want show %d", got.Screening.Show, show.ID)
			}
		})
	}
}

func TestGetTicketById(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		wantErr bool
	}{
		{
			name:    "returns the ticket for an existing id",
			id:      1,
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

			_, seat, _, screening := seedTicketParents(t, pool)
			user := seedUser(t, pool)
			if _, err := repo.Create(t.Context(), screening.ID, seat.ID, user.ID, models.Booked); err != nil {
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
			if got.ID != 1 || got.ScreeningId != screening.ID || got.SeatId != seat.ID ||
				got.UserId != user.ID || got.Status != models.Booked {
				t.Errorf("GetById() = %+v", got)
			}
			if got.Number != 1 || got.Price != 100 {
				t.Errorf("GetById() number/price = %d/%d, want 1/100", got.Number, got.Price)
			}
		})
	}
}

func TestGetAllTickets(t *testing.T) {
	t.Run("returns all tickets ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewTicketRepository(pool)
		hall, seat, show, screening := seedTicketParents(t, pool)
		user := seedUser(t, pool)
		secondSeat := seedSeat(t, pool, hall.ID, 2, 1)

		first, err := repo.Create(t.Context(), screening.ID, seat.ID, user.ID, models.Booked)
		if err != nil {
			t.Fatalf("seed first ticket: %v", err)
		}
		second, err := repo.Create(t.Context(), screening.ID, secondSeat.ID, user.ID, models.Held)
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
		if got[0].ID != first.ID || got[0].SeatId != seat.ID ||
			got[0].ScreeningId != screening.ID || got[0].UserId != user.ID {
			t.Errorf("GetAll()[0] = %+v", got[0])
		}
		if got[1].ID != second.ID || got[1].SeatId != secondSeat.ID || got[1].Status != models.Held {
			t.Errorf("GetAll()[1] = %+v", got[1])
		}
		if hall.ID == 0 || show.ID == 0 {
			t.Fatal("parents were not seeded")
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
	hall, seat1, show, screening := seedTicketParents(t, pool)
	user := seedUser(t, pool)
	seat2 := seedSeat(t, pool, hall.ID, 2, 1)

	ticket1, err := repo.Create(t.Context(), screening.ID, seat1.ID, user.ID, models.Booked)
	if err != nil {
		t.Fatalf("seed first ticket: %v", err)
	}
	ticket2, err := repo.Create(t.Context(), screening.ID, seat2.ID, user.ID, models.Held)
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
			ticket.ScreeningId != screening.ID || ticket.UserId != user.ID {
			t.Errorf("GetAllWithDetails()[%d] = %+v", i, ticket)
		}
		if ticket.Seat == nil || ticket.Seat.ID != want.SeatId {
			t.Errorf("GetAllWithDetails()[%d].Seat = %+v", i, ticket.Seat)
		}
		if ticket.Seat.Category == nil || ticket.Seat.Category.Name != "normal" {
			t.Errorf("GetAllWithDetails()[%d].Seat.Category = %+v, want normal", i, ticket.Seat.Category)
		}
		if ticket.Number != want.Number || ticket.Price != 100 {
			t.Errorf("GetAllWithDetails()[%d] number/price = %d/%d, want %d/100",
				i, ticket.Number, ticket.Price, want.Number)
		}
		if ticket.Seat.Hall == nil || ticket.Seat.Hall.ID != hall.ID {
			t.Errorf("GetAllWithDetails()[%d].Seat.Hall = %+v, want hall %d", i, ticket.Seat.Hall, hall.ID)
		}
		if ticket.User == nil || ticket.User.ID != user.ID {
			t.Errorf("GetAllWithDetails()[%d].User = %+v, want user %d", i, ticket.User, user.ID)
		}
		if ticket.Screening == nil || ticket.Screening.ID != screening.ID {
			t.Errorf("GetAllWithDetails()[%d].Screening = %+v", i, ticket.Screening)
		}
		if ticket.Screening.Show == nil || ticket.Screening.Show.ID != show.ID {
			t.Errorf("GetAllWithDetails()[%d].Screening.Show = %+v, want show %d", i, ticket.Screening.Show, show.ID)
		}
	}
}

func TestGetTicketsByScreeningAndUser(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewTicketRepository(pool)
	hall, seat1, show, screening1 := seedTicketParents(t, pool)
	user := seedUser(t, pool)
	seat2 := seedSeat(t, pool, hall.ID, 2, 1)
	screening2 := seedScreening(t, pool, hall.ID, show.ID, screeningBase.Add(2*time.Hour))

	if _, err := repo.Create(t.Context(), screening1.ID, seat1.ID, user.ID, models.Booked); err != nil {
		t.Fatalf("seed first ticket: %v", err)
	}
	if _, err := repo.Create(t.Context(), screening1.ID, seat2.ID, user.ID, models.Booked); err != nil {
		t.Fatalf("seed second ticket: %v", err)
	}
	if _, err := repo.Create(t.Context(), screening2.ID, seat1.ID, user.ID, models.Booked); err != nil {
		t.Fatalf("seed third ticket: %v", err)
	}

	byScreening, err := repo.GetByScreening(t.Context(), screening1.ID)
	if err != nil {
		t.Fatalf("GetByScreening() failed: %v", err)
	}
	if len(byScreening) != 2 {
		t.Errorf("GetByScreening() returned %d tickets, want 2", len(byScreening))
	}

	byUser, err := repo.GetByUser(t.Context(), user.ID)
	if err != nil {
		t.Fatalf("GetByUser() failed: %v", err)
	}
	if len(byUser) != 3 {
		t.Errorf("GetByUser() returned %d tickets, want 3", len(byUser))
	}
}

// TestCreateTicket_PricingBySeatCategory verifies the core pricing rule:
//
//	ticket.price = show.price × seat_category.multiplier / 100
//	ticket.number = seat.seat_number
func TestCreateTicket_PricingBySeatCategory(t *testing.T) {
	pool := testdb.Setup(t)
	ticketRepo := models.NewTicketRepository(pool)
	showRepo := models.NewShowRepository(pool)
	seatRepo := models.NewSeatRepository(pool)

	hall := seedHall(t, pool)
	// Base price 80, so the arithmetic is not trivially the multiplier.
	show, err := showRepo.Create(t.Context(), "Hamlet", 120, "Shakespeare", 80)
	if err != nil {
		t.Fatalf("seed show: %v", err)
	}
	screening := seedScreening(t, pool, hall.ID, show.ID, screeningBase)
	user := seedUser(t, pool)

	boxCategory := seedSeatCategory(t, pool, "box", 125)
	if boxCategory.ID == 0 {
		t.Fatal("box category was not created")
	}

	tests := []struct {
		seatNumber uint32
		categoryId int64
		wantPrice  uint32
	}{
		{seatNumber: 1, categoryId: 1, wantPrice: 80},               // normal 100%
		{seatNumber: 2, categoryId: 2, wantPrice: 120},              // premium 150%
		{seatNumber: 3, categoryId: 3, wantPrice: 160},              // VIP 200%
		{seatNumber: 4, categoryId: boxCategory.ID, wantPrice: 100}, // box 125%
	}

	for _, tt := range tests {
		seat, err := seatRepo.Create(t.Context(), hall.ID, tt.seatNumber, tt.categoryId)
		if err != nil {
			t.Fatalf("seed seat %d: %v", tt.seatNumber, err)
		}

		ticket, err := ticketRepo.Create(t.Context(), screening.ID, seat.ID, user.ID, models.Booked)
		if err != nil {
			t.Fatalf("Create() for seat %d failed: %v", tt.seatNumber, err)
		}

		// The ticket number is the seat number inside the hall.
		if ticket.Number != tt.seatNumber {
			t.Errorf("ticket for seat %d has number %d, want %d",
				tt.seatNumber, ticket.Number, tt.seatNumber)
		}
		if ticket.Price != tt.wantPrice {
			t.Errorf("ticket for category %d costs %d, want %d",
				tt.categoryId, ticket.Price, tt.wantPrice)
		}
	}
}
