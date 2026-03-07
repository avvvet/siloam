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

	switch *botFlag {
	case "siloam":
		database, err := db.Open("siloam.db")
		if err != nil {
			log.Fatalf("Failed to open Siloam database: %v", err)
		}
		defer database.Close()
		siloam, err := bot.New(cfg, database)
		if err != nil {
			log.Fatalf("Failed to create Siloam bot: %v", err)
		}
		log.Println("Siloam is running...")
		siloam.Start()

	case "tahor":
		tahorDB, err := db.Open("tahor.db")
		if err != nil {
			log.Fatalf("Failed to open Tahor database: %v", err)
		}
		defer tahorDB.Close()
		tahorBot, err := tahor.New(cfg, tahorDB)
		if err != nil {
			log.Fatalf("Failed to create Tahor bot: %v", err)
		}
		log.Println("Tahor is running...")
		tahorBot.Start()

	default:
		database, err := db.Open("siloam.db")
		if err != nil {
			log.Fatalf("Failed to open Siloam database: %v", err)
		}
		defer database.Close()
		tahorDB, err := db.Open("tahor.db")
		if err != nil {
			log.Fatalf("Failed to open Tahor database: %v", err)
		}
		defer tahorDB.Close()
		siloam, err := bot.New(cfg, database)
		if err != nil {
			log.Fatalf("Failed to create Siloam bot: %v", err)
		}
		tahorBot, err := tahor.New(cfg, tahorDB)
		if err != nil {
			log.Fatalf("Failed to create Tahor bot: %v", err)
		}
		log.Println("Siloam and Tahor are running...")
		go tahorBot.Start()
		siloam.Start()
	}
}
