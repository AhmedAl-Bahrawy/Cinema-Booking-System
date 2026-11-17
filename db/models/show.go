package models

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Show struct {
	ID              int64  `db:"id"               json:"id"               binding:"omitempty,min=1"`
	Name            string `db:"name"             json:"name"             binding:"required,min=1,max=255"`
	DurationMinutes int64  `db:"duration_minutes" json:"duration_minutes" binding:"required,min=1"` // per-minutes
	AuthorName      string `db:"author_name"      json:"author_name"      binding:"required,min=1,max=255"`
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
