package main

import (
	"log"
	"net/http"
	"os"
	"time"

	"telegram-approval-bot/bot"
	"telegram-approval-bot/config"
	"telegram-approval-bot/db"

	"github.com/joho/godotenv"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	cfg := config.LoadConfig()
	if cfg.BotToken == "" {
		log.Fatal("BOT_TOKEN is required")
	}
	if cfg.MongoURI == "" {
		log.Fatal("MONGODB_URI is required")
	}

	log.Printf("Starting bot (Admin: %d, Port: %s)", cfg.AdminID, cfg.Port)

	database := db.InitDB(cfg.MongoURI)
	approvalBot := bot.NewBot(cfg, database)

	// Start Worker
	go approvalBot.Start()

	// Notify Admin
	go approvalBot.SendMessage(cfg.AdminID, "🤖 <b>Bot Started</b>")

	// Keep-Alive Ping (Prevents Render from spinning down)
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			resp, err := http.Get("http://localhost:" + cfg.Port + "/health")
			if err == nil {
				resp.Body.Close()
				log.Println("Keep-alive ping sent")
			}
		}
	}()

	// Shared Health Check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})

	if os.Getenv("MODE") == "webhook" {
		http.HandleFunc(cfg.SecretPath, approvalBot.WebhookHandler)
		log.Printf("Starting in WEBHOOK mode on port %s", cfg.Port)
	} else {
		log.Printf("Starting in POLLING mode. Health server on port %s", cfg.Port)
		go approvalBot.StartPolling()
	}

	// Always listen on all interfaces to satisfy Render's port binding check.
	// Render expects a successful bind to its detected port within a few minutes.
	addr := ":" + cfg.Port
	log.Printf("HTTP Server now binding to %s", addr)

	err := http.ListenAndServe(addr, nil)
	if err != nil {
		log.Fatalf("Critical: HTTP Server failed to bind: %v", err)
	}
}
