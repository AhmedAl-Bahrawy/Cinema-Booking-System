package models_test

import (
	"errors"
	"testing"
	"time"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/internal/testdb"
	"github.com/jackc/pgx/v5"
)

func TestCreateScreening(t *testing.T) {
	tests := []struct {
		name    string // description of this test case
		endsAt  time.Time
		wantErr bool
	}{
		{
			name:   "creates a valid screening",
			endsAt: screeningBase.Add(2 * time.Hour),
		},
		{
			// screenings_time_valid CHECK (ends_at > starts_at).
			name:    "end before start is rejected",
			endsAt:  screeningBase.Add(-time.Hour),
			wantErr: true,
		},
		{
			// Equal start/end is an empty range, so the CHECK also rejects it.
			name:    "zero-length screening is rejected",
			endsAt:  screeningBase,
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := testdb.Setup(t)
			repo := models.NewScreeningRepository(pool)
			hall := seedHall(t, pool)
			show := seedShow(t, pool)

			got, gotErr := repo.Create(t.Context(), hall.ID, show.ID, screeningBase, tt.endsAt)
			if gotErr != nil {
				if !tt.wantErr {
					t.Fatalf("Create() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}
			if got.ID != 1 || got.HallId != hall.ID || got.ShowId != show.ID ||
				!got.StartsAt.Equal(screeningBase) || !got.EndsAt.Equal(tt.endsAt) {
				t.Errorf("Create() = %+v", got)
			}
		})
	}
}

// TestCreateScreeningAt checks the convenience method that calculates EndsAt
// from the show's duration.
func TestCreateScreeningAt(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewScreeningRepository(pool)
	hall := seedHall(t, pool)
	show := seedShow(t, pool) // Hamlet, 120 minutes

	got, err := repo.CreateAt(t.Context(), hall.ID, show.ID, screeningBase)
	if err != nil {
		t.Fatalf("CreateAt() failed: %v", err)
	}

	wantEnd := screeningBase.Add(120 * time.Minute)
	if !got.EndsAt.Equal(wantEnd) {
		t.Errorf("CreateAt() EndsAt = %v, want %v", got.EndsAt, wantEnd)
	}
}

func TestCreateScreening_Overlap(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewScreeningRepository(pool)
	hallA := seedHall(t, pool)
	hallB, err := models.NewHallRepository(pool).Create(t.Context(), "Hall B", 50)
	if err != nil {
		t.Fatalf("seed hall B: %v", err)
	}
	hamlet := seedShow(t, pool)
	othello, err := models.NewShowRepository(pool).Create(t.Context(), "Othello", 90, "Shakespeare")
	if err != nil {
		t.Fatalf("seed Othello: %v", err)
	}

	// 10:00–12:00 in Hall A.
	if _, err := repo.Create(t.Context(), hallA.ID, hamlet.ID, screeningBase, screeningBase.Add(2*time.Hour)); err != nil {
		t.Fatalf("seed first screening: %v", err)
	}

	// Overlapping slot in the same hall is rejected, even for another show.
	if _, err := repo.Create(t.Context(), hallA.ID, othello.ID,
		screeningBase.Add(time.Hour), screeningBase.Add(3*time.Hour)); err == nil {
		t.Error("Create() accepted an overlapping screening in the same hall")
	}

	// A back-to-back screening in the same hall is allowed.
	if _, err := repo.Create(t.Context(), hallA.ID, othello.ID,
		screeningBase.Add(2*time.Hour), screeningBase.Add(4*time.Hour)); err != nil {
		t.Errorf("Create() rejected a back-to-back screening: %v", err)
	}

	// The same time in a different hall is allowed.
	if _, err := repo.Create(t.Context(), hallB.ID, othello.ID,
		screeningBase, screeningBase.Add(90*time.Minute)); err != nil {
		t.Errorf("Create() rejected the same slot in another hall: %v", err)
	}
}

// TestCreateScreening_SameHallDifferentShowsSameDay verifies the requested
// behaviour: a hall can change the show during the day.
func TestCreateScreening_SameHallDifferentShowsSameDay(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewScreeningRepository(pool)
	hall := seedHall(t, pool)
	hamlet := seedShow(t, pool)
	othello, err := models.NewShowRepository(pool).Create(t.Context(), "Othello", 90, "Shakespeare")
	if err != nil {
		t.Fatalf("seed Othello: %v", err)
	}

	morning, err := repo.CreateAt(t.Context(), hall.ID, hamlet.ID, screeningBase)
	if err != nil {
		t.Fatalf("CreateAt() morning failed: %v", err)
	}
	evening, err := repo.CreateAt(t.Context(), hall.ID, othello.ID, screeningBase.Add(8*time.Hour))
	if err != nil {
		t.Fatalf("CreateAt() evening failed: %v", err)
	}

	if morning.ShowId == evening.ShowId {
		t.Error("both screenings point at the same show")
	}
	if !morning.StartsAt.Truncate(24 * time.Hour).Equal(evening.StartsAt.Truncate(24 * time.Hour)) {
		t.Error("screenings were not scheduled on the same day")
	}

	schedule, err := repo.GetByHall(t.Context(), hall.ID)
	if err != nil {
		t.Fatalf("GetByHall() failed: %v", err)
	}
	if len(schedule) != 2 {
		t.Fatalf("GetByHall() returned %d screenings, want 2", len(schedule))
	}
	if schedule[0].ShowId != hamlet.ID || schedule[1].ShowId != othello.ID {
		t.Errorf("schedule order = (%d, %d), want (%d, %d)",
			schedule[0].ShowId, schedule[1].ShowId, hamlet.ID, othello.ID)
	}
}

func TestCreateScreening_InvalidForeignKeys(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewScreeningRepository(pool)
	hall := seedHall(t, pool)
	show := seedShow(t, pool)

	if _, err := repo.Create(t.Context(), 999, show.ID, screeningBase, screeningBase.Add(time.Hour)); err == nil {
		t.Error("Create() accepted a non-existent hall")
	}
	if _, err := repo.Create(t.Context(), hall.ID, 999, screeningBase, screeningBase.Add(time.Hour)); err == nil {
		t.Error("Create() accepted a non-existent show")
	}
}

func TestGetScreeningWithDetails(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewScreeningRepository(pool)
	hall := seedHall(t, pool)
	show := seedShow(t, pool)
	if _, err := repo.CreateAt(t.Context(), hall.ID, show.ID, screeningBase); err != nil {
		t.Fatalf("seed screening: %v", err)
	}

	got, err := repo.GetWithDetails(t.Context(), 1)
	if err != nil {
		t.Fatalf("GetWithDetails() failed: %v", err)
	}
	if got.Hall == nil || got.Hall.ID != hall.ID || got.Hall.Name != "Hall A" {
		t.Errorf("GetWithDetails() Hall = %+v, want hall %d", got.Hall, hall.ID)
	}
	if got.Show == nil || got.Show.ID != show.ID || got.Show.Name != "Hamlet" ||
		got.Show.DurationMinutes != 120 {
		t.Errorf("GetWithDetails() Show = %+v, want show %d", got.Show, show.ID)
	}

	_, err = repo.GetWithDetails(t.Context(), 999)
	if !errors.Is(err, pgx.ErrNoRows) {
		t.Errorf("GetWithDetails(999) error = %v, want pgx.ErrNoRows", err)
	}
}

func TestGetAllScreeningsWithDetails(t *testing.T) {
	pool := testdb.Setup(t)
	repo := models.NewScreeningRepository(pool)
	hall := seedHall(t, pool)
	show := seedShow(t, pool)

	first, err := repo.CreateAt(t.Context(), hall.ID, show.ID, screeningBase)
	if err != nil {
		t.Fatalf("seed first screening: %v", err)
	}
	second, err := repo.CreateAt(t.Context(), hall.ID, show.ID, screeningBase.Add(3*time.Hour))
	if err != nil {
		t.Fatalf("seed second screening: %v", err)
	}

	got, err := repo.GetAllWithDetails(t.Context())
	if err != nil {
		t.Fatalf("GetAllWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllWithDetails() returned %d screenings, want 2", len(got))
	}
	if got[0].ID != first.ID || got[1].ID != second.ID {
		t.Errorf("GetAllWithDetails() order = (%d, %d)", got[0].ID, got[1].ID)
	}
	for i := range got {
		if got[i].Hall == nil || got[i].Hall.ID != hall.ID {
			t.Errorf("GetAllWithDetails()[%d].Hall = %+v", i, got[i].Hall)
		}
		if got[i].Show == nil || got[i].Show.ID != show.ID {
			t.Errorf("GetAllWithDetails()[%d].Show = %+v", i, got[i].Show)
		}
	}
}

func TestGetAllScreenings(t *testing.T) {
	t.Run("returns all screenings ordered by start time", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewScreeningRepository(pool)
		hall := seedHall(t, pool)
		show := seedShow(t, pool)

		second, err := repo.CreateAt(t.Context(), hall.ID, show.ID, screeningBase.Add(4*time.Hour))
		if err != nil {
			t.Fatalf("seed second screening: %v", err)
		}
		first, err := repo.CreateAt(t.Context(), hall.ID, show.ID, screeningBase)
		if err != nil {
			t.Fatalf("seed first screening: %v", err)
		}

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}
		if len(got) != 2 {
			t.Fatalf("GetAll() returned %d screenings, want 2", len(got))
		}
		if got[0].ID != first.ID || got[1].ID != second.ID {
			t.Errorf("GetAll() order = (%d, %d), want (%d, %d)",
				got[0].ID, got[1].ID, first.ID, second.ID)
		}
	})

	t.Run("returns an empty list when there are no screenings", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewScreeningRepository(pool)

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAll() returned %d screenings, want 0", len(got))
		}
	})
}
