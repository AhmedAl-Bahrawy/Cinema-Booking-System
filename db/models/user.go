package models

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
	Username string `db:"username" json:"username" binding:"required,min=3,max=100"`
	FullName string `db:"full_name" json:"full_name" binding:"required,min=3,max=255"`

	// A user owns tickets, not physical seats. The same user can hold several
	// tickets for different screenings (even on the same day).
	Tickets []*Ticket `db:"-" json:"tickets,omitempty" binding:"-"`
}

type UserReq struct {
	Username string `db:"username" json:"username" binding:"required,min=3,max=100"`
	FullName string `db:"full_name" json:"full_name" binding:"required,min=3,max=255"`
}

type UserTicket struct {
	userID   int64
	username string
	fullName string

	ticketID    *int64 // NULL when the user has no tickets
	screeningID *int64
	seatID      *int64
	number      *int64
	ticketPrice *int64
	status      *int16

	seatHallID         *int64
	seatNumber         *int64
	seatCategoryID     *int64
	categoryName       *string
	categoryMultiplier *int64

	screeningHallID *int64
	showID          *int64
	startsAt        *time.Time
	endsAt          *time.Time
	hallID          *int64
	hallName        *string
	capacity        *int64
	showName        *string
	durationMinutes *int64
	authorName      *string
	showPrice       *int64
}

type UserRepository struct {
	db *pgxpool.Pool
}

func NewUserRepository(db *pgxpool.Pool) *UserRepository {
	return &UserRepository{db: db}
}

func (r *UserRepository) Create(ctx context.Context, username string, fullName string) (*User, error) {
	query := `INSERT INTO users (username, full_name) VALUES ($1, $2) RETURNING id, username, full_name`

	var user User
	err := r.db.QueryRow(ctx, query, username, fullName).Scan(&user.ID, &user.Username, &user.FullName)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetById(ctx context.Context, id int64) (*User, error) {
	query := `SELECT id, username, full_name FROM users WHERE id = $1`

	var user User
	err := r.db.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.FullName)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func (r *UserRepository) GetAll(ctx context.Context) ([]*User, error) {
	query := `SELECT id, username, full_name FROM users ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query users: %w", err)
	}

	users, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*User, error) {
		var user User
		err := row.Scan(&user.ID, &user.Username, &user.FullName)
		return &user, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect users: %w", err)
	}

	return users, nil
}

// GetAllWithDetails returns every user with their tickets filled in. Each
// ticket carries its Seat and its Screening (with Hall and Show).
// Users without tickets still appear, with an empty Tickets list.
func (r *UserRepository) GetAllWithDetails(ctx context.Context) ([]*User, error) {
	query := `
	SELECT
	    u.id, u.username, u.full_name,
	    t.id, t.screening_id, t.seat_id, t.number, t.price, t.status,
	    s.hall_id, s.seat_number, s.category_id,
	    c.name, c.price_multiplier_percent,
	    sc.hall_id, sc.show_id, sc.starts_at, sc.ends_at,
	    h.id, h.name, h.capacity,
	    sh.name, sh.duration_minutes, sh.author_name, sh.price
	FROM users u
	LEFT JOIN tickets t ON t.user_id = u.id
	LEFT JOIN seats s ON s.id = t.seat_id
	LEFT JOIN seat_categories c ON c.id = s.category_id
	LEFT JOIN screenings sc ON sc.id = t.screening_id
	LEFT JOIN halls h ON h.id = sc.hall_id
	LEFT JOIN shows sh ON sh.id = sc.show_id
	ORDER BY u.id, t.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query users with details: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (UserTicket, error) {
		var r UserTicket
		err := row.Scan(
			&r.userID, &r.username, &r.fullName,
			&r.ticketID, &r.screeningID, &r.seatID, &r.number, &r.ticketPrice, &r.status,
			&r.seatHallID, &r.seatNumber, &r.seatCategoryID,
			&r.categoryName, &r.categoryMultiplier,
			&r.screeningHallID, &r.showID, &r.startsAt, &r.endsAt,
			&r.hallID, &r.hallName, &r.capacity,
			&r.showName, &r.durationMinutes, &r.authorName, &r.showPrice,
		)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect users with details: %w", err)
	}

	// Group the flat rows: one user per id, with their tickets attached.
	users := make([]*User, 0, len(raw))
	byID := make(map[int64]*User)
	for _, r := range raw {
		user, ok := byID[r.userID]
		if !ok {
			user = &User{ID: r.userID, Username: r.username, FullName: r.fullName}
			byID[r.userID] = user
			users = append(users, user)
		}
		if r.ticketID == nil {
			continue
		}

		hall := &Hall{ID: *r.hallID, Name: *r.hallName, Capacity: uint32(*r.capacity)}
		show := &Show{
			ID:              *r.showID,
			Name:            *r.showName,
			DurationMinutes: *r.durationMinutes,
			AuthorName:      *r.authorName,
			Price:           uint32(*r.showPrice),
		}

		// Ticket.User, Seat.Tickets, Screening.Tickets and Hall/Show
		// child lists stay nil to avoid JSON cycles.
		user.Tickets = append(user.Tickets, &Ticket{
			ID:          *r.ticketID,
			ScreeningId: *r.screeningID,
			SeatId:      *r.seatID,
			UserId:      r.userID,
			Number:      uint32(*r.number),
			Price:       uint32(*r.ticketPrice),
			Status:      TicketStatus(*r.status),
			Seat: &Seat{
				ID:         *r.seatID,
				HallId:     *r.seatHallID,
				SeatNumber: uint32(*r.seatNumber),
				CategoryId: *r.seatCategoryID,
				Category: &SeatCategory{
					ID:                     *r.seatCategoryID,
					Name:                   *r.categoryName,
					PriceMultiplierPercent: uint32(*r.categoryMultiplier),
				},
				Hall: hall,
			},
			Screening: &Screening{
				ID:       *r.screeningID,
				HallId:   *r.screeningHallID,
				ShowId:   *r.showID,
				StartsAt: *r.startsAt,
				EndsAt:   *r.endsAt,
				Hall:     hall,
				Show:     show,
			},
		})
	}

	return users, nil
}

func (r *UserRepository) UsernameExists(ctx context.Context, username string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`

	var exists bool
	err := r.db.QueryRow(ctx, query, username).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}
