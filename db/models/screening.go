package models

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Screening is one show playing in one hall during one time slot.
//
// This is the entity that makes the relation correct:
//
//	hall ──< screenings >── show
//
// A hall can have many screenings on the same day (different shows and/or
// different times), but the database exclusion constraint guarantees that two
// screenings in the same hall never overlap.
type Screening struct {
	ID       int64     `db:"id"        json:"id"        binding:"omitempty,min=1"`
	HallId   int64     `db:"hall_id"   json:"hall_id"   binding:"required,min=1"`
	ShowId   int64     `db:"show_id"   json:"show_id"   binding:"required,min=1"`
	StartsAt time.Time `db:"starts_at" json:"starts_at" binding:"required"`
	EndsAt   time.Time `db:"ends_at"   json:"ends_at"   binding:"required"`

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
	Show *Show `db:"-" json:"show,omitempty" binding:"-"`
}

type ScreeningReq struct {
	HallId   int64     `db:"hall_id"   json:"hall_id"   binding:"required,min=1"`
	ShowId   int64     `db:"show_id"   json:"show_id"   binding:"required,min=1"`
	StartsAt time.Time `db:"starts_at" json:"starts_at" binding:"required"`
	EndsAt   time.Time `db:"ends_at"   json:"ends_at"   binding:"required"`
}

type ScreeningRepository struct {
	db *pgxpool.Pool
}

func NewScreeningRepository(db *pgxpool.Pool) *ScreeningRepository {
	return &ScreeningRepository{db: db}
}

// Create schedules a show in a hall. The database rejects the insert when the
// time slot overlaps another screening in the same hall.
func (r *ScreeningRepository) Create(ctx context.Context, hallId, showId int64, startsAt, endsAt time.Time) (*Screening, error) {
	query := `
	INSERT INTO screenings (hall_id, show_id, starts_at, ends_at)
	VALUES ($1, $2, $3, $4)
	RETURNING id, hall_id, show_id, starts_at, ends_at`

	var screening Screening
	err := r.db.QueryRow(ctx, query, hallId, showId, startsAt, endsAt).
		Scan(&screening.ID, &screening.HallId, &screening.ShowId, &screening.StartsAt, &screening.EndsAt)
	if err != nil {
		return nil, err
	}

	return &screening, nil
}

// CreateAt schedules a show starting at the given time and calculates EndsAt
// from the show's duration.
func (r *ScreeningRepository) CreateAt(ctx context.Context, hallId, showId int64, startsAt time.Time) (*Screening, error) {
	duration, err := r.ShowDuration(ctx, showId)
	if err != nil {
		return nil, err
	}

	return r.Create(ctx, hallId, showId, startsAt, startsAt.Add(time.Duration(duration)*time.Minute))
}

// ShowDuration returns a show's duration in minutes, or pgx.ErrNoRows when the
// show does not exist.
func (r *ScreeningRepository) ShowDuration(ctx context.Context, showId int64) (int64, error) {
	var duration int64
	if err := r.db.QueryRow(ctx, `SELECT duration_minutes FROM shows WHERE id = $1`, showId).Scan(&duration); err != nil {
		return 0, err
	}
	return duration, nil
}

func (r *ScreeningRepository) GetById(ctx context.Context, id int64) (*Screening, error) {
	query := `SELECT id, hall_id, show_id, starts_at, ends_at FROM screenings WHERE id = $1`

	var screening Screening
	err := r.db.QueryRow(ctx, query, id).
		Scan(&screening.ID, &screening.HallId, &screening.ShowId, &screening.StartsAt, &screening.EndsAt)
	if err != nil {
		return nil, err
	}

	return &screening, nil
}

