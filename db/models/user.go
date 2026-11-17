package models

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

type User struct {
	ID       int64  `db:"id"       json:"id"       binding:"omitempty,min=1"`
	Username string `db:"username" json:"username" binding:"required,min=3,max=100"`
	FullName string `db:"full_name" json:"full_name" binding:"required,min=3,max=255"`
}

func CreateUser(ctx context.Context, pool *pgxpool.Pool, username string, fullName string) (*User, error) {
	query := `INSERT INTO users (username, full_name) VALUES ($1, $2) RETURNING id, username, full_name`

	var user User
	err := pool.QueryRow(ctx, query, username, fullName).Scan(&user.ID, &user.Username, &user.FullName)
	if err != nil {
		return nil, err
	}

	return &user, nil
}

func GetUserById(ctx context.Context, pool *pgxpool.Pool, id int64) (*User, error) {
	query := `SELECT id, username, full_name FROM users WHERE id = $1`

	var user User
	err := pool.QueryRow(ctx, query, id).Scan(&user.ID, &user.Username, &user.FullName)
	if err != nil {
		return nil, err
	}

	return &user, nil
}
