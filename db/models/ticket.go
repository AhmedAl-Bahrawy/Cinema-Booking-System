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

func GetTicketById(ctx context.Context, pool *pgxpool.Pool, id int64) (*Ticket, error) {
	query := `SELECT id, seat_id, hall_id, show_id FROM tickets WHERE id = $1`

	var ticket Ticket
	err := pool.QueryRow(ctx, query, id).Scan(&ticket.ID, &ticket.SeatId, &ticket.HallId, &ticket.ShowId)
	if err != nil {
		return nil, err
	}

	return &ticket, nil
}
