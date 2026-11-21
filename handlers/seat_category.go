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

func (h *Handler) CreateSeatCategory(c *gin.Context) {
	ctx := c.Request.Context()

	var newCategory models.SeatCategoryReq
	if err := c.ShouldBindJSON(&newCategory); err != nil {
		log.Printf("bind failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	newCategory.Name = strings.TrimSpace(newCategory.Name)

	created, err := h.SeatCategories.Create(ctx, newCategory.Name, newCategory.PriceMultiplierPercent)
	if err != nil {
		if isUniqueViolation(err) {
			c.JSON(http.StatusConflict, gin.H{"error": "this seat category already exists"})
			return
		}
		log.Printf("create seat category failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, created)
}

func (h *Handler) GetSeatCategory(c *gin.Context) {
	id, ok := parseIDParam(c, "id")
	if !ok {
		return
	}

	category, err := h.SeatCategories.GetById(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			c.JSON(http.StatusNotFound, gin.H{"error": "seat category not found"})
			return
		}
		log.Printf("get seat category failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, category)
}

func (h *Handler) GetAllSeatCategories(c *gin.Context) {
	categories, err := h.SeatCategories.GetAll(c.Request.Context())
	if err != nil {
		log.Printf("get seat categories failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusOK, categories)
}
