package main

import (
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

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

	// Shared Health Check
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("OK")) })

	if os.Getenv("MODE") == "webhook" {
		http.HandleFunc(cfg.SecretPath, approvalBot.WebhookHandler)
		log.Printf("Webhook active on port %s", cfg.Port)
	} else {
		log.Printf("Polling mode active. Health check on port %s", cfg.Port)
		go approvalBot.StartPolling()
	}

	server := &http.Server{Addr: ":" + cfg.Port}
	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		server.Close()
	}()

	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("Server failed: %v", err)
	}
}
