package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) CreateSeat(c *gin.Context) {
	ctx := c.Request.Context()

	var newSeat models.SeatReq
	if err := c.ShouldBindJSON(&newSeat); err != nil {
		log.Printf("bind failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	created, err := h.Seats.Create(ctx, newSeat.HallId, newSeat.SeatNumber, newSeat.Price)
	if err != nil {
		switch {
		case isUniqueViolation(err):
			c.JSON(http.StatusConflict, gin.H{"error": "this seat number already exists in the hall"})
		case isForeignKeyViolation(err):
			c.JSON(http.StatusBadRequest, gin.H{"error": "hall does not exist"})
		default:
			log.Printf("create seat failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetSeat(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	seat, err := h.Seats.GetWithDetails(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "seat not found"})
			return
		}
		log.Printf("get seat failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, seat)
}
