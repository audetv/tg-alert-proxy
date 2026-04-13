package mtproto

import (
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"strconv"
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
}

// Config конфигурация MTProto клиента
type Config struct {
	ProxyEnabled bool
	ProxyAddr    string
	ProxySecret  string
	AppID        int
	AppHash      string
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
		client: client,
	}, nil
}

// Connect подключается к Telegram и авторизует бота
func (c *Client) Connect(parentCtx context.Context, token string) error {
	ctx, cancel := context.WithCancel(parentCtx)
	c.cancel = cancel

	return c.client.Run(ctx, func(ctx context.Context) error {
		start := time.Now()
		log.Printf("🔐 MTProto Run started")

		// Проверяем статус
		status, err := c.client.Auth().Status(ctx)
		if err != nil {
			log.Printf("⚠️ Auth status check failed: %v", err)
		} else {
			log.Printf("📊 Auth status: authorized=%v", status.Authorized)
		}

		// Авторизуем бота
		log.Printf("🔐 Authenticating bot...")
		authStart := time.Now()
		if _, err := c.client.Auth().Bot(ctx, token); err != nil {
			log.Printf("❌ Auth failed after %v: %v", time.Since(authStart), err)
			return fmt.Errorf("auth failed: %w", err)
		}
		log.Printf("✅ Auth completed in %v", time.Since(authStart))

		c.api = c.client.API()
		c.sender = message.NewSender(c.api)

		c.mu.Lock()
		c.ready = true
		c.mu.Unlock()

		log.Printf("✅ MTProto client ready in %v", time.Since(start))

		<-ctx.Done()
		return ctx.Err()
	})
}

// SendMessage отправляет простое текстовое сообщение
func (c *Client) SendMessage(ctx context.Context, token, chatID, text string) (int, error) {
	c.mu.RLock()
	ready := c.ready
	c.mu.RUnlock()

	if !ready {
		return 0, fmt.Errorf("client not ready")
	}

	// Временно авторизуемся с переданным токеном, если он отличается
	// TODO: поддержка множества токенов

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

// resolvePeer определяет peer по chatID (username или ID)
func (c *Client) resolvePeer(ctx context.Context, chatID string) (tg.InputPeerClass, error) {
	// Если начинается с @ - это username
	if len(chatID) > 0 && chatID[0] == '@' {
		username := chatID[1:]
		resolved, err := c.api.ContactsResolveUsername(ctx, &tg.ContactsResolveUsernameRequest{
			Username: username,
		})
		if err != nil {
			return nil, err
		}
		if len(resolved.Chats) > 0 {
			if ch, ok := resolved.Chats[0].(*tg.Channel); ok {
				return &tg.InputPeerChannel{
					ChannelID:  ch.ID,
					AccessHash: ch.AccessHash,
				}, nil
			}
		}
		return nil, fmt.Errorf("chat not found: %s", chatID)
	}

	// Иначе пробуем как числовой ID
	id, err := strconv.ParseInt(chatID, 10, 64)
	if err != nil {
		return nil, fmt.Errorf("invalid chat ID: %w", err)
	}
	return &tg.InputPeerChannel{ChannelID: id}, nil
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
