package main

import (
	"flag"
	"log"

	"github.com/avvvet/siloam/bot"
	"github.com/avvvet/siloam/config"
	"github.com/avvvet/siloam/db"
	"github.com/avvvet/siloam/tahor"
)

func main() {
	botFlag := flag.String("bot", "all", "Which bot to run: siloam, tahor, or all")
	flag.Parse()

	cfg := config.Load()

	database, err := db.Open("siloam.db")
	if err != nil {
		log.Fatalf("Failed to open database: %v", err)
	}
	defer database.Close()

	switch *botFlag {
	case "siloam":
		siloam, err := bot.New(cfg, database)
		if err != nil {
			log.Fatalf("Failed to create Siloam bot: %v", err)
		}
		log.Println("Siloam is running...")
		siloam.Start()

	case "tahor":
		tahorBot, err := tahor.New(cfg, database)
		if err != nil {
			log.Fatalf("Failed to create Tahor bot: %v", err)
		}
		log.Println("Tahor is running...")
		tahorBot.Start()

	default:
		siloam, err := bot.New(cfg, database)
		if err != nil {
			log.Fatalf("Failed to create Siloam bot: %v", err)
		}
		tahorBot, err := tahor.New(cfg, database)
		if err != nil {
			log.Fatalf("Failed to create Tahor bot: %v", err)
		}
		log.Println("Siloam and Tahor are running...")
		go tahorBot.Start()
		siloam.Start()
	}
}
