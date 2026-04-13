package mtproto

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gotd/td/telegram"
	"github.com/gotd/td/telegram/dcs"
	"github.com/gotd/td/telegram/message"
	"github.com/gotd/td/tg"
)

// Client MTProto клиент для отправки сообщений
type Client struct {
	client *telegram.Client
	api    *tg.Client
	sender *message.Sender
	mu     sync.RWMutex
	ready  bool
	cancel context.CancelFunc

	// Кеш авторизованных токенов
	authCache map[string]bool
	authMu    sync.RWMutex
}

// Config конфигурация MTProto клиента
type Config struct {
	ProxyEnabled bool
	ProxyAddr    string
	ProxySecret  string
	AppID        int
	AppHash      string
	SessionDir   string
}

// New создает новый MTProto клиент
func New(cfg *Config) (*Client, error) {
	var resolver dcs.Resolver

	if cfg.ProxyEnabled {
		proxyAddr := cfg.ProxyAddr
		if proxyAddr == "" {
			proxyAddr = "tg-ws-proxy:1443"
		}

		secret := cfg.ProxySecret
		if secret == "" {
			return nil, fmt.Errorf("proxy secret is required when proxy enabled")
		}

		log.Printf("🔄 MTProto proxy enabled: %s", proxyAddr)

		secretBytes, err := hex.DecodeString(secret)
		if err != nil {
			return nil, fmt.Errorf("invalid secret hex: %w", err)
		}

		mtProxy, err := dcs.MTProxy(proxyAddr, secretBytes, dcs.MTProxyOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to create MTProxy resolver: %w", err)
		}
		resolver = mtProxy

		log.Printf("✅ MTProto resolver configured")
	} else {
		log.Printf("📡 Direct MTProto connection")
		resolver = dcs.Plain(dcs.PlainOptions{})
	}

	client := telegram.NewClient(cfg.AppID, cfg.AppHash, telegram.Options{
		Resolver: resolver,
		DC:       2, // Основной DC для ботов
	})

	return &Client{
		client:    client,
		authCache: make(map[string]bool),
	}, nil
}

// Connect подключается к Telegram и ждёт готовности
func (c *Client) Connect(parentCtx context.Context) error {
	ctx, cancel := context.WithCancel(parentCtx)
	c.cancel = cancel

	// Канал для сигнала готовности
	readyCh := make(chan struct{})

	go func() {
		if err := c.client.Run(ctx, func(ctx context.Context) error {
			start := time.Now()
			log.Printf("🔐 MTProto Run started")

			c.api = c.client.API()
			c.sender = message.NewSender(c.api)

			c.mu.Lock()
			c.ready = true
			c.mu.Unlock()

			close(readyCh) // Сигнализируем о готовности

			log.Printf("✅ MTProto client ready in %v", time.Since(start))

			<-ctx.Done()
			return ctx.Err()
		}); err != nil {
			log.Printf("❌ MTProto Run failed: %v", err)
			c.mu.Lock()
			c.ready = false
			c.mu.Unlock()
		}
	}()

	// Ждём готовности или таймаута
	select {
	case <-readyCh:
		log.Printf("✅ MTProto client connected and ready")
		return nil
	case <-time.After(30 * time.Second):
		return fmt.Errorf("timeout waiting for MTProto client to be ready")
	case <-ctx.Done():
		return ctx.Err()
	}
}

// fetchUpdates получает диалоги для прогрева кеша AccessHash
func (c *Client) fetchUpdates(ctx context.Context) {
	log.Printf("🔥 Warming up cache: fetching dialogs...")

	// Правильный запрос с пустым offset_peer
	dialogs, err := c.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
		OffsetPeer: &tg.InputPeerEmpty{}, // Пустой peer вместо nil
		Limit:      100,
	})
	if err != nil {
		log.Printf("⚠️ Failed to fetch dialogs: %v", err)
		return
	}

	// Логируем найденные чаты
	if dialogsSlice, ok := dialogs.(*tg.MessagesDialogsSlice); ok {
		log.Printf("📡 Found %d chats in dialogs", len(dialogsSlice.Chats))
		for _, chat := range dialogsSlice.Chats {
			if ch, ok := chat.(*tg.Channel); ok {
				log.Printf("   📌 Channel: %s (ID: %d, AccessHash: %d)",
					ch.Title, ch.ID, ch.AccessHash)
			}
			if ch, ok := chat.(*tg.Chat); ok {
				log.Printf("   👥 Group: %s (ID: %d)", ch.Title, ch.ID)
			}
		}
	} else if dialogsClass, ok := dialogs.(*tg.MessagesDialogs); ok {
		log.Printf("📡 Found %d chats", len(dialogsClass.Chats))
		for _, chat := range dialogsClass.Chats {
			if ch, ok := chat.(*tg.Channel); ok {
				log.Printf("   📌 Channel: %s (ID: %d, AccessHash: %d)",
					ch.Title, ch.ID, ch.AccessHash)
			}
		}
	}

	log.Printf("✅ Cache warmup completed")
}

