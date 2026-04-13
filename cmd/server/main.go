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
	log.Printf("📋 Config: HTTP_PORT=%s, ProxyEnabled=%v, QueueMaxSize=%d",
		cfg.HTTPPort, cfg.ProxyEnabled, cfg.QueueMaxSize)

	// Создаем очередь
	msgQueue := queue.NewMemoryQueue(cfg.QueueMaxSize)

	// Загружаем сохраненные сообщения
	if err := msgQueue.LoadFromFile(cfg.QueueFilePath); err != nil {
		log.Printf("⚠️ Failed to load queue from file: %v", err)
	} else {
		log.Printf("📦 Loaded %d messages from queue file", msgQueue.Len())
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
		if err := msgQueue.SaveToFile(cfg.QueueFilePath); err != nil {
			log.Printf("❌ Failed to save queue: %v", err)
		} else {
			log.Printf("💾 Queue saved to %s (%d messages)", cfg.QueueFilePath, msgQueue.Len())
		}

		os.Exit(0)
	}()

	// Держим приложение запущенным
	select {}
}
