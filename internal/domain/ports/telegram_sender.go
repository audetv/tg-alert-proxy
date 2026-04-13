package ports

import (
	"context"

	"github.com/audetv/tg-alert-proxy/internal/domain"
)

// TelegramSender интерфейс для отправки сообщений в Telegram
type TelegramSender interface {
	Send(ctx context.Context, msg *domain.Message) (*domain.SendResult, error)
	IsReady() bool
	Close() error
}
