package models

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Hall struct {
	ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
	Name     string `db:"name"     json:"name"     binding:"required,min=1,max=255"`
	Capacity uint32 `db:"capacity" json:"capacity" binding:"required,min=1"`
}

func CreateHall(ctx context.Context, pool *pgxpool.Pool, name string, capacity uint32) (*Hall, error) {
	query := `INSERT INTO halls (name, capacity) VALUES ($1, $2) RETURNING id, name, capacity`

	var hall Hall
	err := pool.QueryRow(ctx, query, name, capacity).Scan(&hall.ID, &hall.Name, &hall.Capacity)
	if err != nil {
		return nil, err
	}

	return &hall, nil
}
