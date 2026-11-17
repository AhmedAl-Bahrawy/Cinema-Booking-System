package models

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Ticket struct {
	ID     int64 `db:"id"      json:"id"      binding:"omitempty,min=1"`
	SeatId int64 `db:"seat_id" json:"seat_id" binding:"required,min=1"`
	HallId int64 `db:"hall_id" json:"hall_id" binding:"required,min=1"`
	ShowId int64 `db:"show_id" json:"show_id" binding:"required,min=1"`

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Seat *Seat `db:"-" json:"seat,omitempty" binding:"-"`
	Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
	Show *Show `db:"-" json:"show,omitempty" binding:"-"`
}

func CreateTicket(ctx context.Context, pool *pgxpool.Pool, seatId, hallId, showId int64) (*Ticket, error) {
	query := `INSERT INTO tickets (seat_id, hall_id, show_id) VALUES ($1, $2, $3) RETURNING id, seat_id, hall_id, show_id`

	var ticket Ticket
	err := pool.QueryRow(ctx, query, seatId, hallId, showId).Scan(&ticket.ID, &ticket.SeatId, &ticket.HallId, &ticket.ShowId)
	if err != nil {
		return nil, err
	}

	return &ticket, nil
}

// GetTicketWithDetails returns the ticket with its Seat (including the Seat's
// own Hall), Hall, and Show filled in from a JOIN query.
func GetTicketWithDetails(ctx context.Context, pool *pgxpool.Pool, id int64) (*Ticket, error) {
	query := `
	SELECT
	    t.id, t.seat_id, t.hall_id, t.show_id,
	    s.id, s.user_id, s.hall_id, s.seat_number, s.status, s.price,
	    h.id, h.name, h.capacity,
	    sh.id, sh.name, sh.duration_minutes, sh.author_name
	FROM tickets t
	JOIN seats s  ON s.id  = t.seat_id
	JOIN halls h  ON h.id  = t.hall_id
	JOIN shows sh ON sh.id = t.show_id
	WHERE t.id = $1`

	var ticket Ticket
	seat := Seat{}
	hall := Hall{}
	show := Show{}

	var dbUserId *int64 // NULL until the seat is held/booked
	var dbSeatNumber, dbPrice, dbCapacity int64

	err := pool.QueryRow(ctx, query, id).Scan(
		&ticket.ID, &ticket.SeatId, &ticket.HallId, &ticket.ShowId,
		&seat.ID, &dbUserId, &seat.HallId, &dbSeatNumber, &seat.Status, &dbPrice,
		&hall.ID, &hall.Name, &dbCapacity,
		&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName,
	)
	if err != nil {
		return nil, err
	}
	if dbUserId != nil {
		seat.UserId = *dbUserId
	}
	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)
	hall.Capacity = uint32(dbCapacity)

	// Assign the joined structs into the ticket.
	seat.Hall = &hall
	ticket.Seat = &seat
	ticket.Hall = &hall
	ticket.Show = &show

	return &ticket, nil
}

func GetTicketById(ctx context.Context, pool *pgxpool.Pool, id int64) (*Ticket, error) {
	query := `SELECT id, seat_id, hall_id, show_id FROM tickets WHERE id = $1`

	var ticket Ticket
	err := pool.QueryRow(ctx, query, id).Scan(&ticket.ID, &ticket.SeatId, &ticket.HallId, &ticket.ShowId)
	if err != nil {
		return nil, err
	}

	return &ticket, nil
}
