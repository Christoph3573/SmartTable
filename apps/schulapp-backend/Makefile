.PHONY: run build generate migrate migrate-down seed

run:
	go run ./cmd/server/main.go

build:
	go build -o bin/schulapp ./cmd/server/main.go

generate:
	oapi-codegen --config=oapi-codegen.yaml openapi.yaml
	sqlc generate
	cd ../codeclub-ui && npm run generate

migrate:
	migrate -path ./migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path ./migrations -database "$(DATABASE_URL)" down

seed:
	go run ./seed/main.go
