package handlers

import (
	"errors"
	"log"
	"net/http"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) CreateTicket(c *gin.Context) {
	ctx := c.Request.Context()

	var newTicket models.TicketReq
	if err := c.ShouldBindJSON(&newTicket); err != nil {
		log.Printf("bind failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	// A created ticket is a confirmed booking unless the caller explicitly
	// asks to keep it on hold.
	status := newTicket.Status
	if status == 0 {
		status = models.Booked
	}

	created, err := h.Tickets.Create(ctx, newTicket.ScreeningId, newTicket.SeatId, newTicket.UserId, status)
	if err != nil {
		switch {
		case isUniqueViolation(err):
			c.JSON(http.StatusConflict, gin.H{"error": "this seat is already sold for this screening"})
		case errors.Is(err, pgx.ErrNoRows), isCheckViolation(err):
			c.JSON(http.StatusBadRequest, gin.H{"error": "seat does not belong to the hall of this screening"})
		case isForeignKeyViolation(err):
			c.JSON(http.StatusBadRequest, gin.H{"error": "user does not exist"})
		default:
			log.Printf("create ticket failed: %v", err)
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetTicket(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	ticket, err := h.Tickets.GetWithDetails(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "ticket not found"})
			return
		}
		log.Printf("get ticket failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, ticket)
}
