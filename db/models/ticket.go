package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// TicketStatus is the lifecycle of a booking.
type TicketStatus int16

const (
	Held      TicketStatus = 1 // temporarily reserved
	Booked    TicketStatus = 2 // paid / confirmed
	Cancelled TicketStatus = 3 // released again
)

// Ticket is the booking record. It is the only place where a user, a seat and
// a screening meet:
//
//	ticket → seat      (which physical seat)
//	ticket → user      (who owns it)
//	ticket → screening (which show, in which hall, at which time)
//
// Because the seat is not stored on the user or on the physical seat row, the
// same seat can be sold to the same (or another) user for a later screening.
type Ticket struct {
	ID          int64        `db:"id"           json:"id"           binding:"omitempty,min=1"`
	ScreeningId int64        `db:"screening_id" json:"screening_id" binding:"required,min=1"`
	SeatId      int64        `db:"seat_id"      json:"seat_id"      binding:"required,min=1"`
	UserId      int64        `db:"user_id"      json:"user_id"      binding:"required,min=1"`
	Status      TicketStatus `db:"status"       json:"status"       binding:"oneof=1 2 3"`

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Seat      *Seat      `db:"-" json:"seat,omitempty"      binding:"-"`
	User      *User      `db:"-" json:"user,omitempty"      binding:"-"`
	Screening *Screening `db:"-" json:"screening,omitempty" binding:"-"`
}

type TicketReq struct {
	ScreeningId int64        `db:"screening_id" json:"screening_id" binding:"required,min=1"`
	SeatId      int64        `db:"seat_id"      json:"seat_id"      binding:"required,min=1"`
	UserId      int64        `db:"user_id"      json:"user_id"      binding:"required,min=1"`
	Status      TicketStatus `db:"status"       json:"status"       binding:"omitempty,oneof=1 2 3"`
}

type TicketRepository struct {
	db *pgxpool.Pool
}

func NewTicketRepository(db *pgxpool.Pool) *TicketRepository {
	return &TicketRepository{db: db}
}

// Create sells a seat for a screening to a user.
//
// The INSERT ... SELECT only produces a row when the seat belongs to the hall
// of the screening, so a seat from another hall can never be sold. The database
// trigger enforces the same rule, and UNIQUE (screening_id, seat_id) prevents
// double booking.
func (r *TicketRepository) Create(ctx context.Context, screeningId, seatId, userId int64, status TicketStatus) (*Ticket, error) {
	query := `
	INSERT INTO tickets (screening_id, seat_id, user_id, status)
	SELECT sc.id, s.id, $3, $4
	FROM screenings sc
	JOIN seats s ON s.id = $2
	WHERE sc.id = $1
	  AND s.hall_id = sc.hall_id
	RETURNING id, screening_id, seat_id, user_id, status`

	var ticket Ticket
	err := r.db.QueryRow(ctx, query, screeningId, seatId, userId, status).
		Scan(&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &ticket.Status)
	if err != nil {
		return nil, err
	}

	return &ticket, nil
}

func (r *TicketRepository) GetById(ctx context.Context, id int64) (*Ticket, error) {
	query := `SELECT id, screening_id, seat_id, user_id, status FROM tickets WHERE id = $1`

	var ticket Ticket
	err := r.db.QueryRow(ctx, query, id).
		Scan(&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &ticket.Status)
	if err != nil {
		return nil, err
	}

	return &ticket, nil
}

