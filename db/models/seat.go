package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Free (0) | Holding (1) | Booked (2)
type BookingStatus int8

const (
	Free    BookingStatus = iota // 0
	Holding                      // 1
	Booked                       // 2
)

type Seat struct {
	ID         int64         `db:"id"          json:"id"          binding:"omitempty,min=1"`
	UserId     int64         `db:"user_id"     json:"user_id"     binding:"omitempty,min=1"` // null until the seat is held/booked
	HallId     int64         `db:"hall_id"     json:"hall_id"     binding:"required,min=1"`
	SeatNumber uint32        `db:"seat_number" json:"seat_number" binding:"required,min=1"`
	Status     BookingStatus `db:"status"      json:"status"      binding:"oneof=0 1 2"`
	Price      uint32        `db:"price"       json:"price"       binding:"gte=0"` // 0 allowed (free seat)

	User *User `db:"-" json:"user,omitempty" binding:"-"`
	Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
}

// CreateSeat inserts a seat. Pass userId = 0 for a free seat (no user yet);
// any other value links the seat to that user (holding/booked).
func CreateSeat(ctx context.Context, pool *pgxpool.Pool, userId, hallId int64, seatNumber uint32, status BookingStatus, price uint32) (*Seat, error) {
	query := `INSERT INTO seats (user_id, hall_id, seat_number, status, price) VALUES ($1, $2, $3, $4, $5) RETURNING id, user_id, hall_id, seat_number, status, price`

	// user_id is NULLABLE: 0 means "no user", which must be stored as NULL.
	var userIdArg any
	if userId != 0 {
		userIdArg = userId
	}

	var seat Seat
	var dbUserId *int64 // NULL until the seat is held/booked
	var dbSeatNumber, dbPrice int64
	err := pool.QueryRow(ctx, query, userIdArg, hallId, seatNumber, status, price).
		Scan(&seat.ID, &dbUserId, &seat.HallId, &dbSeatNumber, &seat.Status, &dbPrice)
	if err != nil {
		return nil, err
	}
	if dbUserId != nil {
		seat.UserId = *dbUserId
	}
	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)

	return &seat, nil
}

func GetSeatById(ctx context.Context, pool *pgxpool.Pool, id int64) (*Seat, error) {
	query := `SELECT id, user_id, hall_id, seat_number, status, price FROM seats WHERE id = $1`

	var seat Seat
	var dbUserId *int64 // NULL until the seat is held/booked
	var dbSeatNumber, dbPrice int64
	err := pool.QueryRow(ctx, query, id).
		Scan(&seat.ID, &dbUserId, &seat.HallId, &dbSeatNumber, &seat.Status, &dbPrice)
	if err != nil {
		return nil, err
	}
	if dbUserId != nil {
		seat.UserId = *dbUserId
	}
	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)

	return &seat, nil
}

func GetAllSeats(ctx context.Context, pool *pgxpool.Pool) ([]*Seat, error) {
	query := `SELECT id, user_id, hall_id, seat_number, status, price FROM seats ORDER BY id`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query seats: %w", err)
	}

	seats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Seat, error) {
		var seat Seat
		var dbUserId *int64 // NULL until the seat is held/booked
		var dbSeatNumber, dbPrice int64

		err := row.Scan(&seat.ID, &dbUserId, &seat.HallId, &dbSeatNumber, &seat.Status, &dbPrice)
		if err != nil {
			return nil, err
		}
		if dbUserId != nil {
			seat.UserId = *dbUserId
		}
		seat.SeatNumber = uint32(dbSeatNumber)
		seat.Price = uint32(dbPrice)

		return &seat, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect seats: %w", err)
	}

	return seats, nil
}

// GetSeatWithDetails returns the seat with its Hall (and User, if any) filled
// in from a JOIN query. A free seat has no user, so User stays nil for it.
func GetSeatWithDetails(ctx context.Context, pool *pgxpool.Pool, id int64) (*Seat, error) {
	query := `
	SELECT
	    s.id, s.user_id, s.hall_id, s.seat_number, s.status, s.price,
	    h.id, h.name, h.capacity,
	    u.id, u.username, u.full_name
	FROM seats s
	JOIN halls h ON h.id = s.hall_id
	LEFT JOIN users u ON u.id = s.user_id
	WHERE s.id = $1`

	rows, err := pool.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query seat with details: %w", err)
	}

	seat, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (*Seat, error) {
		return scanSeatWithDetails(row)
	})
	if err != nil {
		return nil, err
	}

	return seat, nil
}

// GetAllSeatsWithDetails returns every seat with its Hall (and User, if any)
// filled in from a JOIN query. Free seats have no user, so User stays nil.
func GetAllSeatsWithDetails(ctx context.Context, pool *pgxpool.Pool) ([]*Seat, error) {
	query := `
	SELECT
	    s.id, s.user_id, s.hall_id, s.seat_number, s.status, s.price,
	    h.id, h.name, h.capacity,
	    u.id, u.username, u.full_name
	FROM seats s
	JOIN halls h ON h.id = s.hall_id
	LEFT JOIN users u ON u.id = s.user_id
	ORDER BY s.id`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query seats with details: %w", err)
	}

	seats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Seat, error) {
		return scanSeatWithDetails(row)
	})
	if err != nil {
		return nil, fmt.Errorf("collect seats with details: %w", err)
	}

	return seats, nil
}

// scanSeatWithDetails scans one seat + hall + (optional) user row.
func scanSeatWithDetails(row pgx.CollectableRow) (*Seat, error) {
	var seat Seat
	hall := Hall{}

	var dbUserId, dbJoinUserId *int64  // NULL for a free seat
	var dbUsername, dbFullName *string // NULL for a free seat
	var dbSeatNumber, dbPrice, dbCapacity int64

	err := row.Scan(
		&seat.ID, &dbUserId, &seat.HallId, &dbSeatNumber, &seat.Status, &dbPrice,
		&hall.ID, &hall.Name, &dbCapacity,
		&dbJoinUserId, &dbUsername, &dbFullName,
	)
	if err != nil {
		return nil, err
	}

	if dbUserId != nil {
		seat.UserId = *dbUserId
		seat.User = &User{ID: *dbUserId, Username: *dbUsername, FullName: *dbFullName}
	}
	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)
	hall.Capacity = uint32(dbCapacity)
	seat.Hall = &hall

	return &seat, nil
}
