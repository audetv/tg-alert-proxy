package http

import (
	"context"
	"log"
	"net/http"
	"time"
)

// Server представляет HTTP сервер
type Server struct {
	server *http.Server
}

// NewServer создает новый HTTP сервер
func NewServer(port string, handler *Handler) *Server {
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	// Добавляем middleware для логирования
	loggedMux := loggingMiddleware(mux)

	return &Server{
		server: &http.Server{
			Addr:         ":" + port,
			Handler:      loggedMux,
			ReadTimeout:  10 * time.Second,
			WriteTimeout: 30 * time.Second,
			IdleTimeout:  60 * time.Second,
		},
	}
}

// Start запускает HTTP сервер
func (s *Server) Start() error {
	log.Printf("🌐 HTTP server listening on %s", s.server.Addr)
	return s.server.ListenAndServe()
}

// Shutdown gracefully останавливает сервер
func (s *Server) Shutdown(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}

// loggingMiddleware логирует все запросы
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Пропускаем health checks чтобы не спамить
		if r.URL.Path != "/health" {
			log.Printf("📥 %s %s", r.Method, r.URL.Path)
		}

		next.ServeHTTP(w, r)

		if r.URL.Path != "/health" {
			log.Printf("📤 %s %s completed in %v", r.Method, r.URL.Path, time.Since(start))
		}
	})
}
