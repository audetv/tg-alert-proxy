package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/audetv/tg-alert-proxy/internal/adapters/queue"
	"github.com/audetv/tg-alert-proxy/internal/config"
	"github.com/audetv/tg-alert-proxy/internal/domain"
)

func main() {
	cfg := config.Load()

	log.Printf("🚀 tg-alert-proxy starting...")
	log.Printf("📋 Config: HTTP_PORT=%s, ProxyEnabled=%v, QueueMaxSize=%d, QueuePath=%s",
		cfg.HTTPPort, cfg.ProxyEnabled, cfg.QueueMaxSize, cfg.QueueFilePath())

	// Создаем очередь
	msgQueue := queue.NewMemoryQueue(cfg.QueueMaxSize)

	// Загружаем сохраненные сообщения
	queuePath := cfg.QueueFilePath()
	if err := msgQueue.LoadFromFile(queuePath); err != nil {
		log.Printf("⚠️ Failed to load queue from file: %v", err)
	} else if msgQueue.Len() > 0 {
		log.Printf("📦 Loaded %d messages from %s", msgQueue.Len(), queuePath)
	}

	// Тестовое сообщение
	testMsg := &domain.Message{
		Token:     "test_token",
		ChatID:    "@test_channel",
		Text:      "Test alert message",
		CreatedAt: time.Now(),
	}
	msgQueue.Push(testMsg)
	log.Printf("📝 Queue size: %d", msgQueue.Len())

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Shutting down...")

		// Сохраняем очередь перед выходом
		if err := msgQueue.SaveToFile(queuePath); err != nil {
			log.Printf("❌ Failed to save queue: %v", err)
		} else {
			log.Printf("💾 Queue saved to %s (%d messages)", queuePath, msgQueue.Len())
		}

		os.Exit(0)
	}()

	// Держим приложение запущенным
	select {}
}
