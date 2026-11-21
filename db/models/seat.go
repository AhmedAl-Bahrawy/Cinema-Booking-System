package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Seat is a physical seat inside a hall.
//
// It deliberately has no user and no booking status: a seat is not "owned" by
// anybody. Who sits in it depends on the screening, and that relationship lives
// on Ticket. This lets the same seat be used by different users for different
// screenings, even on the same day.
type Seat struct {
	ID         int64  `db:"id"          json:"id"          binding:"omitempty,min=1"`
	HallId     int64  `db:"hall_id"     json:"hall_id"     binding:"required,min=1"`
	SeatNumber uint32 `db:"seat_number" json:"seat_number" binding:"required,min=1"`
	Price      uint32 `db:"price"       json:"price"       binding:"gte=0"` // 0 allowed (free seat)

	// Joined relation: filled manually after a JOIN query, never scanned directly.
	Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
}

type SeatReq struct {
	HallId     int64  `db:"hall_id"     json:"hall_id"     binding:"required,min=1"`
	SeatNumber uint32 `db:"seat_number" json:"seat_number" binding:"required,min=1"`
	Price      uint32 `db:"price"       json:"price"       binding:"gte=0"` // 0 allowed (free seat)
}

type SeatRepository struct {
	db *pgxpool.Pool
}

func NewSeatRepository(db *pgxpool.Pool) *SeatRepository {
	return &SeatRepository{db: db}
}

func (r *SeatRepository) Create(ctx context.Context, hallId int64, seatNumber uint32, price uint32) (*Seat, error) {
	query := `INSERT INTO seats (hall_id, seat_number, price) VALUES ($1, $2, $3) RETURNING id, hall_id, seat_number, price`

	var seat Seat
	var dbSeatNumber, dbPrice int64
	err := r.db.QueryRow(ctx, query, hallId, seatNumber, price).
		Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &dbPrice)
	if err != nil {
		return nil, err
	}
	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)

	return &seat, nil
}

func (r *SeatRepository) GetById(ctx context.Context, id int64) (*Seat, error) {
	query := `SELECT id, hall_id, seat_number, price FROM seats WHERE id = $1`

	var seat Seat
	var dbSeatNumber, dbPrice int64
	err := r.db.QueryRow(ctx, query, id).
		Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &dbPrice)
	if err != nil {
		return nil, err
	}
	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)

	return &seat, nil
}

func (r *SeatRepository) GetAll(ctx context.Context) ([]*Seat, error) {
	query := `SELECT id, hall_id, seat_number, price FROM seats ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query seats: %w", err)
	}

	seats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Seat, error) {
		var seat Seat
		var dbSeatNumber, dbPrice int64

		err := row.Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &dbPrice)
		if err != nil {
			return nil, err
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

// GetWithDetails returns the seat with the Hall it physically belongs to.
func (r *SeatRepository) GetWithDetails(ctx context.Context, id int64) (*Seat, error) {
	query := `
	SELECT
	    s.id, s.hall_id, s.seat_number, s.price,
	    h.id, h.name, h.capacity
	FROM seats s
	JOIN halls h ON h.id = s.hall_id
	WHERE s.id = $1`

	rows, err := r.db.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query seat with details: %w", err)
	}

	seat, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (*Seat, error) {
		return scanSeatWithHall(row)
	})
	if err != nil {
		return nil, err
	}

	return seat, nil
}

// GetAllWithDetails returns every seat with the Hall it belongs to.
func (r *SeatRepository) GetAllWithDetails(ctx context.Context) ([]*Seat, error) {
	query := `
	SELECT
	    s.id, s.hall_id, s.seat_number, s.price,
	    h.id, h.name, h.capacity
	FROM seats s
	JOIN halls h ON h.id = s.hall_id
	ORDER BY s.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query seats with details: %w", err)
	}

	seats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Seat, error) {
		return scanSeatWithHall(row)
	})
	if err != nil {
		return nil, fmt.Errorf("collect seats with details: %w", err)
	}

	return seats, nil
}

// GetByHall returns the physical seats of one hall (plain rows, no JOINs).
func (r *SeatRepository) GetByHall(ctx context.Context, hallId int64) ([]*Seat, error) {
	query := `SELECT id, hall_id, seat_number, price FROM seats WHERE hall_id = $1 ORDER BY seat_number`

	rows, err := r.db.Query(ctx, query, hallId)
	if err != nil {
		return nil, fmt.Errorf("query seats by hall: %w", err)
	}

	seats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Seat, error) {
		var seat Seat
		var dbSeatNumber, dbPrice int64

		err := row.Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &dbPrice)
		if err != nil {
			return nil, err
		}
		seat.SeatNumber = uint32(dbSeatNumber)
		seat.Price = uint32(dbPrice)

		return &seat, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect seats by hall: %w", err)
	}

	return seats, nil
}

// scanSeatWithHall scans one seat + hall row.
func scanSeatWithHall(row pgx.CollectableRow) (*Seat, error) {
	var seat Seat
	hall := Hall{}

	var dbSeatNumber, dbPrice, dbCapacity int64

	err := row.Scan(
		&seat.ID, &seat.HallId, &dbSeatNumber, &dbPrice,
		&hall.ID, &hall.Name, &dbCapacity,
	)
	if err != nil {
		return nil, err
	}

	seat.SeatNumber = uint32(dbSeatNumber)
	seat.Price = uint32(dbPrice)
	hall.Capacity = uint32(dbCapacity)
	seat.Hall = &hall

	return &seat, nil
}
