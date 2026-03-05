package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken   string
	GroupID    int64
	BotCreator string
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	groupIDStr := os.Getenv("GROUP_ID")
	groupID, err := strconv.ParseInt(groupIDStr, 10, 64)
	if err != nil {
		log.Fatalf("Invalid GROUP_ID: %v", err)
	}

	return &Config{
		BotToken:   os.Getenv("BOT_TOKEN"),
		GroupID:    groupID,
		BotCreator: os.Getenv("BOT_CREATOR"),
	}
}
