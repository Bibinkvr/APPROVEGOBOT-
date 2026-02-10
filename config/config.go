package config

import (
	"os"
	"strconv"
)

type Config struct {
	BotToken     string
	MongoURI     string
	AdminID      int64
	Port         string
	LogChannelID int64
	SecretPath   string
}

func LoadConfig() Config {
	adminID, _ := strconv.ParseInt(os.Getenv("ADMIN_ID"), 10, 64)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	token := os.Getenv("BOT_TOKEN")

	return Config{
		BotToken:     token,
		MongoURI:     os.Getenv("MONGODB_URI"),
		AdminID:      adminID,
		Port:         port,
		SecretPath:   "/webhook/" + os.Getenv("BOT_TOKEN"),
		LogChannelID: parseID(os.Getenv("LOG_CHANNEL_ID")),
	}
}

func parseID(s string) int64 {
	id, _ := strconv.ParseInt(s, 10, 64)
	return id
}