// ensureAuth проверяет и при необходимости выполняет авторизацию с токеном
func (c *Client) ensureAuth(ctx context.Context, token string) error {
	// Проверяем кеш
	c.authMu.RLock()
	if c.authCache[token] {
		c.authMu.RUnlock()
		return nil
	}
	c.authMu.RUnlock()

	// Выполняем авторизацию
	log.Printf("🔐 Authorizing bot with token: %s...", token[:min(10, len(token))])

	if _, err := c.client.Auth().Bot(ctx, token); err != nil {
		// Проверяем, не авторизованы ли уже
		if strings.Contains(err.Error(), "already authorized") ||
			strings.Contains(err.Error(), "Unauthorized") == false {
			// Сохраняем в кеш даже при некоторых ошибках
			c.authMu.Lock()
			c.authCache[token] = true
			c.authMu.Unlock()
			log.Printf("✅ Bot already authorized")
			return nil
		}
		return fmt.Errorf("auth failed: %w", err)
	}

	// Сохраняем в кеш
	c.authMu.Lock()
	c.authCache[token] = true
	c.authMu.Unlock()

	log.Printf("✅ Bot authorized successfully")
	return nil
}

// SendMessage отправляет простое текстовое сообщение
func (c *Client) SendMessage(ctx context.Context, token, chatID, text string) (int, error) {
	c.mu.RLock()
	ready := c.ready
	c.mu.RUnlock()

	if !ready {
		return 0, fmt.Errorf("client not ready")
	}

	// Авторизуемся с токеном (использует кеш)
	if err := c.ensureAuth(ctx, token); err != nil {
		return 0, fmt.Errorf("auth failed: %w", err)
	}

	peer, err := c.resolvePeer(ctx, chatID)
	if err != nil {
		return 0, fmt.Errorf("resolve peer failed: %w", err)
	}

	start := time.Now()
	result, err := c.sender.To(peer).Text(ctx, text)
	if err != nil {
		return 0, fmt.Errorf("send failed: %w", err)
	}
	log.Printf("📤 Message sent in %v", time.Since(start))

	return extractMessageID(result), nil
}

func (c *Client) resolvePeer(ctx context.Context, chatID string) (tg.InputPeerClass, error) {
	// 1. Обработка username (@name)
	if len(chatID) > 0 && chatID[0] == '@' {
		username := chatID[1:]
		resolved, err := c.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: username,
		})
		if err != nil {
			return nil, fmt.Errorf("resolve username @%s: %w", username, err)
		}

		for _, chat := range resolved.Chats {
			if ch, ok := chat.(*tg.Channel); ok {
				log.Printf("📡 Resolved @%s -> channel %d with AccessHash", username, ch.ID)
				return &tg.InputPeerChannel{
					ChannelID:  ch.ID,
					AccessHash: ch.AccessHash,
				}, nil
			}
		}

		return nil, fmt.Errorf("peer not found: %s", chatID)
	}

	// 2. Обработка числового ID
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}

	// Для каналов (-100...)
	if strings.HasPrefix(chatID, "-100") {
		// Сначала пробуем найти в диалогах
		dialogs, err := c.api.MessagesGetDialogs(ctx, &tg.MessagesGetDialogsRequest{
			OffsetPeer: &tg.InputPeerEmpty{},
			Limit:      100,
		})
		if err == nil {
			if dialogsSlice, ok := dialogs.(*tg.MessagesDialogsSlice); ok {
				for _, chat := range dialogsSlice.Chats {
					if ch, ok := chat.(*tg.Channel); ok && ch.ID == id {
						log.Printf("📡 Found channel %d in cached dialogs, using AccessHash", id)
						return &tg.InputPeerChannel{
							ChannelID:  ch.ID,
							AccessHash: ch.AccessHash,
						}, nil
					}
				}
			}
		}

		// Если не нашли в кеше — пробуем запросить напрямую
		inputChannel := &tg.InputChannel{
			ChannelID:  id,
			AccessHash: 0,
		}

		channels, err := c.api.ChannelsGetChannels(ctx, []tg.InputChannelClass{inputChannel})
		if err != nil {
			return nil, fmt.Errorf("channel %d not accessible: %w", id, err)
		}

		for _, chat := range channels.GetChats() {
			if ch, ok := chat.(*tg.Channel); ok {
				log.Printf("📡 Got AccessHash for channel %d from API", id)
				return &tg.InputPeerChannel{
					ChannelID:  ch.ID,
					AccessHash: ch.AccessHash,
				}, nil
			}
		}

		return nil, fmt.Errorf("channel %d not accessible: AccessHash missing", id)
	}

	// Для обычных групп (отрицательный ID без -100)
	if id < 0 {
		return &tg.InputPeerChat{ChatID: -id}, nil
	}

	return nil, fmt.Errorf("cannot resolve user %d without AccessHash", id)
}

// IsReady возвращает готовность клиента
func (c *Client) IsReady() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.ready
}

// Close закрывает клиент
func (c *Client) Close() error {
	if c.cancel != nil {
		c.cancel()
	}
	return nil
}

// extractMessageID извлекает ID сообщения из ответа API
func extractMessageID(result tg.UpdatesClass) int {
	switch update := result.(type) {
	case *tg.UpdateShortSentMessage:
		return update.ID
	case *tg.Updates:
		for _, upd := range update.Updates {
			if msgUpdate, ok := upd.(*tg.UpdateMessageID); ok {
				return msgUpdate.ID
			}
			if newMsg, ok := upd.(*tg.UpdateNewMessage); ok {
				if msg, ok := newMsg.Message.(*tg.Message); ok {
					return msg.ID
				}
			}
		}
	}
	return 0
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
