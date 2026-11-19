package handlers

import "github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"

type Handler struct {
	Halls   *models.HallRepository
	Seats   *models.SeatRepository
	Shows   *models.ShowRepository
	Tickets *models.TicketRepository
	Users   *models.UserRepository
}

func NewHandler(h *models.HallRepository, st *models.SeatRepository, sh *models.ShowRepository, t *models.TicketRepository, u *models.UserRepository) *Handler {
	return &Handler{Halls: h, Seats: st, Shows: sh, Tickets: t, Users: u}
}
