package queue

import (
	"context"
	"encoding/json"
	"os"
	"sync"

	"github.com/audetv/tg-alert-proxy/internal/domain"
)

// MemoryQueue реализует очередь сообщений в памяти
type MemoryQueue struct {
	messages []*domain.Message
	mu       sync.RWMutex
	maxSize  int
}

// NewMemoryQueue создает новую очередь в памяти
func NewMemoryQueue(maxSize int) *MemoryQueue {
	return &MemoryQueue{
		messages: make([]*domain.Message, 0, maxSize),
		maxSize:  maxSize,
	}
}

// Push добавляет сообщение в очередь
func (q *MemoryQueue) Push(msg *domain.Message) error {
	q.mu.Lock()
	defer q.mu.Unlock()

	// Если очередь переполнена, удаляем самое старое сообщение
	if len(q.messages) >= q.maxSize {
		q.messages = q.messages[1:]
	}

	q.messages = append(q.messages, msg)
	return nil
}

// Pop извлекает сообщение из очереди
func (q *MemoryQueue) Pop(ctx context.Context) (*domain.Message, error) {
	q.mu.Lock()
	defer q.mu.Unlock()

	if len(q.messages) == 0 {
		return nil, nil
	}

	msg := q.messages[0]
	q.messages = q.messages[1:]
	return msg, nil
}

// Len возвращает текущий размер очереди
func (q *MemoryQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.messages)
}

// SaveToFile сохраняет очередь в JSON файл
func (q *MemoryQueue) SaveToFile(path string) error {
	q.mu.RLock()
	defer q.mu.RUnlock()

	// Создаем директорию если не существует
	if err := os.MkdirAll(getDir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(q.messages, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}

// LoadFromFile загружает очередь из JSON файла
func (q *MemoryQueue) LoadFromFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var messages []*domain.Message
	if err := json.Unmarshal(data, &messages); err != nil {
		return err
	}

	q.mu.Lock()
	defer q.mu.Unlock()

	for _, msg := range messages {
		if len(q.messages) < q.maxSize {
			q.messages = append(q.messages, msg)
		}
	}

	return nil
}

// getDir возвращает директорию из пути к файлу
func getDir(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' || path[i] == '\\' {
			return path[:i]
		}
	}
	return "."
}

// Messages возвращает копию всех сообщений (для тестирования)
func (q *MemoryQueue) Messages() []*domain.Message {
	q.mu.RLock()
	defer q.mu.RUnlock()

	result := make([]*domain.Message, len(q.messages))
	copy(result, q.messages)
	return result
}
