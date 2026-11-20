package handlers

import (
	"log"
	"net/http"
	"strings"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/gin-gonic/gin"
)

func (h *Handler) CreateHall(c *gin.Context) {
	ctx := c.Request.Context()

	var newHall models.HallReq
	if err := c.ShouldBindJSON(&newHall); err != nil {
		log.Printf("bind failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	newHall.Name = strings.TrimSpace(newHall.Name)
	newHall.Capacity = uint32(newHall.Capacity)

	created, err := h.Halls.Create(ctx, newHall.Name, newHall.Capacity)
	if err != nil {
		log.Printf("create user failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "hall created successfully", "id": created.ID, "name": created.Name})
}
