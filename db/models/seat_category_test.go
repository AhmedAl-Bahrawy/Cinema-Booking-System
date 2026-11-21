package models_test

import (
	"errors"
	"testing"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
)

// TestSeededSeatCategories verifies the categories inserted by the migration.
func TestSeededSeatCategories(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewSeatCategoryRepository(pool)

	got, err := repo.GetAll(t.Context())
	if err != nil {
		t.Fatalf("GetAll() failed: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("GetAll() returned %d categories, want 3", len(got))
	}

	want := []struct {
		name       string
		multiplier uint32
	}{
		{"normal", 100},
		{"premium", 150},
		{"vip", 200},
	}
	for i, w := range want {
		if got[i].ID != int64(i+1) || got[i].Name != w.name ||
			got[i].PriceMultiplierPercent != w.multiplier {
			t.Errorf("GetAll()[%d] = %+v, want %s (%d)", i, got[i], w.name, w.multiplier)
		}
	}
}

func TestCreateSeatCategory(t *testing.T) {
	tests := []struct {
		name       string
		category   string
		multiplier uint32
		wantErr    bool
	}{
		{
			name:       "creates a custom category between normal and VIP",
			category:   "box",
			multiplier: 125,
		},
		{
			// price_multiplier_percent has CHECK (> 0).
			name:       "zero multiplier is rejected",
			category:   "free",
			multiplier: 0,
			wantErr:    true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewSeatCategoryRepository(pool)

			got, gotErr := repo.Create(t.Context(), tt.category, tt.multiplier)
			if gotErr != nil {
				if !tt.wantErr {
					t.Fatalf("Create() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}
			if got.ID != 4 || got.Name != tt.category || got.PriceMultiplierPercent != tt.multiplier {
				t.Errorf("Create() = %+v", got)
			}
		})
	}
}

// TestCreateSeatCategory_DuplicateName verifies the UNIQUE constraint on name.
func TestCreateSeatCategory_DuplicateName(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewSeatCategoryRepository(pool)

	second, err := repo.Create(t.Context(), "normal", 100)
	if err == nil {
		t.Errorf("Create() succeeded unexpectedly for duplicate name: %+v", second)
	}
}

func TestGetSeatCategoryById(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewSeatCategoryRepository(pool)

	got, err := repo.GetById(t.Context(), 2)
	if err != nil {
		t.Fatalf("GetById() failed: %v", err)
	}
	if got.ID != 2 || got.Name != "premium" || got.PriceMultiplierPercent != 150 {
		t.Errorf("GetById() = %+v, want premium (150)", got)
	}

	if _, err := repo.GetById(t.Context(), 999); !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetById(999) error = %v, want pgx.ErrNoRows", err)
	}
}
