.PHONY: build test run lint docker-up docker-down

build:
	go build -o bin/egudoc ./cmd/server

test:
	go test ./... -v -race -count=1

run:
	go run ./cmd/server

lint:
	golangci-lint run ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down
