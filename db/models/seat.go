package models

// Free (0) | Holding (1) | Booked (2)
type BookingStatus int8

const (
	Free    BookingStatus = iota // 0
	Holding                      // 1
	Booked                       // 2
)

type Seat struct {
	ID         int64         `db:"id"          json:"id"          binding:"omitempty,min=1"`
	UserId     int64         `db:"user_id"     json:"user_id"     binding:"omitempty,min=1"` // null until the seat is held/booked
	HallId     int64         `db:"hall_id"     json:"hall_id"     binding:"required,min=1"`
	SeatNumber uint32        `db:"seat_number" json:"seat_number" binding:"required,min=1"`
	Status     BookingStatus `db:"status"      json:"status"      binding:"oneof=0 1 2"`
	Price      uint32        `db:"price"       json:"price"       binding:"gte=0"` // 0 allowed (free seat)

	// Joined relations: filled manually after a JOIN query, never scanned directly.
	User *User `db:"-" json:"user,omitempty" binding:"-"`
	Hall *Hall `db:"-" json:"hall,omitempty" binding:"-"`
}
