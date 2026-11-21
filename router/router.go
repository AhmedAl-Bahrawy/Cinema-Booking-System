package router

import (
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/handlers"
	"github.com/gin-gonic/gin"
)

func SetupRouter(h *handlers.Handler) *gin.Engine {
	r := gin.Default()

	r.POST("/user", h.CreateUser)
	r.POST("/hall", h.CreateHall)
	r.POST("/show", h.CreateShow)
	r.GET("/show/:id", h.GetShow)
	r.POST("/seat-category", h.CreateSeatCategory)
	r.GET("/seat-category/:id", h.GetSeatCategory)
	r.GET("/seat-category", h.GetAllSeatCategories)
	r.POST("/seat", h.CreateSeat)
	r.GET("/seat/:id", h.GetSeat)
	r.POST("/screening", h.CreateScreening)
	r.GET("/screening/:id", h.GetScreening)
	r.POST("/ticket", h.CreateTicket)
	r.GET("/ticket/:id", h.GetTicket)

	return r
}
