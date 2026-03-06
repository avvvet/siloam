package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	BotToken           string
	GroupID            int64
	BotCreator         string
	BotCreatorUsername string
	PreviousReadings   map[string]int // unit -> previous reading
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
		BotToken:           os.Getenv("BOT_TOKEN"),
		GroupID:            groupID,
		BotCreator:         os.Getenv("BOT_CREATOR"),
		BotCreatorUsername: os.Getenv("BOT_CREATOR_USERNAME"),
		PreviousReadings:   parsePreviousReadings(os.Getenv("PREVIOUS_READINGS")),
	}
}

// parsePreviousReadings parses "a=100,b=200,c=300" from env
func parsePreviousReadings(raw string) map[string]int {
	result := make(map[string]int)
	if raw == "" {
		return result
	}
	parts := strings.Split(raw, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		kv := strings.Split(part, "=")
		if len(kv) != 2 {
			continue
		}
		unit := strings.ToLower(strings.TrimSpace(kv[0]))
		val, err := strconv.Atoi(strings.TrimSpace(kv[1]))
		if err == nil {
			result[unit] = val
		}
	}
	return result
}
