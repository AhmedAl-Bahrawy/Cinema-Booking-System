package models

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// SeatCategory is the "seat status" that controls the ticket price:
// normal, premium, VIP, or any custom level between them.
//
// PriceMultiplierPercent is a percentage of the show price:
//
//	100 -> 1.00x (normal)
//	150 -> 1.50x (premium)
//	200 -> 2.00x (VIP)
type SeatCategory struct {
	ID                     int64  `db:"id"                       json:"id"                       binding:"omitempty,min=1"`
	Name                   string `db:"name"                     json:"name"                     binding:"required,min=1,max=50"`
	PriceMultiplierPercent uint32 `db:"price_multiplier_percent" json:"price_multiplier_percent" binding:"required,min=1"`
}

type SeatCategoryReq struct {
	Name                   string `db:"name"                     json:"name"                     binding:"required,min=1,max=50"`
	PriceMultiplierPercent uint32 `db:"price_multiplier_percent" json:"price_multiplier_percent" binding:"required,min=1"`
}

type SeatCategoryRepository struct {
	db *pgxpool.Pool
}

func NewSeatCategoryRepository(db *pgxpool.Pool) *SeatCategoryRepository {
	return &SeatCategoryRepository{db: db}
}

func (r *SeatCategoryRepository) Create(ctx context.Context, name string, priceMultiplierPercent uint32) (*SeatCategory, error) {
	query := `
	INSERT INTO seat_categories (name, price_multiplier_percent)
	VALUES ($1, $2)
	RETURNING id, name, price_multiplier_percent`

	var category SeatCategory
	var dbMultiplier int64
	err := r.db.QueryRow(ctx, query, name, priceMultiplierPercent).
		Scan(&category.ID, &category.Name, &dbMultiplier)
	if err != nil {
		return nil, err
	}
	category.PriceMultiplierPercent = uint32(dbMultiplier)

	return &category, nil
}

func (r *SeatCategoryRepository) GetById(ctx context.Context, id int64) (*SeatCategory, error) {
	query := `SELECT id, name, price_multiplier_percent FROM seat_categories WHERE id = $1`

	var category SeatCategory
	var dbMultiplier int64
	err := r.db.QueryRow(ctx, query, id).Scan(&category.ID, &category.Name, &dbMultiplier)
	if err != nil {
		return nil, err
	}
	category.PriceMultiplierPercent = uint32(dbMultiplier)

	return &category, nil
}

func (r *SeatCategoryRepository) GetAll(ctx context.Context) ([]*SeatCategory, error) {
	query := `SELECT id, name, price_multiplier_percent FROM seat_categories ORDER BY id`

	rows, err := r.db.Query(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("query seat categories: %w", err)
	}

	categories, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (*SeatCategory, error) {
		var category SeatCategory
		var dbMultiplier int64
		err := row.Scan(&category.ID, &category.Name, &dbMultiplier)
		if err != nil {
			return nil, err
		}
		category.PriceMultiplierPercent = uint32(dbMultiplier)
		return &category, nil
	})
	if err != nil {
		return nil, fmt.Errorf("collect seat categories: %w", err)
	}

	return categories, nil
}
