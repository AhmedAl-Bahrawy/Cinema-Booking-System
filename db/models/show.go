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

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Tickets []*Ticket `db:"-" json:"tickets,omitempty" binding:"-"`
}

func CreateShow(ctx context.Context, pool *pgxpool.Pool, name string, durationMinutes int64, authorName string) (*Show, error) {
	query := `INSERT INTO shows (name, duration_minutes, author_name) VALUES ($1, $2, $3) RETURNING id, name, duration_minutes, author_name`

	var show Show
	err := pool.QueryRow(ctx, query, name, durationMinutes, authorName).Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName)
	if err != nil {
		return nil, err
	}

	return &show, nil
}

func GetShowById(ctx context.Context, pool *pgxpool.Pool, id int64) (*Show, error) {
	query := `SELECT id, name, duration_minutes, author_name FROM shows WHERE id = $1`

	var show Show
	err := pool.QueryRow(ctx, query, id).Scan(&show.ID, &show.Name, &show.DurationMinutes, &show.AuthorName)
	if err != nil {
		return nil, err
	}

	return &show, nil
}

func GetAllShows(ctx context.Context, pool *pgxpool.Pool) ([]*Show, error) {
	query := `SELECT id, name, duration_minutes, author_name FROM shows ORDER BY id`

	rows, err := pool.Query(ctx, query)
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

// showTicketRow is one raw row of the shows + tickets LEFT JOIN.
type showTicketRow struct {
	showID          int64
	name            string
	durationMinutes int64
	authorName      string
	ticketID        *int64 // NULL when the show has no tickets
	seatID          *int64
	hallID          *int64
}

// GetAllShowsWithDetails returns every show with its tickets filled in.
// Shows without tickets still appear, with an empty Tickets list.
func GetAllShowsWithDetails(ctx context.Context, pool *pgxpool.Pool) ([]*Show, error) {
	query := `
	SELECT
	    sh.id, sh.name, sh.duration_minutes, sh.author_name,
	    t.id, t.seat_id, t.hall_id
	FROM shows sh
	LEFT JOIN tickets t ON t.show_id = sh.id
	ORDER BY sh.id, t.id`

	rows, err := pool.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query shows with details: %w", err)
	}

	raw, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (showTicketRow, error) {
		var r showTicketRow
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
