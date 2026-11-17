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

func CreateUser(ctx context.Context, pool *pgxpool.Pool, username string, fullName string) {

}
