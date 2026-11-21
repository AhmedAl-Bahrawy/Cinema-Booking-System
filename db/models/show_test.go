package models_test

import (
	"errors"
	"testing"
	"time"

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
			want:            &models.Show{ID: 1, Name: "Hamlet", DurationMinutes: 120, AuthorName: "Shakespeare", Price: 100},
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
			repo := models.NewShowRepository(pool)

			got, gotErr := repo.Create(t.Context(), tt.showName, tt.durationMinutes, tt.authorName, 100)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Create() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Create() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID || got.Name != tt.want.Name ||
				got.DurationMinutes != tt.want.DurationMinutes || got.AuthorName != tt.want.AuthorName ||
				got.Price != tt.want.Price {
				t.Errorf("Create() = %+v, want %+v", got, tt.want)
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
			want:    &models.Show{ID: 1, Name: "Hamlet", DurationMinutes: 120, AuthorName: "Shakespeare", Price: 100},
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
			repo := models.NewShowRepository(pool)

			// Seed a known show so the test is independent of other data.
			if _, err := repo.Create(t.Context(), "Hamlet", 120, "Shakespeare", 100); err != nil {
				t.Fatalf("seed show: %v", err)
			}

			got, gotErr := repo.GetById(t.Context(), tt.id)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("GetById() failed: %v", gotErr)
					return
				}
				if !errors.Is(gotErr, pgx.ErrNoRows) {
					t.Errorf("GetById() error = %v, want pgx.ErrNoRows", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("GetById() succeeded unexpectedly")
			}
			if got.ID != tt.want.ID || got.Name != tt.want.Name ||
				got.DurationMinutes != tt.want.DurationMinutes || got.AuthorName != tt.want.AuthorName ||
				got.Price != tt.want.Price {
				t.Errorf("GetById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetAllShows(t *testing.T) {
	t.Run("returns all shows ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewShowRepository(pool)

		first, err := repo.Create(t.Context(), "Hamlet", 120, "Shakespeare", 100)
		if err != nil {
			t.Fatalf("seed first show: %v", err)
		}
		second, err := repo.Create(t.Context(), "Othello", 90, "Shakespeare", 100)
		if err != nil {
			t.Fatalf("seed second show: %v", err)
		}

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAll() returned %d shows, want 2", len(got))
		}
		if got[0].ID != first.ID || got[0].Name != "Hamlet" ||
			got[0].DurationMinutes != 120 || got[0].AuthorName != "Shakespeare" {
			t.Errorf("GetAll()[0] = %+v, want %+v", got[0], first)
		}
		if got[1].ID != second.ID || got[1].Name != "Othello" ||
			got[1].DurationMinutes != 90 || got[1].AuthorName != "Shakespeare" {
			t.Errorf("GetAll()[1] = %+v, want %+v", got[1], second)
		}
	})

	t.Run("returns an empty list when there are no shows", func(t *testing.T) {
		pool := testdb.Setup(t)
		repo := models.NewShowRepository(pool)

		got, err := repo.GetAll(t.Context())
		if err != nil {
			t.Fatalf("GetAll() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAll() returned %d shows, want 0", len(got))
		}
	})
}

func TestGetAllShowsWithDetails(t *testing.T) {
	pool := testdb.Setup(t)
	showRepo := models.NewShowRepository(pool)

	show1, err := showRepo.Create(t.Context(), "Hamlet", 120, "Shakespeare", 100)
	if err != nil {
		t.Fatalf("seed first show: %v", err)
	}
	show2, err := showRepo.Create(t.Context(), "Othello", 90, "Shakespeare", 100)
	if err != nil {
		t.Fatalf("seed second show: %v", err)
	}
	hallA := seedHall(t, pool)
	hallB, err := models.NewHallRepository(pool).Create(t.Context(), "Hall B", 50)
	if err != nil {
		t.Fatalf("seed second hall: %v", err)
	}

	// Show 1 is scheduled twice (two halls); Show 2 has no screenings.
	screening1 := seedScreening(t, pool, hallA.ID, show1.ID, screeningBase)
	screening2 := seedScreening(t, pool, hallB.ID, show1.ID, screeningBase.Add(24*time.Hour))

	got, err := showRepo.GetAllWithDetails(t.Context())
	if err != nil {
		t.Fatalf("GetAllWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllWithDetails() returned %d shows, want 2", len(got))
	}

	// Show 1: two screenings attached.
	if got[0].ID != show1.ID {
		t.Fatalf("GetAllWithDetails()[0].ID = %d, want %d", got[0].ID, show1.ID)
	}
	if len(got[0].Screenings) != 2 {
		t.Fatalf("GetAllWithDetails()[0].Screenings has %d screenings, want 2", len(got[0].Screenings))
	}
	if got[0].Screenings[0].ID != screening1.ID || got[0].Screenings[0].HallId != hallA.ID {
		t.Errorf("GetAllWithDetails()[0].Screenings[0] = %+v", got[0].Screenings[0])
	}
	if got[0].Screenings[0].Hall == nil || got[0].Screenings[0].Hall.Name != "Hall A" {
		t.Errorf("GetAllWithDetails()[0].Screenings[0].Hall = %+v, want Hall A", got[0].Screenings[0].Hall)
	}
	if got[0].Screenings[1].ID != screening2.ID || got[0].Screenings[1].HallId != hallB.ID {
		t.Errorf("GetAllWithDetails()[0].Screenings[1] = %+v", got[0].Screenings[1])
	}

	// Show 2: no screenings.
	if got[1].ID != show2.ID {
		t.Fatalf("GetAllWithDetails()[1].ID = %d, want %d", got[1].ID, show2.ID)
	}
	if len(got[1].Screenings) != 0 {
		t.Errorf("GetAllWithDetails()[1].Screenings has %d screenings, want 0", len(got[1].Screenings))
	}
}
