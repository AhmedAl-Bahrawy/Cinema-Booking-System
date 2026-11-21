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
// Number is the seat number inside the hall. Price is a snapshot computed at
// sale time from the show price and the seat category:
//
//	price = show.price * seat_category.price_multiplier_percent / 100
type Ticket struct {
	ID          int64        `db:"id"           json:"id"           binding:"omitempty,min=1"`
	ScreeningId int64        `db:"screening_id" json:"screening_id" binding:"required,min=1"`
	SeatId      int64        `db:"seat_id"      json:"seat_id"      binding:"required,min=1"`
	UserId      int64        `db:"user_id"      json:"user_id"      binding:"required,min=1"`
	Number      uint32       `db:"number"       json:"number"       binding:"required,min=1"`
	Price       uint32       `db:"price"        json:"price"        binding:"gte=0"`
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
// of the screening, so a seat from another hall can never be sold. It also
// copies the hall seat number into ticket.number and computes the ticket price
// from the show price and the seat category. The database trigger enforces the
// same seat/number rule, and UNIQUE (screening_id, seat_id) prevents double
// booking.
func (r *TicketRepository) Create(ctx context.Context, screeningId, seatId, userId int64, status TicketStatus) (*Ticket, error) {
	query := `
	INSERT INTO tickets (screening_id, seat_id, user_id, number, price, status)
	SELECT
	    sc.id,
	    s.id,
	    $3,
	    s.seat_number,
	    ROUND(sh.price * c.price_multiplier_percent / 100.0)::INTEGER,
	    $4
	FROM screenings sc
	JOIN shows sh ON sh.id = sc.show_id
	JOIN seats s ON s.id = $2
	JOIN seat_categories c ON c.id = s.category_id
	WHERE sc.id = $1
	  AND s.hall_id = sc.hall_id
	RETURNING id, screening_id, seat_id, user_id, number, price, status`

	var ticket Ticket
	var dbNumber, dbPrice int64
	err := r.db.QueryRow(ctx, query, screeningId, seatId, userId, status).
		Scan(&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &dbNumber, &dbPrice, &ticket.Status)
	if err != nil {
		return nil, err
	}
	ticket.Number = uint32(dbNumber)
	ticket.Price = uint32(dbPrice)

	return &ticket, nil
}

func (r *TicketRepository) GetById(ctx context.Context, id int64) (*Ticket, error) {
	query := `SELECT id, screening_id, seat_id, user_id, number, price, status FROM tickets WHERE id = $1`

	var ticket Ticket
	var dbNumber, dbPrice int64
	err := r.db.QueryRow(ctx, query, id).
		Scan(&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &dbNumber, &dbPrice, &ticket.Status)
	if err != nil {
		return nil, err
	}
	ticket.Number = uint32(dbNumber)
	ticket.Price = uint32(dbPrice)

	return &ticket, nil
}

func (r *TicketRepository) GetAll(ctx context.Context) ([]*Ticket, error) {
	query := `SELECT id, screening_id, seat_id, user_id, number, price, status FROM tickets ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query tickets: %w", err)
	}

	tickets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		return scanTicket(row)
	})
	if err != nil {
		return nil, fmt.Errorf("collect tickets: %w", err)
	}

	return tickets, nil
}

// GetByScreening returns all tickets of one screening (plain rows).
func (r *TicketRepository) GetByScreening(ctx context.Context, screeningId int64) ([]*Ticket, error) {
	query := `
	SELECT id, screening_id, seat_id, user_id, number, price, status
	FROM tickets
	WHERE screening_id = $1
	ORDER BY number`

	rows, err := r.db.Query(ctx, query, screeningId)
	if err != nil {
		return nil, fmt.Errorf("query tickets by screening: %w", err)
	}

	tickets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		return scanTicket(row)
	})
	if err != nil {
		return nil, fmt.Errorf("collect tickets by screening: %w", err)
	}

	return tickets, nil
}

// GetByUser returns all tickets owned by one user (plain rows).
func (r *TicketRepository) GetByUser(ctx context.Context, userId int64) ([]*Ticket, error) {
	query := `
	SELECT id, screening_id, seat_id, user_id, number, price, status
	FROM tickets
	WHERE user_id = $1
	ORDER BY id`

	rows, err := r.db.Query(ctx, query, userId)
	if err != nil {
		return nil, fmt.Errorf("query tickets by user: %w", err)
	}

	tickets, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Ticket, error) {
		return scanTicket(row)
	})
	if err != nil {
		return nil, fmt.Errorf("collect tickets by user: %w", err)
	}

	return tickets, nil
}

// GetWithDetails returns the ticket with its Seat (and seat category), User and
// Screening filled in. The Screening also carries its Hall and Show, so a
// ticket can answer: which seat number, which user, which price, which hall and
// which show.
func (r *TicketRepository) GetWithDetails(ctx context.Context, id int64) (*Ticket, error) {
	query := `
	SELECT
	    t.id, t.screening_id, t.seat_id, t.user_id, t.number, t.price, t.status,
	    s.id, s.hall_id, s.seat_number, s.category_id,
	    c.id, c.name, c.price_multiplier_percent,
	    u.id, u.username, u.full_name,
	    sc.id, sc.hall_id, sc.show_id, sc.starts_at, sc.ends_at,
	    h.id, h.name, h.capacity,
	    sh.id, sh.name, sh.duration_minutes, sh.author_name, sh.price
	FROM tickets t
	JOIN seats s       ON s.id  = t.seat_id
	JOIN seat_categories c ON c.id = s.category_id
	JOIN users u       ON u.id  = t.user_id
	JOIN screenings sc ON sc.id = t.screening_id
	JOIN halls h       ON h.id  = sc.hall_id
	JOIN shows sh      ON sh.id = sc.show_id
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
	    t.id, t.screening_id, t.seat_id, t.user_id, t.number, t.price, t.status,
	    s.id, s.hall_id, s.seat_number, s.category_id,
	    c.id, c.name, c.price_multiplier_percent,
	    u.id, u.username, u.full_name,
	    sc.id, sc.hall_id, sc.show_id, sc.starts_at, sc.ends_at,
	    h.id, h.name, h.capacity,
	    sh.id, sh.name, sh.duration_minutes, sh.author_name, sh.price
	FROM tickets t
	JOIN seats s       ON s.id  = t.seat_id
	JOIN seat_categories c ON c.id = s.category_id
	JOIN users u       ON u.id  = t.user_id
	JOIN screenings sc ON sc.id = t.screening_id
	JOIN halls h       ON h.id  = sc.hall_id
	JOIN shows sh      ON sh.id = sc.show_id
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

// scanTicket scans one plain ticket row.
func scanTicket(row pgx.CollectableRow) (*Ticket, error) {
	var ticket Ticket
	var dbNumber, dbPrice int64
	err := row.Scan(
		&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId,
		&dbNumber, &dbPrice, &ticket.Status,
	)
	if err != nil {
		return nil, err
	}
	ticket.Number = uint32(dbNumber)
	ticket.Price = uint32(dbPrice)
	return &ticket, nil
}

// scanTicketWithDetails scans one ticket + seat + category + user + screening +
// hall + show row.
func scanTicketWithDetails(row pgx.CollectableRow) (*Ticket, error) {
	var ticket Ticket
	seat := Seat{}
	category := SeatCategory{}
	user := User{}
	screening := Screening{}
	hall := Hall{}
	show := Show{}

	var dbNumber, dbPrice, dbSeatNumber, dbMultiplier, dbCapacity, dbShowPrice int64

	err := row.Scan(
		&ticket.ID, &ticket.ScreeningId, &ticket.SeatId, &ticket.UserId, &dbNumber, &dbPrice, &ticket.Status,
		&seat.ID, &seat.HallId, &dbSeatNumber, &seat.CategoryId,
		&category.ID, &category.Name, &dbMultiplier,
		&user.ID, &user.Username, &user.FullName,
		&screening.ID, &screening.HallId, &screening.ShowId, &screening.StartsAt, &screening.EndsAt,
		&hall.ID, &hall.Name, &dbCapacity,
		&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName, &dbShowPrice,
	)
	if err != nil {
		return nil, err
	}

	ticket.Number = uint32(dbNumber)
	ticket.Price = uint32(dbPrice)

	seat.SeatNumber = uint32(dbSeatNumber)
	category.PriceMultiplierPercent = uint32(dbMultiplier)
	seat.Category = &category
	seat.Hall = &hall

	hall.Capacity = uint32(dbCapacity)
	show.Price = uint32(dbShowPrice)

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
