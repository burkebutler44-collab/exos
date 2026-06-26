.PHONY: generate migrate-up migrate-down run test

generate:
	buf generate
	sqlc generate

migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up

migrate-down:
	goose -dir db/migrations postgres "$$DATABASE_URL" down

run:
	go run ./cmd/api

test:
	go test ./...
