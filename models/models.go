package models

import (
	"time"
)

// MongoDB Models
type User struct {
	UserID        int64     `bson:"user_id"`
	Username      string    `bson:"username"`
	FirstSeen     time.Time `bson:"first_seen"`
	SourceChannel int64     `bson:"source_channel"`
}

type Channel struct {
	ChannelID int64  `bson:"channel_id"`
	Title     string `bson:"title"`
}

// Telegram API Types
type Update struct {
	UpdateID        int64            `json:"update_id"`
	Message         *Message         `json:"message,omitempty"`
	ChatJoinRequest *ChatJoinRequest `json:"chat_join_request,omitempty"`
}

type Message struct {
	MessageID      int64    `json:"message_id"`
	From           UserTG   `json:"from"`
	Text           string   `json:"text,omitempty"`
	ReplyToMessage *Message `json:"reply_to_message,omitempty"`
}

type ChatJoinRequest struct {
	Chat       ChatTG      `json:"chat"`
	From       UserTG      `json:"from"`
	InviteLink interface{} `json:"invite_link,omitempty"`
}

type UserTG struct {
	ID       int64  `json:"id"`
	Username string `json:"username,omitempty"`
}

type ChatTG struct {
	ID    int64  `json:"id"`
	Title string `json:"title,omitempty"`
}

type InlineKeyboardMarkup struct {
	InlineKeyboard [][]InlineKeyboardButton `json:"inline_keyboard"`
}

type InlineKeyboardButton struct {
	Text string `json:"text"`
	URL  string `json:"url,omitempty"`
}

// Request Types for Speed Optimization
type ApproveChatJoinRequest struct {
	ChatID string `json:"chat_id"`
	UserID int64  `json:"user_id"`
}

type SendMessageRequest struct {
	ChatID      string                `json:"chat_id"`
	Text        string                `json:"text"`
	ParseMode   string                `json:"parse_mode,omitempty"`
	ReplyMarkup *InlineKeyboardMarkup `json:"reply_markup,omitempty"`
}
