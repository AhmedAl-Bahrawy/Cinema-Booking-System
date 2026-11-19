package main

import (
	"log"
	"os"

	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/db/models"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/handlers"
	"github.com/AhmedAl-Bahrawy/Cinema-Booking-System/router"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func loadEnv() (*string, error) {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
		return nil, err
	}

	databaseURL := os.Getenv("GOOSE_DBSTRING")
	if databaseURL == "" {
		log.Fatal("DB_URL is empty")
	}
	return &databaseURL, nil
}

func loadPool(databaseURL *string) (*pgxpool.Pool, error) {
	pool, err := db.ConnectDB(databaseURL)
	if err != nil {
		return nil, err
	}

	return pool, nil
}

func main() {
	databaseURL, err := loadEnv()
	if err != nil {
		log.Fatal(err)
	}

	pool, err := loadPool(databaseURL)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()

	hallRep := models.NewHallRepository(pool)
	seatRep := models.NewSeatRepository(pool)
	showRep := models.NewShowRepository(pool)
	ticketRep := models.NewTicketRepository(pool)
	userRep := models.NewUserRepository(pool)

	h := handlers.NewHandler(hallRep, seatRep, showRep, ticketRep, userRep)

	r := router.SetupRouter(h)

	err = r.Run(":8080")
	if err != nil {
		log.Fatal(err)
		return
	}
}
