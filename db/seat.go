package db

// Free (0) | Holding (1) | Booked (2)
type BookingStatus int8

const (
	Free    BookingStatus = iota // 0
	Holding                      // 1
	Booked                       // 2
)

type Seat struct {
	ID         int64
	HallId     int64
	SeatNumber uint32
	Status     BookingStatus
	Price      uint32

	Hall *Hall
}
