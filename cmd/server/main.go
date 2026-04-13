package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	httpadapter "github.com/audetv/tg-alert-proxy/internal/adapters/http"
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

	// Создаем сервис
	service := app.NewService(sender, msgQueue)

	// Создаем HTTP обработчик и сервер
	handler := httpadapter.NewHandler(service)
	server := httpadapter.NewServer(cfg.HTTPPort, handler)

	// 1. ЗАПУСКАЕМ HTTP СЕРВЕР В ГОРУТИНЕ (не ждём MTProto)
	go func() {
		log.Printf("🌐 HTTP server listening on :%s", cfg.HTTPPort)
		log.Printf("📨 Send alerts via: POST http://localhost:%s/send", cfg.HTTPPort)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("❌ HTTP server failed: %v", err)
		}
	}()

	// 2. ЗАПУСКАЕМ ПОДКЛЮЧЕНИЕ SENDER В ФОНЕ
	go func() {
		log.Printf("🔄 MTProto connecting in background...")
		ctx := context.Background()
		if err := sender.Connect(ctx); err != nil {
			log.Printf("⚠️ Failed to connect sender: %v (will retry)", err)
		}
	}()

	log.Printf("✅ Service initialized, sender_ready=%v, queue_size=%d",
		sender.IsReady(), msgQueue.Len())

	// 3. ОЖИДАЕМ СИГНАЛ ЗАВЕРШЕНИЯ
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("🛑 Shutting down...")

	// Останавливаем сервис
	service.Stop()

	// Останавливаем HTTP сервер
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Printf("⚠️ HTTP server shutdown error: %v", err)
	}

	// Закрываем sender
	if err := sender.Close(); err != nil {
		log.Printf("⚠️ Sender close error: %v", err)
	}

	// Сохраняем очередь
	if err := msgQueue.SaveToFile(queuePath); err != nil {
		log.Printf("❌ Failed to save queue: %v", err)
	} else {
		log.Printf("💾 Queue saved to %s (%d messages)", queuePath, msgQueue.Len())
	}

	log.Println("👋 Goodbye!")
}
