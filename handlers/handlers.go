package handlers

import "github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"

type Handler struct {
	Halls      *models.HallRepository
	Seats      *models.SeatRepository
	Shows      *models.ShowRepository
	Screenings *models.ScreeningRepository
	Tickets    *models.TicketRepository
	Users      *models.UserRepository
}

func NewHandler(h *models.HallRepository, st *models.SeatRepository, sh *models.ShowRepository, sc *models.ScreeningRepository, t *models.TicketRepository, u *models.UserRepository) *Handler {
	return &Handler{Halls: h, Seats: st, Shows: sh, Screenings: sc, Tickets: t, Users: u}
}