func (r *TicketRepository) GetAll(ctx context.Context) ([]*Ticket, error) {
	query := `SELECT id, screening_id, seat_id, user_id, status FROM tickets ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query tickets: %w", err)
	}

	tickets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		var ticket Ticket
		err := row.Scan(&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &ticket.Status)
		return &ticket, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect tickets: %w", err)
	}

	return tickets, nil
}

// GetByScreening returns all tickets of one screening (plain rows).
func (r *TicketRepository) GetByScreening(ctx context.Context, screeningId int64) ([]*Ticket, error) {
	query := `SELECT id, screening_id, seat_id, user_id, status FROM tickets WHERE screening_id = $1 ORDER BY seat_id`

	rows, err := r.db.Query(ctx, query, screeningId)
	if err != nil {
		return nil, fmt.Errorf("query tickets by screening: %w", err)
	}

	tickets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		var ticket Ticket
		err := row.Scan(&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &ticket.Status)
		return &ticket, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect tickets by screening: %w", err)
	}

	return tickets, nil
}

// GetByUser returns all tickets owned by one user (plain rows).
func (r *TicketRepository) GetByUser(ctx context.Context, userId int64) ([]*Ticket, error) {
	query := `SELECT id, screening_id, seat_id, user_id, status FROM tickets WHERE user_id = $1 ORDER BY id`

	rows, err := r.db.Query(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("query tickets by user: %w", err)
	}

	tickets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		var ticket Ticket
		err := row.Scan(&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &ticket.Status)
		return &ticket, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect tickets by user: %w", err)
	}

	return tickets, nil
}

// GetWithDetails returns the ticket with its Seat, User and Screening filled
// in. The Screening also carries its Hall and Show, so a ticket can answer:
// which seat, which user, which hall and which show.
func (r *TicketRepository) GetWithDetails(ctx context.Context, id int64) (*Ticket, error) {
	query := `
	SELECT
	    t.id, t.screening_id, t.seat_id, t.user_id, t.status,
	    s.id, s.hall_id, s.seat_number, s.price,
	    u.id, u.username, u.full_name,
	    sc.id, sc.hall_id, sc.show_id, sc.starts_at, sc.ends_at,
	    h.id, h.name, h.capacity,
	    sh.id, sh.name, sh.duration_minutes, sh.author_name
	FROM tickets t
	JOIN seats s      ON s.id  = t.seat_id
	JOIN users u      ON u.id  = t.user_id
	JOIN screenings sc ON sc.id = t.screening_id
	JOIN halls h      ON h.id  = sc.hall_id
	JOIN shows sh     ON sh.id = sc.show_id
	WHERE t.id = $1`

	rows, err := r.db.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query ticket with details: %w", err)
	}

	ticket, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		return scanTicketWithDetails(row)
	})
	if err != nil {
		return nil, err
	}

	return ticket, nil
}

// GetAllWithDetails returns every ticket with its Seat, User and Screening
// (including hall and show) filled in.
func (r *TicketRepository) GetAllWithDetails(ctx context.Context) ([]*Ticket, error) {
	query := `
	SELECT
	    t.id, t.screening_id, t.seat_id, t.user_id, t.status,
	    s.id, s.hall_id, s.seat_number, s.price,
	    u.id, u.username, u.full_name,
	    sc.id, sc.hall_id, sc.show_id, sc.starts_at, sc.ends_at,
	    h.id, h.name, h.capacity,
	    sh.id, sh.name, sh.duration_minutes, sh.author_name
	FROM tickets t
	JOIN seats s      ON s.id  = t.seat_id
	JOIN users u      ON u.id  = t.user_id
	JOIN screenings sc ON sc.id = t.screening_id
	JOIN halls h      ON h.id  = sc.hall_id
	JOIN shows sh     ON sh.id = sc.show_id
	ORDER BY t.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query tickets with details: %w", err)
	}

	tickets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		return scanTicketWithDetails(row)
	})
	if err != nil {
		return nil, fmt.Errorf("collect tickets with details: %w", err)
	}

	return tickets, nil
}

// scanTicketWithDetails scans one ticket + seat + user + screening + hall + show row.
func scanTicketWithDetails(row pgx.CollectableRow) (*Ticket, error) {
	var ticket Ticket
	seat := Seat{}
	user := User{}
	screening := Screening{}
	hall := Hall{}
	show := Show{}

	var dbSeatNumber, dbPrice, dbCapacity int64

	err := row.Scan(
		&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &ticket.Status,
		&seat.ID, &seat.HallId, &dbSeatNumber, &dbPrice,
		&user.ID, &user.Username, &user.FullName,
		&screening.ID, &screening.HallId, &screening.ShowId, &screening.StartsAt, &screening.EndsAt,
		&hall.ID, &hall.Name, &dbCapacity,
		&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName,
	)
	if err != nil {
		return nil, err
	}

	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)
	hall.Capacity = uint32(dbCapacity)

	// Seat → hall.
	seat.Hall = &hall
	// Screening → hall + show.
	screening.Hall = &hall
	screening.Show = &show

	// Ticket → seat + user + screening.
	// Leave the back-references (Seat.Tickets, User.Tickets, Screening.Tickets,
	// Hall.Screenings, Show.Screenings) nil to avoid JSON cycles.
	ticket.Seat = &seat
	ticket.User = &user
	ticket.Screening = &screening

	return &ticket, nil
}
