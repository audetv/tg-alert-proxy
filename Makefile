.PHONY: build run test clean docker-build docker-up docker-down

build:
	go build -o tg-alert-proxy ./cmd/server

run: build
	./tg-alert-proxy

test:
	go test -v ./...

clean:
	rm -f tg-alert-proxy
	rm -rf ./data

docker-build:
	docker build -t tg-alert-proxy:local .

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f

docker-restart:
	docker compose restart tg-alert-proxy

# Генерация секрета для прокси
gen-secret:
	@echo "Generated secret: $$(openssl rand -hex 16)"