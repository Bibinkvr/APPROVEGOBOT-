package bot

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"telegram-approval-bot/config"
	"telegram-approval-bot/db"
	"telegram-approval-bot/models"
)

type Bot struct {
	Config          config.Config
	DB              *db.Database
	HTTPClient      *http.Client
	JobQueue        chan models.Update
	WorkerPool      int
	BaseURL         string // Pre-calculated URL for speed
	BroadcastStatus *BroadcastStatus
}

type BroadcastStatus struct {
	IsRunning bool
	Processed int
	Total     int
	StartTime time.Time
}

func NewBot(cfg config.Config, database *db.Database) *Bot {
	return &Bot{
		Config: cfg,
		DB:     database,
		HTTPClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				IdleConnTimeout:     90 * time.Second,
				MaxIdleConnsPerHost: 100,
				ForceAttemptHTTP2:   true,
			},
			Timeout: 40 * time.Second,
		},
		JobQueue:        make(chan models.Update, 1000),
		WorkerPool:      30, // Increased concurrency
		BaseURL:         "https://api.telegram.org/bot" + cfg.BotToken,
		BroadcastStatus: &BroadcastStatus{},
	}
}

func (b *Bot) Start() {
	for i := 0; i < b.WorkerPool; i++ {
		go b.StartWorker()
	}
}

func (b *Bot) StartWorker() {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Worker Recovered: %v", r)
			go b.StartWorker() // Auto-restart
		}
	}()
	for update := range b.JobQueue {
		b.ProcessUpdate(update)
	}
}

func (b *Bot) StartPolling() {
	b.HTTPClient.Get(fmt.Sprintf("https://api.telegram.org/bot%s/deleteWebhook", b.Config.BotToken))

	offset := int64(0)
	log.Println("Bot started in Polling Mode")
	for {
		url := fmt.Sprintf("https://api.telegram.org/bot%s/getUpdates?offset=%d&timeout=30", b.Config.BotToken, offset)
		resp, err := b.HTTPClient.Get(url)
		if err != nil {
			time.Sleep(5 * time.Second)
			continue
		}

		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		var updateResp struct {
			Ok     bool            `json:"ok"`
			Result []models.Update `json:"result"`
		}
		if err := json.Unmarshal(body, &updateResp); err != nil {
			time.Sleep(2 * time.Second)
			continue
		}

		for _, update := range updateResp.Result {
			// SPEED-FIX: Handle join requests INSTANTLY by skipping the queue
			if update.ChatJoinRequest != nil {
				go b.HandleJoinRequest(update.ChatJoinRequest)
			} else {
				b.JobQueue <- update
			}
			offset = update.UpdateID + 1
		}
	}
}

func (b *Bot) HandleJoinRequest(req *models.ChatJoinRequest) {
	// ULTRA-FAST: Approve IMMEDIATELY
	go b.ApproveRequest(req.Chat.ID, req.From.ID)

	// Log to Channel Async
	if b.Config.LogChannelID != 0 {
		logText := fmt.Sprintf("✅ <b>New Approval</b>\n\n"+
			"<b>User:</b> %d (%s)\n"+
			"<b>Channel:</b> %s (%d)\n"+
			"<b>Time:</b> %s",
			req.From.ID, req.From.Username,
			req.Chat.Title, req.Chat.ID,
			time.Now().Format("2006-01-02 15:04:05"))
		go b.SendMessage(b.Config.LogChannelID, logText)
	}

	// Save User Async
	b.DB.SaveUserAsync(models.User{
		UserID:        req.From.ID,
		Username:      req.From.Username,
		SourceChannel: req.Chat.ID,
		FirstSeen:     time.Now(),
	})

	// Mark Channel as seen
	b.DB.AddChannelAsync(models.Channel{
		ChannelID: req.Chat.ID,
		Title:     req.Chat.Title,
	})

	// Send Welcome Message Async
	go b.SendMessage(req.From.ID, "Welcome! /start to know more.")
}

func (b *Bot) HandleMessage(msg *models.Message) {
	if msg.Text == "/start" {
		welcomeText := "Add me as an admin to your channel/group and I will approve join requests instantly! ⚡️"
		keyboard := models.InlineKeyboardMarkup{
			InlineKeyboard: [][]models.InlineKeyboardButton{{
				{Text: "Add me to your Channel", URL: "https://t.me/AcceptAutoReqBot?startchannel=true"},
			}},
		}
		b.SendMessage(msg.From.ID, welcomeText, keyboard)

		b.DB.SaveUserAsync(models.User{
			UserID:    msg.From.ID,
			Username:  msg.From.Username,
			FirstSeen: time.Now(),
		})
		return
	}

	if b.IsAdmin(msg.From.ID) && msg.Text == "/broadcast" && msg.ReplyToMessage != nil {
		go b.StartAdvancedBroadcast(msg.From.ID, msg.ReplyToMessage.MessageID)
		return
	}

	if b.IsAdmin(msg.From.ID) && len(msg.Text) > 11 && msg.Text[:11] == "/broadcast " {
		go b.StartBroadcast(msg.Text[11:])
		return
	}

	if b.IsAdmin(msg.From.ID) && msg.Text == "/stats" {
		totalUsers, _ := b.DB.GetTotalUsers()
		status := "Idle"
		if b.BroadcastStatus.IsRunning {
			status = fmt.Sprintf("Running 🏃‍♂️\n<b>Progress:</b> %d/%d\n<b>Started:</b> %s",
				b.BroadcastStatus.Processed, b.BroadcastStatus.Total,
				b.BroadcastStatus.StartTime.Format("15:04:05"))
		}

		statsText := fmt.Sprintf("📊 <b>Bot Statistics</b>\n\n"+
			"<b>Total Users:</b> %d\n"+
			"<b>Broadcast Status:</b> %s",
			totalUsers, status)

		b.SendMessage(msg.From.ID, statsText)
	}
}

