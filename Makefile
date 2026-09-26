GOBIN := $(shell go env GOPATH)/bin
DB_URL ?= postgres://wikipetia:wikipetia@localhost:5433/wikipetia?sslmode=disable
TEST_DB_URL ?= postgres://wikipetia:wikipetia@localhost:5433/wikipetia_test?sslmode=disable

.PHONY: up down migrate-up migrate-down generate test run psql testdb

up:
	docker compose up -d --wait

down:
	docker compose down

testdb: up
	docker compose exec -T db psql -U wikipetia -d wikipetia -c "SELECT 1 FROM pg_database WHERE datname='wikipetia_test'" -tA | grep -q 1 || \
		docker compose exec -T db createdb -U wikipetia wikipetia_test
	$(GOBIN)/migrate -path api/db/migrations -database "$(TEST_DB_URL)" up

migrate-up: up
	$(GOBIN)/migrate -path api/db/migrations -database "$(DB_URL)" up

migrate-down:
	$(GOBIN)/migrate -path api/db/migrations -database "$(DB_URL)" down 1

generate:
	cd api && $(GOBIN)/sqlc generate
	cd api && go run ./cmd/api -dump-openapi > openapi.yaml

test: testdb
	cd api && TEST_DATABASE_URL="$(TEST_DB_URL)" go test ./... -count=1

run: migrate-up
	cd api && DATABASE_URL="$(DB_URL)" go run ./cmd/api

psql: up
	docker compose exec db psql -U wikipetia -d wikipetia
