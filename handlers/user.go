package handlers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
)

var (
	usernameRe = regexp.MustCompile(`^[a-zA-Z0-9_]{3,100}$`)

	ErrUsernameFormat = errors.New("username must be 3–100 chars, English letters, digits, or underscore")
	ErrFullNameLen    = errors.New("full name must be 3–255 characters")
	ErrUsernameTaken  = errors.New("this username is already taken")
)

func validateUser(ctx context.Context, h *Handler, user models.UserReq) error {
	u := strings.TrimSpace(user.Username)
	if !usernameRe.MatchString(u) {
		return ErrUsernameFormat
	}
	if n := len(strings.TrimSpace(user.FullName)); n < 3 || n > 255 {
		return ErrFullNameLen
	}

	exists, err := h.Users.UsernameExists(ctx, u)
	if err != nil {
		return fmt.Errorf("checking username existence: %w", err)
	}
	if exists {
		return ErrUsernameTaken
	}
	return nil
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return false
}

func (h *Handler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()

	var newUser models.UserReq
	if err := c.ShouldBindJSON(&newUser); err != nil {
		log.Printf("bind failed: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	newUser.Username = strings.TrimSpace(newUser.Username)

	if err := validateUser(ctx, h, newUser); err != nil {
		log.Printf("create user validation failed: %v", err)
		switch {
		case errors.Is(err, ErrUsernameFormat), errors.Is(err, ErrFullNameLen):
			c.JSON(http.StatusUnprocessableEntity, gin.H{"error": err.Error()})
		case errors.Is(err, ErrUsernameTaken):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		}
		return
	}

	created, err := h.Users.Create(ctx, newUser.Username, newUser.FullName)
	if err != nil {
		if isUniqueViolation(err) { // pgErr.Code == "23505"
			c.JSON(http.StatusConflict, gin.H{"error": ErrUsernameTaken.Error()})
			return
		}
		log.Printf("create user failed: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "user created successfully", "id": created.ID, "username": created.Username})
}
