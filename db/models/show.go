package models

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Show struct {
	ID              int64  `db:"id"               json:"id"               binding:"omitempty,min=1"`
	Name            string `db:"name"             json:"name"             binding:"required,min=1,max=255"`
	DurationMinutes int64  `db:"duration_minutes" json:"duration_minutes" binding:"required,min=1"` // per-minutes
	AuthorName      string `db:"author_name"      json:"author_name"      binding:"required,min=1,max=255"`
	Price           uint32 `db:"price"            json:"price"            binding:"gte=0"` // base price for a normal seat

	// A show reaches a hall through screenings: show → screenings → hall.
	Screenings []*Screening `db:"-" json:"screenings,omitempty" binding:"-"`
}

type ShowReq struct {
	Name            string `db:"name"             json:"name"             binding:"required,min=1,max=255"`
	DurationMinutes int64  `db:"duration_minutes" json:"duration_minutes" binding:"required,min=1"`
	AuthorName      string `db:"author_name"      json:"author_name"      binding:"required,min=1,max=255"`
	Price           uint32 `db:"price"            json:"price"            binding:"gte=0"`
}

type ShowScreening struct {
	showID          int64
	name            string
	durationMinutes int64
	authorName      string
	price           int64
	screeningID     *int64 // NULL when the show has no screenings
	hallID          *int64
	startsAt        *time.Time
	endsAt          *time.Time
	hallName        *string
	capacity        *int64
}

type ShowRepository struct {
	db *pgxpool.Pool
}

func NewShowRepository(db *pgxpool.Pool) *ShowRepository {
	return &ShowRepository{db: db}
}

func (r *ShowRepository) Create(ctx context.Context, name string, durationMinutes int64, authorName string, price uint32) (*Show, error) {
	query := `
	INSERT INTO shows (name, duration_minutes, author_name, price)
	VALUES ($1, $2, $3, $4)
	RETURNING id, name, duration_minutes, author_name, price`

	var show Show
	var dbPrice int64
	err := r.db.QueryRow(ctx, query, name, durationMinutes, authorName, price).
		Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName, &dbPrice)
	if err != nil {
		return nil, err
	}
	show.Price = uint32(dbPrice)

	return &show, nil
}

func (r *ShowRepository) GetById(ctx context.Context, id int64) (*Show, error) {
	query := `SELECT id, name, duration_minutes, author_name, price FROM shows WHERE id = $1`

	var show Show
	var dbPrice int64
	err := r.db.QueryRow(ctx, query, id).
		Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName, &dbPrice)
	if err != nil {
		return nil, err
	}
	show.Price = uint32(dbPrice)

	return &show, nil
}

func (r *ShowRepository) GetAll(ctx context.Context) ([]*Show, error) {
	query := `SELECT id, name, duration_minutes, author_name, price FROM shows ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query shows: %w", err)
	}

	shows, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Show, error) {
		var show Show
		var dbPrice int64
		err := row.Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName, &dbPrice)
		if err != nil {
			return nil, err
		}
		show.Price = uint32(dbPrice)
		return &show, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect shows: %w", err)
	}

	return shows, nil
}

// GetAllWithDetails returns every show with its scheduled Screenings filled
// in. Each screening carries the Hall it plays in.
// Shows without screenings still appear, with an empty Screenings list.
func (r *ShowRepository) GetAllWithDetails(ctx context.Context) ([]*Show, error) {
	query := `
	SELECT
	    sh.id, sh.name, sh.duration_minutes, sh.author_name, sh.price,
	    sc.id, sc.hall_id, sc.starts_at, sc.ends_at,
	    h.name, h.capacity
	FROM shows sh
	LEFT JOIN screenings sc ON sc.show_id = sh.id
	LEFT JOIN halls h ON h.id = sc.hall_id
	ORDER BY sh.id, sc.starts_at, sc.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query shows with details: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ShowScreening, error) {
		var r ShowScreening
		err := row.Scan(
			&r.showID, &r.name, &r.durationMinutes, &r.authorName, &r.price,
			&r.screeningID, &r.hallID, &r.startsAt, &r.endsAt,
			&r.hallName, &r.capacity,
		)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect shows with details: %w", err)
	}

	// Group the flat rows: one show per id, with its screenings attached.
	shows := make([]*Show, 0, len(raw))
	byID := make(map[int64]*Show)
	for _, r := range raw {
		show, ok := byID[r.showID]
		if !ok {
			show = &Show{
				ID:              r.showID,
				Name:            r.name,
				DurationMinutes: r.durationMinutes,
				AuthorName:      r.authorName,
				Price:           uint32(r.price),
			}
			byID[r.showID] = show
			shows = append(shows, show)
		}
		// Screening.Show and Hall.Screenings are left nil on purpose to avoid
		// a Show -> Screenings -> Show cycle in JSON.
		if r.screeningID != nil {
			show.Screenings = append(show.Screenings, &Screening{
				ID:       *r.screeningID,
				HallId:   *r.hallID,
				ShowId:   r.showID,
				StartsAt: *r.startsAt,
				EndsAt:   *r.endsAt,
				Hall:     &Hall{ID: *r.hallID, Name: *r.hallName, Capacity: uint32(*r.capacity)},
			})
		}
	}

	return shows, nil
}
