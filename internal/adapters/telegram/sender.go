package telegram

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/audetv/tg-alert-proxy/internal/adapters/mtproto"
	"github.com/audetv/tg-alert-proxy/internal/domain"
	"github.com/audetv/tg-alert-proxy/internal/domain/ports"
)

// Sender реализует интерфейс ports.TelegramSender через MTProto
type Sender struct {
	client *mtproto.Client
	ready  bool
}

// NewSender создает новый Sender
func NewSender(cfg *mtproto.Config) (*Sender, error) {
	client, err := mtproto.New(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create mtproto client: %w", err)
	}

	return &Sender{
		client: client,
	}, nil
}

// Connect подключает клиент к Telegram
func (s *Sender) Connect(ctx context.Context) error {
	log.Printf("🔌 Connecting MTProto client...")
	if err := s.client.Connect(ctx); err != nil {
		return fmt.Errorf("connect failed: %w", err)
	}
	s.ready = true
	log.Printf("✅ MTProto client connected")
	return nil
}

// Send отправляет сообщение в Telegram
func (s *Sender) Send(ctx context.Context, msg *domain.Message) (*domain.SendResult, error) {
	if !s.ready {
		return nil, fmt.Errorf("sender not ready")
	}

	start := time.Now()

	messageID, err := s.client.SendMessage(ctx, msg.Token, msg.ChatID, msg.Text)
	if err != nil {
		return &domain.SendResult{
			Success: false,
			Error:   err,
		}, err
	}

	log.Printf("✅ Message sent to %s in %v", msg.ChatID, time.Since(start))

	return &domain.SendResult{
		MessageID: messageID,
		Success:   true,
	}, nil
}

// IsReady возвращает готовность сендера
func (s *Sender) IsReady() bool {
	return s.ready && s.client.IsReady()
}

// Close закрывает сендер
func (s *Sender) Close() error {
	if s.client != nil {
		return s.client.Close()
	}
	return nil
}

// Проверяем что реализует интерфейс
var _ ports.TelegramSender = (*Sender)(nil)
