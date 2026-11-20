package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
	Username string `db:"username" json:"username" binding:"required,min=3,max=100"`
	FullName string `db:"full_name" json:"full_name" binding:"required,min=3,max=255"`

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Seats []*Seat `db:"-" json:"seats,omitempty" binding:"-"`
}

type UserReq struct {
	Username string `db:"username" json:"username" binding:"required,min=3,max=100"`
	FullName string `db:"full_name" json:"full_name" binding:"required,min=3,max=255"`
}

type UserSeat struct {
	userID     int64
	username   string
	fullName   string
	seatID     *int64 // NULL when the user has no seats
	hallId     *int64
	seatNumber *int64
	status     *int16
	price      *int64
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

// GetAllWithDetails returns every user with their seats filled in.
// Users without seats still appear, with an empty Seats list.
func (r *UserRepository) GetAllWithDetails(ctx context.Context) ([]*User, error) {
	query := `
	SELECT
	    u.id, u.username, u.full_name,
	    s.id, s.hall_id, s.seat_number, s.status, s.price
	FROM users u
	LEFT JOIN seats s ON s.user_id = u.id
	ORDER BY u.id, s.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query users with details: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (UserSeat, error) {
		var r UserSeat
		err := row.Scan(
			&r.userID, &r.username, &r.fullName,
			&r.seatID, &r.hallId, &r.seatNumber, &r.status, &r.price,
		)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect users with details: %w", err)
	}

	// Group the flat rows: one user per id, with their seats attached.
	users := make([]*User, 0, len(raw))
	byID := make(map[int64]*User)
	for _, r := range raw {
		user, ok := byID[r.userID]
		if !ok {
			user = &User{ID: r.userID, Username: r.username, FullName: r.fullName}
			byID[r.userID] = user
			users = append(users, user)
		}
		if r.seatID != nil {
			user.Seats = append(user.Seats, &Seat{
				ID:         *r.seatID,
				UserId:     r.userID,
				HallId:     *r.hallId,
				SeatNumber: uint32(*r.seatNumber),
				Status:     BookingStatus(*r.status),
				Price:      uint32(*r.price),
			})
		}
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
