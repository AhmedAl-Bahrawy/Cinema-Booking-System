package db

type Show struct {
	ID              int64  `db:"id"               json:"id"               binding:"omitempty,min=1"`
	Name            string `db:"name"             json:"name"             binding:"required,min=1,max=255"`
	DurationMinutes int64  `db:"duration_minutes" json:"duration_minutes" binding:"required,min=1"` // per-minutes
	AuthorName      string `db:"author_name"      json:"author_name"      binding:"required,min=1,max=255"`
}
