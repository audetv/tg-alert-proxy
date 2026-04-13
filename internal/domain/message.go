package domain

import "time"

// Message представляет сообщение для отправки в Telegram
type Message struct {
	Token     string    `json:"token"`
	ChatID    string    `json:"chat_id"`
	Text      string    `json:"text"`
	CreatedAt time.Time `json:"created_at"`
}

// SendResult результат отправки сообщения
type SendResult struct {
	MessageID int   `json:"message_id,omitempty"`
	Success   bool  `json:"success"`
	Queued    bool  `json:"queued,omitempty"`
	Error     error `json:"-"`
}
