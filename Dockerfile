# syntax=docker/dockerfile:1.7

FROM golang:1.25-alpine AS builder

# Устанавливаем tzdata для zoneinfo
RUN apk add --no-cache tzdata

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Аргументы для версионирования
ARG VERSION=dev
ARG COMMIT=none
ARG BUILD_TIME=unknown

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -a -tags netgo \
    -ldflags "-w -extldflags '-static' \
              -X github.com/audetv/tg-alert-proxy/internal/version.Version=${VERSION} \
              -X github.com/audetv/tg-alert-proxy/internal/version.Commit=${COMMIT} \
              -X github.com/audetv/tg-alert-proxy/internal/version.BuildTime=${BUILD_TIME}" \
    -o tg-alert-proxy ./cmd/server

# Финальный образ
FROM scratch

WORKDIR /app

COPY --from=builder /app/tg-alert-proxy /app/tg-alert-proxy
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo

ENV TZ=Europe/Moscow

EXPOSE 8080

VOLUME ["/data"]

ENTRYPOINT ["/app/tg-alert-proxy"]