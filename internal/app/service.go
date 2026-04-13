package app

import (
	"context"
	"log"
	"time"

	"github.com/audetv/tg-alert-proxy/internal/domain"
	"github.com/audetv/tg-alert-proxy/internal/domain/ports"
)

// Service реализует бизнес-логику отправки уведомлений
type Service struct {
	sender ports.TelegramSender
	queue  ports.MessageQueue
	stopCh chan struct{}
}

// NewService создает новый сервис
func NewService(sender ports.TelegramSender, queue ports.MessageQueue) *Service {
	s := &Service{
		sender: sender,
		queue:  queue,
		stopCh: make(chan struct{}),
	}

	// Запускаем воркер для обработки очереди
	go s.queueWorker()

	return s
}

// Send отправляет сообщение. Если сендер не готов - ставит в очередь
func (s *Service) Send(ctx context.Context, msg *domain.Message) (*domain.SendResult, error) {
	msg.CreatedAt = time.Now()

	if s.sender.IsReady() {
		log.Printf("📤 Sending message directly to %s", msg.ChatID)
		return s.sender.Send(ctx, msg)
	}

	// Сендер не готов - ставим в очередь
	log.Printf("⏳ Sender not ready, queueing message for %s (queue size: %d)",
		msg.ChatID, s.queue.Len()+1)

	if err := s.queue.Push(msg); err != nil {
		return nil, err
	}

	return &domain.SendResult{
		Success: false,
		Queued:  true,
	}, nil
}

// queueWorker обрабатывает очередь когда сендер становится готов
func (s *Service) queueWorker() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.processQueue()
		}
	}
}

// processQueue обрабатывает все сообщения из очереди
func (s *Service) processQueue() {
	if !s.sender.IsReady() {
		return
	}

	for {
		msg, err := s.queue.Pop(context.Background())
		if err != nil {
			log.Printf("⚠️ Failed to pop from queue: %v", err)
			return
		}
		if msg == nil {
			// Очередь пуста
			return
		}

		log.Printf("📤 Processing queued message for %s", msg.ChatID)

		result, err := s.sender.Send(context.Background(), msg)
		if err != nil {
			log.Printf("❌ Failed to send queued message: %v, requeueing", err)
			// Возвращаем в очередь для повторной попытки
			if err := s.queue.Push(msg); err != nil {
				log.Printf("❌ Failed to requeue message: %v", err)
			}
			return // Ждем следующего тика
		}

		log.Printf("✅ Queued message sent: ID=%d", result.MessageID)
	}
}

// Stats возвращает статистику сервиса
func (s *Service) Stats() map[string]interface{} {
	return map[string]interface{}{
		"sender_ready": s.sender.IsReady(),
		"queue_size":   s.queue.Len(),
	}
}

// Stop останавливает сервис
func (s *Service) Stop() {
	close(s.stopCh)
}
