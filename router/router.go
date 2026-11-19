package router

import (
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/handlers"
	"github.com/gin-gonic/gin"
)

func SetupRouter(h *handlers.Handler) *gin.Engine {
	r := gin.Default()

	r.POST("/user", h.CreateUser)

	return r
}
