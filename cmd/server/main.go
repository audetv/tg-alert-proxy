package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"

	"github.com/audetv/tg-alert-proxy/internal/adapters/mtproto"
	"github.com/audetv/tg-alert-proxy/internal/adapters/queue"
	"github.com/audetv/tg-alert-proxy/internal/adapters/telegram"
	"github.com/audetv/tg-alert-proxy/internal/app"
	"github.com/audetv/tg-alert-proxy/internal/config"
)

func init() {
	// Загружаем .env файл если существует
	if err := godotenv.Load(); err != nil {
		log.Printf("⚠️ No .env file found, using environment variables")
	}
}

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

	// Создаем MTProto сендер
	mtprotoCfg := &mtproto.Config{
		ProxyEnabled: cfg.ProxyEnabled,
		ProxyAddr:    cfg.ProxyAddr,
		ProxySecret:  cfg.ProxySecret,
		AppID:        cfg.AppID,
		AppHash:      cfg.AppHash,
	}

	sender, err := telegram.NewSender(mtprotoCfg)
	if err != nil {
		log.Fatalf("❌ Failed to create sender: %v", err)
	}

	// Подключаем сендер
	ctx := context.Background()
	if err := sender.Connect(ctx); err != nil {
		log.Printf("⚠️ Failed to connect sender: %v (will retry)", err)
		// Не фатально, сервис продолжит работу и будет ставить сообщения в очередь
	}
	defer sender.Close()

	// Создаем сервис
	service := app.NewService(sender, msgQueue)

	log.Printf("✅ Service ready, sender_ready=%v, queue_size=%d",
		sender.IsReady(), msgQueue.Len())

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigChan
		log.Println("🛑 Shutting down...")

		// Останавливаем сервис
		service.Stop()

		// Сохраняем очередь
		if err := msgQueue.SaveToFile(queuePath); err != nil {
			log.Printf("❌ Failed to save queue: %v", err)
		} else {
			log.Printf("💾 Queue saved to %s (%d messages)", queuePath, msgQueue.Len())
		}

		os.Exit(0)
	}()

	// Держим приложение запущенным (пока без HTTP сервера)
	select {}
}
