package models

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Hall struct {
	ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
	Name     string `db:"name"     json:"name"     binding:"required,min=1,max=255"`
	Capacity uint32 `db:"capacity" json:"capacity" binding:"required,min=1"`

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Seats      []*Seat      `db:"-" json:"seats,omitempty"      binding:"-"`
	Screenings []*Screening `db:"-" json:"screenings,omitempty" binding:"-"`
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
	seatNumber *int64
	price      *int64
}

type HallScreening struct {
	hallID          int64
	screeningID     *int64 // NULL when the hall has no screenings
	showID          *int64
	startsAt        *time.Time
	endsAt          *time.Time
	showName        *string
	durationMinutes *int64
	authorName      *string
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

// GetAllWithDetails returns every hall with its Seats and its Screenings.
// Each screening carries its Show, so a hall can answer "which show is on?".
func (r *HallRepository) GetAllWithDetails(ctx context.Context) ([]*Hall, error) {
	halls, err := r.getAllWithSeats(ctx)
	if err != nil {
		return nil, err
	}

	byID := make(map[int64]*Hall, len(halls))
	for _, hall := range halls {
		byID[hall.ID] = hall
	}

	if err := r.attachScreenings(ctx, byID); err != nil {
		return nil, err
	}

	return halls, nil
}

func (r *HallRepository) getAllWithSeats(ctx context.Context) ([]*Hall, error) {
	query := `
	SELECT
	    h.id, h.name, h.capacity,
	    s.id, s.seat_number, s.price
	FROM halls h
	LEFT JOIN seats s ON s.hall_id = h.id
	ORDER BY h.id, s.seat_number, s.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query halls with seats: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (HallSeat, error) {
		var r HallSeat
		err := row.Scan(
			&r.hallID, &r.name, &r.capacity,
			&r.seatID, &r.seatNumber, &r.price,
		)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect halls with seats: %w", err)
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
			hall.Seats = append(hall.Seats, &Seat{
				ID:         *r.seatID,
				HallId:     r.hallID,
				SeatNumber: uint32(*r.seatNumber),
				Price:      uint32(*r.price),
			})
		}
	}

	return halls, nil
}

func (r *HallRepository) attachScreenings(ctx context.Context, byID map[int64]*Hall) error {
	query := `
	SELECT
	    h.id,
	    sc.id, sc.show_id, sc.starts_at, sc.ends_at,
	    sh.name, sh.duration_minutes, sh.author_name
	FROM halls h
	LEFT JOIN screenings sc ON sc.hall_id = h.id
	LEFT JOIN shows sh ON sh.id = sc.show_id
	ORDER BY h.id, sc.starts_at, sc.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return fmt.Errorf("query halls with screenings: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (HallScreening, error) {
		var r HallScreening
		err := row.Scan(
			&r.hallID,
			&r.screeningID, &r.showID, &r.startsAt, &r.endsAt,
			&r.showName, &r.durationMinutes, &r.authorName,
		)
		return r, err
	})
	if err != nil {
		return fmt.Errorf("collect halls with screenings: %w", err)
	}

	for _, r := range raw {
		hall, ok := byID[r.hallID]
		if !ok || r.screeningID == nil {
			continue
		}

		// Screening.Hall and Show.Screenings are left nil to avoid JSON cycles.
		hall.Screenings = append(hall.Screenings, &Screening{
			ID:       *r.screeningID,
			HallId:   r.hallID,
			ShowId:   *r.showID,
			StartsAt: *r.startsAt,
			EndsAt:   *r.endsAt,
			Show: &Show{
				ID:              *r.showID,
				Name:            *r.showName,
				DurationMinutes: *r.durationMinutes,
				AuthorName:      *r.authorName,
			},
		})
	}

	return nil
}