func (b *Bot) CopyMessage(toID, fromID, messageID int64) {
	url := fmt.Sprintf("https://api.telegram.org/bot%s/copyMessage", b.Config.BotToken)
	data := map[string]interface{}{
		"chat_id":      chat_id_to_string(toID),
		"from_chat_id": chat_id_to_string(fromID),
		"message_id":   messageID,
	}
	resp, err := b.callTelegram(url, data)
	if err == nil {
		resp.Body.Close()
	}
}

func (b *Bot) StartAdvancedBroadcast(fromChatID, messageID int64) {
	users, err := b.DB.GetAllUserIDs()
	if err != nil {
		return
	}

	b.BroadcastStatus.IsRunning = true
	b.BroadcastStatus.Total = len(users)
	b.BroadcastStatus.Processed = 0
	b.BroadcastStatus.StartTime = time.Now()

	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for _, userID := range users {
		<-ticker.C
		b.CopyMessage(userID, fromChatID, messageID)
		b.BroadcastStatus.Processed++
	}
	b.BroadcastStatus.IsRunning = false
}

func (b *Bot) IsAdmin(id int64) bool {
	return id == b.Config.AdminID
}

func (b *Bot) ApproveRequest(chatID, userID int64) error {
	url := b.BaseURL + "/approveChatJoinRequest"
	data := models.ApproveChatJoinRequest{
		ChatID: chat_id_to_string(chatID),
		UserID: userID,
	}
	resp, err := b.callTelegram(url, data)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (b *Bot) SendMessage(chatID int64, text string, keyboards ...models.InlineKeyboardMarkup) {
	data := models.SendMessageRequest{
		ChatID:    fmt.Sprintf("%d", chatID),
		Text:      text,
		ParseMode: "HTML",
	}
	if len(keyboards) > 0 && len(keyboards[0].InlineKeyboard) > 0 {
		data.ReplyMarkup = &keyboards[0]
	}

	payload, _ := json.Marshal(data)
	resp, err := b.HTTPClient.Post(b.BaseURL+"/sendMessage", "application/json", bytes.NewBuffer(payload))
	if err == nil {
		resp.Body.Close()
	}
}

func (b *Bot) StartBroadcast(text string) {
	users, err := b.DB.GetAllUserIDs()
	if err != nil {
		return
	}

	b.BroadcastStatus.IsRunning = true
	b.BroadcastStatus.Total = len(users)
	b.BroadcastStatus.Processed = 0
	b.BroadcastStatus.StartTime = time.Now()

	ticker := time.NewTicker(40 * time.Millisecond)
	defer ticker.Stop()
	for _, userID := range users {
		<-ticker.C
		b.SendMessage(userID, text)
		b.BroadcastStatus.Processed++
	}
	b.BroadcastStatus.IsRunning = false
}

func (b *Bot) callTelegram(url string, data interface{}) (*http.Response, error) {
	payload, _ := json.Marshal(data)
	return b.HTTPClient.Post(url, "application/json", bytes.NewBuffer(payload))
}

func chat_id_to_string(id int64) string {
	return fmt.Sprintf("%d", id)
}

func (b *Bot) WebhookHandler(w http.ResponseWriter, r *http.Request) {
	var update models.Update
	if err := json.NewDecoder(r.Body).Decode(&update); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	// SPEED-FIX: Handle join requests INSTANTLY by skipping the queue
	if update.ChatJoinRequest != nil {
		go b.HandleJoinRequest(update.ChatJoinRequest)
		w.WriteHeader(http.StatusOK)
		return
	}
	select {
	case b.JobQueue <- update:
		w.WriteHeader(http.StatusOK)
	default:
		w.WriteHeader(http.StatusOK)
	}
}

func (b *Bot) ProcessUpdate(u models.Update) {
	if u.ChatJoinRequest != nil {
		b.HandleJoinRequest(u.ChatJoinRequest)
	} else if u.Message != nil {
		b.HandleMessage(u.Message)
	}
}
