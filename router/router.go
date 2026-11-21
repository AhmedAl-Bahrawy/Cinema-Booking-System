package router

import (
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/handlers"
	"github.com/gin-gonic/gin"
)

func SetupRouter(h *handlers.Handler) *gin.Engine {
	r := gin.Default()

	r.POST("/user", h.CreateUser)
	r.POST("/hall", h.CreateHall)
	r.POST("/seat", h.CreateSeat)
	r.GET("/seat/:id", h.GetSeat)
	r.POST("/screening", h.CreateScreening)
	r.GET("/screening/:id", h.GetScreening)
	r.POST("/ticket", h.CreateTicket)
	r.GET("/ticket/:id", h.GetTicket)

	return r
}
