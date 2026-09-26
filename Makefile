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

.PHONY: boundaries check seed

boundaries:
	./scripts/check-boundaries.sh

check: boundaries
	cd api && go vet ./...
	cd api && $(GOBIN)/sqlc diff
	cd api && go run ./cmd/api -dump-openapi > /tmp/openapi.check.yaml
	diff -q api/openapi.yaml /tmp/openapi.check.yaml || \
		(echo "openapi.yaml is stale — run 'make generate' and commit it" && exit 1)
	$(MAKE) test

seed: migrate-up
	cd api && DATABASE_URL="$(DB_URL)" go run ./cmd/seed
