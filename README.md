# tg-alert-proxy

[![Docker Build and Publish](https://github.com/audetv/tg-alert-proxy/actions/workflows/docker-publish.yml/badge.svg)](https://github.com/audetv/tg-alert-proxy/actions/workflows/docker-publish.yml)
[![Go Version](https://img.shields.io/badge/Go-1.23-blue.svg)](https://go.dev/)

**Лёгкий HTTP-прокси для отправки уведомлений в Telegram через MTProto в обход блокировок.**

Сервис принимает привычные HTTP-запросы (как в Zabbix, скриптах мониторинга) и отправляет их в Telegram через надёжное MTProto-соединение, используя встроенный прокси `tg-ws-proxy`.

---

## 🚀 Быстрый старт

### 1. Создайте `.env` файл с секретом для прокси

```bash
# Сгенерируйте случайный секрет
echo "TG_WS_PROXY_SECRET=$(openssl rand -hex 16)" > .env
```

### 2. Скачайте `docker-compose.yml`

```yaml
services:
  tg-ws-proxy:
    image: ghcr.io/audetv/tg-ws-proxy:latest
    container_name: tg-ws-proxy
    environment:
      - TG_WS_PROXY_HOST=0.0.0.0
      - TG_WS_PROXY_PORT=1443
      - TG_WS_PROXY_DC_IPS=2:149.154.167.220 4:149.154.167.220
      - TG_WS_PROXY_SECRET=${TG_WS_PROXY_SECRET}
    networks:
      - alert-net
    restart: unless-stopped

  tg-alert-proxy:
    image: ghcr.io/audetv/tg-alert-proxy:latest
    container_name: tg-alert-proxy
    ports:
      - "${HTTP_PORT:-8080}:8080"
    environment:
      - HTTP_PORT=8080
      - TG_WS_PROXY_ENABLED=${TG_WS_PROXY_ENABLED:-true}
      - TG_WS_PROXY_ADDR=${TG_WS_PROXY_ADDR:-tg-ws-proxy:1443}
      - TG_WS_PROXY_SECRET=${TG_WS_PROXY_SECRET}
      - TG_APP_ID=${TG_APP_ID:-2040}
      - TG_APP_HASH=${TG_APP_HASH:-b18441a1ff607e10a989891a5462e627}
      - QUEUE_MAX_SIZE=${QUEUE_MAX_SIZE:-100}
      - QUEUE_DIR=/data
      - QUEUE_FILE_NAME=queue.json
      - SESSION_DIR=/data
    volumes:
      - ./data:/data
    networks:
      - alert-net
    depends_on:
      - tg-ws-proxy
    restart: unless-stopped

networks:
  alert-net:
    driver: bridge
```

### 3. Запустите сервис

```bash
docker compose up -d
```

### 4. Отправьте тестовое уведомление

```bash
curl -X POST http://localhost:8080/send \
  -H "Content-Type: application/json" \
  -d '{
    "token": "7685586471:AA...ваш_токен_бота",
    "chat_id": "@your_channel",
    "text": "🚀 Тестовое сообщение!"
  }'
```

---

## 📨 API

### `POST /send`

Отправляет сообщение в Telegram.

**Параметры запроса (JSON):**

| Поле      | Тип    | Обязательное | Описание                                |
|-----------|--------|--------------|-----------------------------------------|
| `token`   | string | Да           | Токен бота Telegram                     |
| `chat_id` | string | Да           | ID канала/группы или username (с `@`)   |
| `text`    | string | Да           | Текст сообщения                         |

**Пример ответа:**
```json
{
  "success": true,
  "message_id": 12345
}
```

При недоступности Telegram сообщение ставится в очередь:
```json
{
  "success": false,
  "queued": true
}
```

### `GET /health`

Проверка работоспособности сервиса.

```bash
curl http://localhost:8080/health
# {"status":"ok"}
```

### `GET /stats`

Статистика сервиса.

```bash
curl http://localhost:8080/stats
# {"sender_ready":true,"queue_size":0}
```

---

## ⚙️ Переменные окружения

| Переменная                | По умолчанию                            | Описание                                                                 |
|---------------------------|-----------------------------------------|---------------------------------------------------------------------------|
| `HTTP_PORT`               | `8080`                                  | Порт HTTP-сервера                                                         |
| `TG_WS_PROXY_ENABLED`     | `true`                                  | Использовать MTProto-прокси                                               |
| `TG_WS_PROXY_ADDR`        | `tg-ws-proxy:1443`                      | Адрес прокси                                                              |
| `TG_WS_PROXY_SECRET`      | *обязательно при включенном прокси*     | Секретный ключ прокси (32 hex символа)                                    |
| `TG_APP_ID`               | `2040`                                  | Telegram API app_id (можно получить на [my.telegram.org](https://my.telegram.org)) |
| `TG_APP_HASH`             | `b18441a1ff607e10a989891a5462e627`      | Telegram API app_hash                                                     |
| `QUEUE_MAX_SIZE`          | `100`                                   | Максимальный размер очереди сообщений                                     |
| `QUEUE_DIR`               | `/data`                                 | Директория для хранения очереди                                           |
| `QUEUE_FILE_NAME`         | `queue.json`                            | Имя файла очереди                                                         |
| `SESSION_DIR`             | `/data`                                 | Директория для хранения MTProto-сессии                                    |

---

## 🔧 Сборка из исходников

```bash
# Клонируйте репозиторий
git clone https://github.com/audetv/tg-alert-proxy.git
cd tg-alert-proxy

# Соберите бинарник
make build

# Запустите (требуется прокси или прямой доступ к Telegram)
./build/tg-alert-proxy
```

---

## ❗ Устранение неполадок

### 1. Ошибка `CHANNEL_INVALID` или `AccessHash missing`

Бот не может отправить сообщение в приватный канал, так как у него нет `AccessHash`.  
**Решение:** отправьте **любое сообщение в этот канал вручную** (через обычный Telegram-клиент). Бот получит апдейт, и `AccessHash` сохранится в файле сессии (`/data/mtproto_session.json`). После этого отправка заработает.

### 2. Сервис не может подключиться к прокси

Проверьте, что `tg-ws-proxy` запущен и доступен по адресу, указанному в `TG_WS_PROXY_ADDR`.  
Убедитесь, что секреты совпадают в обоих сервисах.

### 3. Сообщения ставятся в очередь и не отправляются

- Проверьте статус сендера: `curl http://localhost:8080/stats`
- Убедитесь, что бот добавлен в канал/группу и имеет права на отправку сообщений.
- Проверьте логи: `docker compose logs -f tg-alert-proxy`

---

## 📦 Используемые образы

- **tg-alert-proxy**: [`ghcr.io/audetv/tg-alert-proxy`](https://github.com/audetv/tg-alert-proxy/pkgs/container/tg-alert-proxy)
- **tg-ws-proxy**: [`ghcr.io/audetv/tg-ws-proxy`](https://github.com/audetv/tg-ws-proxy/pkgs/container/tg-ws-proxy)

---

## 📄 Лицензия

[MIT License](LICENSE)
