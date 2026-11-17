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
	var dbCapacity int64 // scan INTEGER into int64, then convert
	err := pool.QueryRow(ctx, query, name, capacity).Scan(&hall.ID, &hall.Name, &dbCapacity)
	if err != nil {
		return nil, err
	}
	hall.Capacity = uint32(dbCapacity)

	return &hall, nil
}

func GetHallById(ctx context.Context, pool *pgxpool.Pool, id int64) (*Hall, error) {
	query := `SELECT id, name, capacity FROM halls WHERE id = $1`

	var hall Hall
	var capacity int64 // scan INTEGER into int64, then convert
	err := pool.QueryRow(ctx, query, id).Scan(&hall.ID, &hall.Name, &capacity)
	if err != nil {
		return nil, err
	}
	hall.Capacity = uint32(capacity)

	return &hall, nil
}
