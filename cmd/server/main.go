package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/audetv/tg-alert-proxy/internal/config"
)

func main() {
	cfg := config.Load()

	log.Printf("🚀 tg-alert-proxy starting...")
	log.Printf("📋 Config: HTTP_PORT=%s, ProxyEnabled=%v, QueueMaxSize=%d",
		cfg.HTTPPort, cfg.ProxyEnabled, cfg.QueueMaxSize)

	// Graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	<-sigChan
	log.Println("🛑 Shutting down...")
}
