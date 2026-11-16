package db

type Ticket struct {
	ID     int64 `db:"id"      json:"id"      binding:"omitempty,min=1"`
	SeatId int64 `db:"seat_id" json:"seat_id" binding:"required,min=1"`
	HallId int64 `db:"hall_id" json:"hall_id" binding:"required,min=1"`
	ShowId int64 `db:"show_id" json:"show_id" binding:"required,min=1"`

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	Seat *Seat `db:"-" json:"seat,omitempty" binding:"-"`
	Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
	Show *Show `db:"-" json:"show,omitempty" binding:"-"`
}
