.PHONY: test build run up down logs migrate

test:
	go test ./...

build:
	go build -o timepilot ./cmd/timepilot

run:
	go run ./cmd/timepilot server

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f app

migrate:
	go run ./cmd/timepilot migrate
