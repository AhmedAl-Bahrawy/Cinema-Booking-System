package models_test

import (
	"errors"
	"testing"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
)

func TestCreateShow(t *testing.T) {
	tests := []struct {
		name            string // description of this test case
		showName        string
		durationMinutes int64
		authorName      string
		want            *models.Show
		wantErr         bool
	}{
		{
			name:            "creates a show with valid data",
			showName:        "Hamlet",
			durationMinutes: 120,
			authorName:      "Shakespeare",
			want:            &models.Show{ID: 1, Name: "Hamlet", DurationMinutes: 120, AuthorName: "Shakespeare"},
			wantErr:         false,
		},
		{
			// duration_minutes has CHECK (duration_minutes > 0), so 0 must be rejected.
			name:            "zero duration is rejected",
			showName:        "Instant Show",
			durationMinutes: 0,
			authorName:      "Author",
			wantErr:         true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)

			got, gotErr := models.CreateShow(t.Context(), pool, tt.showName, tt.durationMinutes, tt.authorName)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("CreateShow() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("CreateShow() succeeded unexpectedly")
			}
			if *got != *tt.want {
				t.Errorf("CreateShow() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetShowById(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		id      int64
		want    *models.Show
		wantErr bool
	}{
		{
			name:    "returns the show for an existing id",
			id:      1,
			want:    &models.Show{ID: 1, Name: "Hamlet", DurationMinutes: 120, AuthorName: "Shakespeare"},
			wantErr: false,
		},
		{
			// No show was seeded for this id, so no rows are returned.
			name:    "returns error for a non-existent id",
			id:      999,
			wantErr: true,
		},
		{
			// id is a BIGSERIAL starting at 1, so 0 can never exist.
			name:    "returns error for id 0",
			id:      0,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)

			// Seed a known show so the test is independent of other data.
			if _, err := models.CreateShow(t.Context(), pool, "Hamlet", 120, "Shakespeare"); err != nil {
				t.Fatalf("seed show: %v", err)
			}

			got, gotErr := models.GetShowById(t.Context(), pool, tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetShowById() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetShowById() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetShowById() succeeded unexpectedly")
			}
			if *got != *tt.want {
				t.Errorf("GetShowById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}
