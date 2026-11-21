package handlers

import (
	"errors"
	"log"
	"net/http"
	"strings"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func (h *Handler) CreateShow(c *gin.Context) {
	ctx := c.Request.Context()

	var newShow models.ShowReq
	if err := c.ShouldBindJSON(&newShow); err != nil {
		log.Printf("bind failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	newShow.Name = strings.TrimSpace(newShow.Name)
	newShow.AuthorName = strings.TrimSpace(newShow.AuthorName)

	created, err := h.Shows.Create(ctx, newShow.Name, newShow.DurationMinutes, newShow.AuthorName, newShow.Price)
	if err != nil {
		log.Printf("create show failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetShow(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	show, err := h.Shows.GetById(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "show not found"})
			return
		}
		log.Printf("get show failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, show)
}
