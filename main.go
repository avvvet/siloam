package main

import (
	"log"

	"github.com/avvvet/siloam/bot"
	"github.com/avvvet/siloam/config"
	"github.com/avvvet/siloam/db"
)

func main() {
	cfg := config.Load()

	database, err := db.Open("siloam.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	siloam, err := bot.New(cfg, database)
	if err != nil {
		log.Fatalf("Failed to create bot: %v", err)
	}

	log.Println("Siloam is running...")
	siloam.Start()
}
