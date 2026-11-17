package models

import (
	"context"

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

	// Joined relations: filled manually after a JOIN query, never scanned directly.
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

	var seat Seat
	hall := Hall{}

	var dbUserId, dbJoinUserId *int64         // user_id + u.id, NULL for a free seat
	var dbUsername, dbFullName *string        // NULL for a free seat
	var dbSeatNumber, dbPrice, dbCapacity int64

	err := pool.QueryRow(ctx, query, id).Scan(
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
