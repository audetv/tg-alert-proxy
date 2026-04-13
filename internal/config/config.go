package config

import (
	"os"
	"strconv"
)

// Config хранит все настройки приложения
type Config struct {
	HTTPPort       string
	ProxyEnabled   bool
	ProxyAddr      string
	ProxySecret    string
	AppID          int
	AppHash        string
	QueueMaxSize   int
	QueueDir       string
	QueueFileName  string
	MetricsEnabled bool
	SessionDir     string
}

// Load загружает конфигурацию из переменных окружения
func Load() *Config {
	return &Config{
		HTTPPort:       getEnv("HTTP_PORT", "8080"),
		ProxyEnabled:   getEnvBool("TG_WS_PROXY_ENABLED", true),
		ProxyAddr:      getEnv("TG_WS_PROXY_ADDR", "tg-ws-proxy:1443"),
		ProxySecret:    getEnv("TG_WS_PROXY_SECRET", ""),
		AppID:          getEnvInt("TG_APP_ID", 2040),
		AppHash:        getEnv("TG_APP_HASH", "b18441a1ff607e10a989891a5462e627"),
		QueueMaxSize:   getEnvInt("QUEUE_MAX_SIZE", 100),
		QueueDir:       getEnv("QUEUE_DIR", "./data"),
		QueueFileName:  getEnv("QUEUE_FILE_NAME", "queue.json"),
		MetricsEnabled: getEnvBool("METRICS_ENABLED", true),
		SessionDir:     getEnv("SESSION_DIR", "/data"),
	}
}

// QueueFilePath возвращает полный путь к файлу очереди
func (c *Config) QueueFilePath() string {
	if c.QueueDir == "" {
		return c.QueueFileName
	}
	return c.QueueDir + "/" + c.QueueFileName
}

func getEnv(key, defaultVal string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) int {
	if val := os.Getenv(key); val != "" {
		if i, err := strconv.Atoi(val); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvBool(key string, defaultVal bool) bool {
	if val := os.Getenv(key); val != "" {
		if b, err := strconv.ParseBool(val); err == nil {
			return b
		}
	}
	return defaultVal
}
