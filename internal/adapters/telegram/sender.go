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

// Connect подключает клиент к Telegram (не блокирует)
func (s *Sender) Connect(ctx context.Context) error {
	log.Printf("🔌 Starting MTProto client connection...")

	// Запускаем подключение в горутине
	go func() {
		if err := s.client.Connect(ctx); err != nil {
			log.Printf("❌ MTProto connect failed: %v", err)
			return
		}
		s.ready = true
		log.Printf("✅ MTProto client connected")
	}()

	// Ждем готовности до 30 секунд
	for i := 0; i < 60; i++ {
		if s.client.IsReady() {
			s.ready = true
			log.Printf("✅ MTProto client ready")
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}

	log.Printf("⚠️ MTProto client not ready after 30s, will retry in background")
	return nil // Не возвращаем ошибку, сервис продолжит работу
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
