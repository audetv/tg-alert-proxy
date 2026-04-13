package ports

import (
	"context"

	"github.com/audetv/tg-alert-proxy/internal/domain"
)

// MessageQueue интерфейс очереди сообщений
type MessageQueue interface {
	Push(msg *domain.Message) error
	Pop(ctx context.Context) (*domain.Message, error)
	Len() int
	SaveToFile(path string) error
	LoadFromFile(path string) error
}
