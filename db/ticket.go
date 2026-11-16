package db

type Ticket struct {
	ID     int64
	SeatId int64
	HallId int64
	ShowId int64

	Seat *Seat
	Hall *Hall
	Show *Show
}
