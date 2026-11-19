package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Show struct {
	ID              int64  `db:"id"               json:"id"               binding:"omitempty,min=1"`
	Name            string `db:"name"             json:"name"             binding:"required,min=1,max=255"`
	DurationMinutes int64  `db:"duration_minutes" json:"duration_minutes" binding:"required,min=1"` // per-minutes
	AuthorName      string `db:"author_name"      json:"author_name"      binding:"required,min=1,max=255"`

	Tickets []*Ticket `db:"-" json:"tickets,omitempty" binding:"-"`
}

type ShowTicket struct {
	showID          int64
	name            string
	durationMinutes int64
	authorName      string
	ticketID        *int64 // NULL when the show has no tickets
	seatID          *int64
	hallID          *int64
}

type ShowRepository struct {
	db *pgxpool.Pool
}

func NewShowRepository(db *pgxpool.Pool) *ShowRepository {
	return &ShowRepository{db: db}
}

func (r *ShowRepository) Create(ctx context.Context, name string, durationMinutes int64, authorName string) (*Show, error) {
	query := `INSERT INTO shows (name, duration_minutes, author_name) VALUES ($1, $2, $3) RETURNING id, name, duration_minutes, author_name`

	var show Show
	err := r.db.QueryRow(ctx, query, name, durationMinutes, authorName).Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName)
	if err != nil {
		return nil, err
	}

	return &show, nil
}

func (r *ShowRepository) GetById(ctx context.Context, id int64) (*Show, error) {
	query := `SELECT id, name, duration_minutes, author_name FROM shows WHERE id = $1`

	var show Show
	err := r.db.QueryRow(ctx, query, id).Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName)
	if err != nil {
		return nil, err
	}

	return &show, nil
}

func (r *ShowRepository) GetAll(ctx context.Context) ([]*Show, error) {
	query := `SELECT id, name, duration_minutes, author_name FROM shows ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query shows: %w", err)
	}

	shows, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*Show, error) {
		var show Show
		err := row.Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName)
		return &show, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect shows: %w", err)
	}

	return shows, nil
}

// GetAllWithDetails returns every show with its tickets filled in.
// Shows without tickets still appear, with an empty Tickets list.
func (r *ShowRepository) GetAllWithDetails(ctx context.Context) ([]*Show, error) {
	query := `
	SELECT
	    sh.id, sh.name, sh.duration_minutes, sh.author_name,
	    t.id, t.seat_id, t.hall_id
	FROM shows sh
	LEFT JOIN tickets t ON t.show_id = sh.id
	ORDER BY sh.id, t.id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query shows with details: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (ShowTicket, error) {
		var r ShowTicket
		err := row.Scan(
			&r.showID, &r.name, &r.durationMinutes, &r.authorName,
			&r.ticketID, &r.seatID, &r.hallID,
		)
		return r, err
	})
	if err != nil {
		return nil, fmt.Errorf("collect shows with details: %w", err)
	}

	// Group the flat rows: one show per id, with its tickets attached.
	shows := make([]*Show, 0, len(raw))
	byID := make(map[int64]*Show)
	for _, r := range raw {
		show, ok := byID[r.showID]
		if !ok {
			show = &Show{ID: r.showID, Name: r.name, DurationMinutes: r.durationMinutes, AuthorName: r.authorName}
			byID[r.showID] = show
			shows = append(shows, show)
		}
		// Ticket.Show is left nil on purpose to avoid a Show -> Tickets -> Show cycle in JSON.
		if r.ticketID != nil {
			show.Tickets = append(show.Tickets, &Ticket{
				ID:     *r.ticketID,
				SeatId: *r.seatID,
				HallId: *r.hallID,
				ShowId: r.showID,
			})
		}
	}

	return shows, nil
}
