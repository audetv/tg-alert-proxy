package http

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"strings"

	"github.com/audetv/tg-alert-proxy/internal/app"
	"github.com/audetv/tg-alert-proxy/internal/domain"
)

// Handler обрабатывает HTTP запросы
type Handler struct {
	service *app.Service
}

// NewHandler создает новый Handler
func NewHandler(service *app.Service) *Handler {
	return &Handler{
		service: service,
	}
}

// SendRequest представляет тело запроса на отправку
type SendRequest struct {
	Token  string `json:"token"`
	ChatID string `json:"chat_id"`
	Text   string `json:"text"`
}

// SendResponse представляет ответ на отправку
type SendResponse struct {
	Success   bool   `json:"success"`
	MessageID int    `json:"message_id,omitempty"`
	Queued    bool   `json:"queued,omitempty"`
	Error     string `json:"error,omitempty"`
}

// StatsResponse представляет статистику сервиса
type StatsResponse struct {
	SenderReady bool `json:"sender_ready"`
	QueueSize   int  `json:"queue_size"`
}

// SendAlert обрабатывает запрос на отправку уведомления
// Поддерживает методы:
// - POST /send с JSON body
// - GET /send с query параметрами ?token=xxx&chat_id=@channel&text=hello
// - POST /send с form data
func (h *Handler) SendAlert(w http.ResponseWriter, r *http.Request) {
	var req SendRequest

	// Пробуем распарсить JSON body
	if r.Method == http.MethodPost && strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.sendError(w, "Invalid JSON body", http.StatusBadRequest)
			return
		}
	} else {
		// Парсим из query параметров или form data
		if err := r.ParseForm(); err != nil {
			h.sendError(w, "Failed to parse form", http.StatusBadRequest)
			return
		}

		req.Token = r.FormValue("token")
		req.ChatID = r.FormValue("chat_id")
		req.Text = r.FormValue("text")

		// Если token и chat_id не указаны в форме, пробуем query параметры
		if req.Token == "" {
			req.Token = r.URL.Query().Get("token")
		}
		if req.ChatID == "" {
			req.ChatID = r.URL.Query().Get("chat_id")
		}
		if req.Text == "" {
			// Для GET запросов текст может быть в query
			req.Text = r.URL.Query().Get("text")
		}
	}

	// Валидация
	if req.Token == "" {
		h.sendError(w, "token is required", http.StatusBadRequest)
		return
	}
	if req.ChatID == "" {
		h.sendError(w, "chat_id is required", http.StatusBadRequest)
		return
	}
	if req.Text == "" {
		h.sendError(w, "text is required", http.StatusBadRequest)
		return
	}

	// Создаем сообщение
	msg := &domain.Message{
		Token:  req.Token,
		ChatID: req.ChatID,
		Text:   req.Text,
	}

	// Отправляем через сервис
	result, err := h.service.Send(r.Context(), msg)
	if err != nil {
		log.Printf("❌ Send failed: %v", err)
		h.sendError(w, err.Error(), http.StatusInternalServerError)
		return
	}

	// Формируем ответ
	resp := SendResponse{
		Success:   result.Success,
		MessageID: result.MessageID,
		Queued:    result.Queued,
	}
	if result.Error != nil {
		resp.Error = result.Error.Error()
	}

	h.sendJSON(w, resp, http.StatusOK)
}

// Health обрабатывает health check запросы
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	io.WriteString(w, `{"status":"ok"}`)
}

// Stats обрабатывает запрос статистики
func (h *Handler) Stats(w http.ResponseWriter, r *http.Request) {
	stats := h.service.Stats()

	resp := StatsResponse{
		SenderReady: stats["sender_ready"].(bool),
		QueueSize:   stats["queue_size"].(int),
	}

	h.sendJSON(w, resp, http.StatusOK)
}

// sendJSON отправляет JSON ответ
func (h *Handler) sendJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// sendError отправляет ошибку в JSON формате
func (h *Handler) sendError(w http.ResponseWriter, message string, status int) {
	resp := SendResponse{
		Success: false,
		Error:   message,
	}
	h.sendJSON(w, resp, status)
}

// RegisterRoutes регистрирует все маршруты
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/send", h.SendAlert)
	mux.HandleFunc("/health", h.Health)
	mux.HandleFunc("/stats", h.Stats)
}
