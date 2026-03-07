package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	// Siloam
	BotToken           string
	GroupID            int64
	BotCreator         string
	BotCreatorUsername string
	PreviousReadings   map[string]int
	AdditionalFee      float64

	// Tahor
	TahorBotToken string
	TahorGroupID  int64
}

func Load() *Config {
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, reading from environment")
	}

	groupID, err := strconv.ParseInt(os.Getenv("GROUP_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Invalid GROUP_ID: %v", err)
	}

	tahorGroupID, err := strconv.ParseInt(os.Getenv("TAHOR_GROUP_ID"), 10, 64)
	if err != nil {
		log.Fatalf("Invalid TAHOR_GROUP_ID: %v", err)
	}

	additionalFee := 50.0
	if v := os.Getenv("ADDITIONAL_FEE"); v != "" {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			additionalFee = f
		}
	}

	return &Config{
		BotToken:           os.Getenv("BOT_TOKEN"),
		GroupID:            groupID,
		BotCreator:         os.Getenv("BOT_CREATOR"),
		BotCreatorUsername: os.Getenv("BOT_CREATOR_USERNAME"),
		PreviousReadings:   parsePreviousReadings(os.Getenv("PREVIOUS_READINGS")),
		AdditionalFee:      additionalFee,
		TahorBotToken:      os.Getenv("TAHOR_BOT_TOKEN"),
		TahorGroupID:       tahorGroupID,
	}
}

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
