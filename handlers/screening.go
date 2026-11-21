package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) CreateScreening(c *gin.Context) {
	ctx := c.Request.Context()

	var newScreening models.ScreeningReq
	if err := c.ShouldBindJSON(&newScreening); err != nil {
		log.Printf("bind failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if !newScreening.EndsAt.After(newScreening.StartsAt) {
		c.JSON(http.StatusUnprocessableEntity, gin.H{"error": "ends_at must be after starts_at"})
		return
	}

	created, err := h.Screenings.Create(ctx, newScreening.HallId, newScreening.ShowId, newScreening.StartsAt, newScreening.EndsAt)
	if err != nil {
		switch {
		case isExclusionViolation(err):
			c.JSON(http.StatusConflict, gin.H{"error": "the hall already has another screening in this time range"})
		case isForeignKeyViolation(err):
			c.JSON(http.StatusBadRequest, gin.H{"error": "hall or show does not exist"})
		default:
			log.Printf("create screening failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetScreening(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	screening, err := h.Screenings.GetWithDetails(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "screening not found"})
			return
		}
		log.Printf("get screening failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, screening)
}
