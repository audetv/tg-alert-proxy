.PHONY: build run test clean docker-build docker-up docker-down docker-logs gen-secret version

BINARY_NAME=tg-alert-proxy
BUILD_DIR=build

# Версия из git tag или dev
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_TIME ?= $(shell date -u '+%Y-%m-%d_%H:%M:%S')

LDFLAGS = -ldflags "-X github.com/audetv/tg-alert-proxy/internal/version.Version=$(VERSION) \
                    -X github.com/audetv/tg-alert-proxy/internal/version.Commit=$(COMMIT) \
                    -X github.com/audetv/tg-alert-proxy/internal/version.BuildTime=$(BUILD_TIME)"

version:
	@echo "Version: $(VERSION)"
	@echo "Commit: $(COMMIT)"
	@echo "Build time: $(BUILD_TIME)"

build:
	@echo "Building $(BINARY_NAME) version $(VERSION)..."
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/server
	@echo "Build complete: $(BUILD_DIR)/$(BINARY_NAME)"

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test -v ./...

clean:
	rm -rf $(BUILD_DIR)
	rm -rf ./data

docker-build:
	@echo "Building Docker image tg-alert-proxy:$(VERSION)..."
	docker build -t tg-alert-proxy:$(VERSION) -t tg-alert-proxy:latest .

docker-dev-up:
	docker compose -f docker-compose.dev.yml up -d

docker-dev-down:
	docker compose -f docker-compose.dev.yml down

docker-prod-up:
	docker compose up -d

docker-prod-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-restart:
	docker compose restart tg-alert-proxy

# Генерация секрета для прокси
gen-secret:
	@echo "Generated secret: $$(openssl rand -hex 16)"