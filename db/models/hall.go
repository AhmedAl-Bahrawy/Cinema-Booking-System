package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Hall struct {
	ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
	Name     string `db:"name"     json:"name"     binding:"required,min=1,max=255"`
	Capacity uint32 `db:"capacity" json:"capacity" binding:"required,min=1"`

	Seats []*Seat `db:"-" json:"seats,omitempty" binding:"-"`
}

type HallReq struct {
	Name     string `db:"name"     json:"name"     binding:"required,min=1,max=255"`
	Capacity uint32 `db:"capacity" json:"capacity" binding:"required,min=1"`
}

type HallSeat struct {
	hallID     int64
	name       string
	capacity   int64
	seatID     *int64 // NULL when the hall has no seats
	seatUserId *int64
	seatNumber *int64
	status     *int16
	price      *int64
}

type HallRepository struct {
	db *pgxpool.Pool
}

func NewHallRepository(db *pgxpool.Pool) *HallRepository {
	return &HallRepository{db: db}
}

func (r *HallRepository) Create(ctx context.Context, name string, capacity uint32) (*Hall, error) {
	query := `INSERT INTO halls (name, capacity) VALUES ($1, $2) RETURNING id, name, capacity`

	var hall Hall
	var dbCapacity int64 // scan INTEGER into int64, then convert
	err := r.db.QueryRow(ctx, query, name, capacity).Scan(&hall.ID, &hall.Name, &dbCapacity)
	if err != nil {
		return nil, err
	}
	hall.Capacity = uint32(dbCapacity)

	return &hall, nil
}

func (r *HallRepository) GetById(ctx context.Context, id int64) (*Hall, error) {
	query := `SELECT id, name, capacity FROM halls WHERE id = $1`

	var hall Hall
	var capacity int64 // scan INTEGER into int64, then convert
	err := r.db.QueryRow(ctx, query, id).Scan(&hall.ID, &hall.Name, &capacity)
	if err != nil {
		return nil, err
	}
	hall.Capacity = uint32(capacity)

	return &hall, nil
}

func (r *HallRepository) GetAll(ctx context.Context) ([]*Hall, error) {
	query := `SELECT id, name, capacity FROM halls ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query halls: %w", err)
	}

	halls, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Hall, error) {
		var hall Hall
		var dbCapacity int64
		err := row.Scan(&hall.ID, &hall.Name, &dbCapacity)
		hall.Capacity = uint32(dbCapacity)
		return &hall, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect halls: %w", err)
	}

	return halls, nil
}

func (r *HallRepository) GetAllWithDetails(ctx context.Context) ([]*Hall, error) {
	query := `
	SELECT
	    h.id, h.name, h.capacity,
	    s.id, s.user_id, s.seat_number, s.status, s.price
	FROM halls h
	LEFT JOIN seats s ON s.hall_id = h.id
	ORDER BY h.id, s.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query halls with details: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (HallSeat, error) {
		var r HallSeat
		err := row.Scan(
			&r.hallID, &r.name, &r.capacity,
			&r.seatID, &r.seatUserId, &r.seatNumber, &r.status, &r.price,
		)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect halls with details: %w", err)
	}

	// Group the flat rows: one hall per id, with its seats attached.
	halls := make([]*Hall, 0, len(raw))
	byID := make(map[int64]*Hall)
	for _, r := range raw {
		hall, ok := byID[r.hallID]
		if !ok {
			hall = &Hall{ID: r.hallID, Name: r.name, Capacity: uint32(r.capacity)}
			byID[r.hallID] = hall
			halls = append(halls, hall)
		}
		// Seat.Hall is left nil on purpose to avoid a Hall -> Seats -> Hall cycle in JSON.
		if r.seatID != nil {
			seat := &Seat{
				ID:         *r.seatID,
				HallId:     r.hallID,
				SeatNumber: uint32(*r.seatNumber),
				Status:     BookingStatus(*r.status),
				Price:      uint32(*r.price),
			}
			if r.seatUserId != nil {
				seat.UserId = *r.seatUserId
			}
			hall.Seats = append(hall.Seats, seat)
		}
	}

	return halls, nil
}
