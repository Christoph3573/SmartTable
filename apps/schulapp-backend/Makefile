.PHONY: run build migrate migrate-down seed

run:
	go run ./cmd/server/main.go

build:
	go build -o bin/schulapp ./cmd/server/main.go

migrate:
	migrate -path ./migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(DATABASE_URL)" down

seed:
	go run ./seed/main.go
