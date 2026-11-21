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
// on Ticket. The seat category (normal / premium / VIP) is what determines the
// ticket price multiplier.
type Seat struct {
	ID         int64  `db:"id"          json:"id"          binding:"omitempty,min=1"`
	HallId     int64  `db:"hall_id"     json:"hall_id"     binding:"required,min=1"`
	SeatNumber uint32 `db:"seat_number" json:"seat_number" binding:"required,min=1"`
	CategoryId int64  `db:"category_id" json:"category_id" binding:"required,min=1"`

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Category *SeatCategory `db:"-" json:"category,omitempty" binding:"-"`
	Hall     *Hall         `db:"-" json:"hall,omitempty"     binding:"-"`
}

type SeatReq struct {
	HallId     int64  `db:"hall_id"     json:"hall_id"     binding:"required,min=1"`
	SeatNumber uint32 `db:"seat_number" json:"seat_number" binding:"required,min=1"`
	CategoryId int64  `db:"category_id" json:"category_id" binding:"omitempty,min=1"`
}

type SeatRepository struct {
	db *pgxpool.Pool
}

func NewSeatRepository(db *pgxpool.Pool) *SeatRepository {
	return &SeatRepository{db: db}
}

func (r *SeatRepository) Create(ctx context.Context, hallId int64, seatNumber uint32, categoryId int64) (*Seat, error) {
	// 0 means "use the normal category".
	if categoryId == 0 {
		categoryId = 1
	}

	query := `
	INSERT INTO seats (hall_id, seat_number, category_id)
	VALUES ($1, $2, $3)
	RETURNING id, hall_id, seat_number, category_id`

	var seat Seat
	var dbSeatNumber int64
	err := r.db.QueryRow(ctx, query, hallId, seatNumber, categoryId).
		Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &seat.CategoryId)
	if err != nil {
		return nil, err
	}
	seat.SeatNumber = uint32(dbSeatNumber)

	return &seat, nil
}

func (r *SeatRepository) GetById(ctx context.Context, id int64) (*Seat, error) {
	query := `SELECT id, hall_id, seat_number, category_id FROM seats WHERE id = $1`

	var seat Seat
	var dbSeatNumber int64
	err := r.db.QueryRow(ctx, query, id).
		Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &seat.CategoryId)
	if err != nil {
		return nil, err
	}
	seat.SeatNumber = uint32(dbSeatNumber)

	return &seat, nil
}

func (r *SeatRepository) GetAll(ctx context.Context) ([]*Seat, error) {
	query := `SELECT id, hall_id, seat_number, category_id FROM seats ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query seats: %w", err)
	}

	seats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Seat, error) {
		var seat Seat
		var dbSeatNumber int64

		err := row.Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &seat.CategoryId)
		if err != nil {
			return nil, err
		}
		seat.SeatNumber = uint32(dbSeatNumber)

		return &seat, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect seats: %w", err)
	}

	return seats, nil
}

// GetWithDetails returns the seat with the Hall it physically belongs to and
// its SeatCategory.
func (r *SeatRepository) GetWithDetails(ctx context.Context, id int64) (*Seat, error) {
	query := `
	SELECT
	    s.id, s.hall_id, s.seat_number, s.category_id,
	    c.id, c.name, c.price_multiplier_percent,
	    h.id, h.name, h.capacity
	FROM seats s
	JOIN seat_categories c ON c.id = s.category_id
	JOIN halls h ON h.id = s.hall_id
	WHERE s.id = $1`

	rows, err := r.db.Query(ctx, query, id)
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

// GetAllWithDetails returns every seat with its Hall and SeatCategory.
func (r *SeatRepository) GetAllWithDetails(ctx context.Context) ([]*Seat, error) {
	query := `
	SELECT
	    s.id, s.hall_id, s.seat_number, s.category_id,
	    c.id, c.name, c.price_multiplier_percent,
	    h.id, h.name, h.capacity
	FROM seats s
	JOIN seat_categories c ON c.id = s.category_id
	JOIN halls h ON h.id = s.hall_id
	ORDER BY s.id`

	rows, err := r.db.Query(ctx, query)
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

// GetByHall returns the physical seats of one hall (plain rows, no JOINs).
func (r *SeatRepository) GetByHall(ctx context.Context, hallId int64) ([]*Seat, error) {
	query := `SELECT id, hall_id, seat_number, category_id FROM seats WHERE hall_id = $1 ORDER BY seat_number`

	rows, err := r.db.Query(ctx, query, hallId)
	if err != nil {
		return nil, fmt.Errorf("query seats by hall: %w", err)
	}

	seats, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Seat, error) {
		var seat Seat
		var dbSeatNumber int64

		err := row.Scan(&seat.ID, &seat.HallId, &dbSeatNumber, &seat.CategoryId)
		if err != nil {
			return nil, err
		}
		seat.SeatNumber = uint32(dbSeatNumber)

		return &seat, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect seats by hall: %w", err)
	}

	return seats, nil
}

// scanSeatWithDetails scans one seat + category + hall row.
func scanSeatWithDetails(row pgx.CollectableRow) (*Seat, error) {
	var seat Seat
	category := SeatCategory{}
	hall := Hall{}

	var dbSeatNumber, dbMultiplier, dbCapacity int64

	err := row.Scan(
		&seat.ID, &seat.HallId, &dbSeatNumber, &seat.CategoryId,
		&category.ID, &category.Name, &dbMultiplier,
		&hall.ID, &hall.Name, &dbCapacity,
	)
	if err != nil {
		return nil, err
	}

	seat.SeatNumber = uint32(dbSeatNumber)
	category.PriceMultiplierPercent = uint32(dbMultiplier)
	hall.Capacity = uint32(dbCapacity)

	seat.Category = &category
	seat.Hall = &hall

	return &seat, nil
}
