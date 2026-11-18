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
			if got.ID != tt.want.ID || got.Name != tt.want.Name ||
				got.DurationMinutes != tt.want.DurationMinutes || got.AuthorName != tt.want.AuthorName {
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
			if got.ID != tt.want.ID || got.Name != tt.want.Name ||
				got.DurationMinutes != tt.want.DurationMinutes || got.AuthorName != tt.want.AuthorName {
				t.Errorf("GetShowById() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestGetAllShows(t *testing.T) {
	t.Run("returns all shows ordered by id", func(t *testing.T) {
		pool := testdb.Setup(t)

		first, err := models.CreateShow(t.Context(), pool, "Hamlet", 120, "Shakespeare")
		if err != nil {
			t.Fatalf("seed first show: %v", err)
		}
		second, err := models.CreateShow(t.Context(), pool, "Othello", 90, "Shakespeare")
		if err != nil {
			t.Fatalf("seed second show: %v", err)
		}

		got, err := models.GetAllShows(t.Context(), pool)
		if err != nil {
			t.Fatalf("GetAllShows() failed: %v", err)
		}

		if len(got) != 2 {
			t.Fatalf("GetAllShows() returned %d shows, want 2", len(got))
		}
		if got[0].ID != first.ID || got[0].Name != "Hamlet" ||
			got[0].DurationMinutes != 120 || got[0].AuthorName != "Shakespeare" {
			t.Errorf("GetAllShows()[0] = %+v, want %+v", got[0], first)
		}
		if got[1].ID != second.ID || got[1].Name != "Othello" ||
			got[1].DurationMinutes != 90 || got[1].AuthorName != "Shakespeare" {
			t.Errorf("GetAllShows()[1] = %+v, want %+v", got[1], second)
		}
	})

	t.Run("returns an empty list when there are no shows", func(t *testing.T) {
		pool := testdb.Setup(t)

		got, err := models.GetAllShows(t.Context(), pool)
		if err != nil {
			t.Fatalf("GetAllShows() failed: %v", err)
		}
		if len(got) != 0 {
			t.Errorf("GetAllShows() returned %d shows, want 0", len(got))
		}
	})
}

func TestGetAllShowsWithDetails(t *testing.T) {
	pool := testdb.Setup(t)

	show1, err := models.CreateShow(t.Context(), pool, "Hamlet", 120, "Shakespeare")
	if err != nil {
		t.Fatalf("seed first show: %v", err)
	}
	show2, err := models.CreateShow(t.Context(), pool, "Othello", 90, "Shakespeare")
	if err != nil {
		t.Fatalf("seed second show: %v", err)
	}
	hall := seedHall(t, pool)

	seat1, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 1, models.Free, 100)
	if err != nil {
		t.Fatalf("seed first seat: %v", err)
	}
	seat2, err := models.CreateSeat(t.Context(), pool, 0, hall.ID, 2, models.Free, 100)
	if err != nil {
		t.Fatalf("seed second seat: %v", err)
	}

	// Show 1 has two tickets; Show 2 has none.
	ticket1, err := models.CreateTicket(t.Context(), pool, seat1.ID, hall.ID, show1.ID)
	if err != nil {
		t.Fatalf("seed first ticket: %v", err)
	}
	ticket2, err := models.CreateTicket(t.Context(), pool, seat2.ID, hall.ID, show1.ID)
	if err != nil {
		t.Fatalf("seed second ticket: %v", err)
	}

	got, err := models.GetAllShowsWithDetails(t.Context(), pool)
	if err != nil {
		t.Fatalf("GetAllShowsWithDetails() failed: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("GetAllShowsWithDetails() returned %d shows, want 2", len(got))
	}

	// Show 1: two tickets attached.
	if got[0].ID != show1.ID {
		t.Fatalf("GetAllShowsWithDetails()[0].ID = %d, want %d", got[0].ID, show1.ID)
	}
	if len(got[0].Tickets) != 2 {
		t.Fatalf("GetAllShowsWithDetails()[0].Tickets has %d tickets, want 2", len(got[0].Tickets))
	}
	if got[0].Tickets[0].ID != ticket1.ID || got[0].Tickets[0].SeatId != seat1.ID ||
		got[0].Tickets[0].HallId != hall.ID || got[0].Tickets[0].ShowId != show1.ID {
		t.Errorf("GetAllShowsWithDetails()[0].Tickets[0] = %+v", got[0].Tickets[0])
	}
	if got[0].Tickets[1].ID != ticket2.ID || got[0].Tickets[1].SeatId != seat2.ID {
		t.Errorf("GetAllShowsWithDetails()[0].Tickets[1] = %+v", got[0].Tickets[1])
	}

	// Show 2: no tickets.
	if got[1].ID != show2.ID {
		t.Fatalf("GetAllShowsWithDetails()[1].ID = %d, want %d", got[1].ID, show2.ID)
	}
	if len(got[1].Tickets) != 0 {
		t.Errorf("GetAllShowsWithDetails()[1].Tickets has %d tickets, want 0", len(got[1].Tickets))
	}
}