func (r *ScreeningRepository) GetAll(ctx context.Context) ([]*Screening, error) {
	query := `SELECT id, hall_id, show_id, starts_at, ends_at FROM screenings ORDER BY starts_at, id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query screenings: %w", err)
	}

	screenings, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Screening, error) {
		var screening Screening
		err := row.Scan(
			&screening.ID, &screening.HallId, &screening.ShowId,
			&screening.StartsAt, &screening.EndsAt,
		)
		return &screening, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect screenings: %w", err)
	}

	return screenings, nil
}

// GetByHall returns the schedule of one hall, ordered by start time.
func (r *ScreeningRepository) GetByHall(ctx context.Context, hallId int64) ([]*Screening, error) {
	query := `
	SELECT id, hall_id, show_id, starts_at, ends_at
	FROM screenings
	WHERE hall_id = $1
	ORDER BY starts_at, id`

	rows, err := r.db.Query(ctx, query, hallId)
	if err != nil {
		return nil, fmt.Errorf("query screenings by hall: %w", err)
	}

	screenings, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Screening, error) {
		var screening Screening
		err := row.Scan(
			&screening.ID, &screening.HallId, &screening.ShowId,
			&screening.StartsAt, &screening.EndsAt,
		)
		return &screening, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect screenings by hall: %w", err)
	}

	return screenings, nil
}

// GetByShow returns every screening of one show, ordered by start time.
func (r *ScreeningRepository) GetByShow(ctx context.Context, showId int64) ([]*Screening, error) {
	query := `
	SELECT id, hall_id, show_id, starts_at, ends_at
	FROM screenings
	WHERE show_id = $1
	ORDER BY starts_at, id`

	rows, err := r.db.Query(ctx, query, showId)
	if err != nil {
		return nil, fmt.Errorf("query screenings by show: %w", err)
	}

	screenings, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Screening, error) {
		var screening Screening
		err := row.Scan(
			&screening.ID, &screening.HallId, &screening.ShowId,
			&screening.StartsAt, &screening.EndsAt,
		)
		return &screening, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect screenings by show: %w", err)
	}

	return screenings, nil
}

// GetWithDetails returns the screening with its Hall and Show filled in.
func (r *ScreeningRepository) GetWithDetails(ctx context.Context, id int64) (*Screening, error) {
	query := `
	SELECT
	    sc.id, sc.hall_id, sc.show_id, sc.starts_at, sc.ends_at,
	    h.id, h.name, h.capacity,
	    sh.id, sh.name, sh.duration_minutes, sh.author_name, sh.price
	FROM screenings sc
	JOIN halls h  ON h.id  = sc.hall_id
	JOIN shows sh ON sh.id = sc.show_id
	WHERE sc.id = $1`

	rows, err := r.db.Query(ctx, query, id)
	if err != nil {
		return nil, fmt.Errorf("query screening with details: %w", err)
	}

	screening, err := pgx.CollectOneRow(rows, func(row pgx.CollectableRow) (*Screening, error) {
		return scanScreeningWithDetails(row)
	})
	if err != nil {
		return nil, err
	}

	return screening, nil
}

// GetAllWithDetails returns every screening with its Hall and Show filled in.
func (r *ScreeningRepository) GetAllWithDetails(ctx context.Context) ([]*Screening, error) {
	query := `
	SELECT
	    sc.id, sc.hall_id, sc.show_id, sc.starts_at, sc.ends_at,
	    h.id, h.name, h.capacity,
	    sh.id, sh.name, sh.duration_minutes, sh.author_name, sh.price
	FROM screenings sc
	JOIN halls h  ON h.id  = sc.hall_id
	JOIN shows sh ON sh.id = sc.show_id
	ORDER BY sc.starts_at, sc.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query screenings with details: %w", err)
	}

	screenings, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Screening, error) {
		return scanScreeningWithDetails(row)
	})
	if err != nil {
		return nil, fmt.Errorf("collect screenings with details: %w", err)
	}

	return screenings, nil
}

// scanScreeningWithDetails scans one screening + hall + show row.
func scanScreeningWithDetails(row pgx.CollectableRow) (*Screening, error) {
	var screening Screening
	hall := Hall{}
	show := Show{}

	var dbCapacity, dbPrice int64

	err := row.Scan(
		&screening.ID, &screening.HallId, &screening.ShowId, &screening.StartsAt, &screening.EndsAt,
		&hall.ID, &hall.Name, &dbCapacity,
		&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName, &dbPrice,
	)
	if err != nil {
		return nil, err
	}

	hall.Capacity = uint32(dbCapacity)
	show.Price = uint32(dbPrice)

	// Leave Hall.Screenings and Show.Screenings nil to avoid JSON cycles.
	screening.Hall = &hall
	screening.Show = &show

	return &screening, nil
}
