package handlers

import (
	"fmt"
	"log"
	"net/http"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/gin-gonic/gin"
)

func validateUser(c *gin.Context, user models.UserReq) error {
	if len(user.Username) < 3 || len(user.Username) > 100 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "username length is lower or longer than must."})
		return fmt.Errorf("username length is lower or longer than must.")
	}
	if len(user.FullName) < 3 || len(user.FullName) > 255 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "fullName length is lower or longer than must."})
		return fmt.Errorf("fullName length is lower or longer than must.")
	}
	return nil
}

func (h *Handler) CreateUser(c *gin.Context) {
	ctx := c.Request.Context()

	var newUser models.UserReq

	err := c.ShouldBindJSON(&newUser)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "JSON can't unmarshal body"})
		return
	}

	err = validateUser(c, newUser)
	if err != nil {
		log.Fatal(err)
		return
	}

	_, err = h.Users.Create(ctx, newUser.Username, newUser.FullName)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "new user can not be created!!"})
		log.Fatal("new user can not be created!!")
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "user created successfully"})
}
