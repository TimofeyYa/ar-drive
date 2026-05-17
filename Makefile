# =============================================================================
# AR Drive Makefile
# =============================================================================
SHELL := /bin/bash
DC    := docker compose
GO    := go

.DEFAULT_GOAL := help

## help: показать список целей
.PHONY: help
help:
	@grep -E '^## ' $(MAKEFILE_LIST) | sed -E 's/^## ([a-zA-Z0-9_-]+): (.*)$$/  \1\t- \2/' | column -t -s $$'\t'

## up: поднять весь стек (backend + frontend) в Docker
.PHONY: up
up:
	$(DC) up -d --build
	@echo "✅ AR Drive доступен на http://localhost:3000"

## down: остановить и удалить контейнеры и тома
.PHONY: down
down:
	$(DC) down -v

## build: пересобрать оба Docker-образа без запуска
.PHONY: build
build:
	$(DC) build

## logs: tail логов всех сервисов
.PHONY: logs
logs:
	$(DC) logs -f --tail=200

## ps: список запущенных сервисов
.PHONY: ps
ps:
	$(DC) ps

## migrate: применить миграции SQLite через goose
.PHONY: migrate
migrate:
	cd backend && goose -dir migrations sqlite3 ../data/app.db up

## migrate-down: откатить последнюю миграцию
.PHONY: migrate-down
migrate-down:
	cd backend && goose -dir migrations sqlite3 ../data/app.db down

## seed: загрузить тестовые данные (фикстуры)
.PHONY: seed
seed:
	cd backend && $(GO) run ./cmd/seed || echo "seed CLI пока не реализован"

## test: прогнать все тесты (backend + frontend)
.PHONY: test
test: test-backend test-frontend

## test-backend: go test со счётчиком покрытия
.PHONY: test-backend
test-backend:
	cd backend && $(GO) test -race -coverprofile=coverage.out ./... && \
		$(GO) tool cover -func=coverage.out | tail -1

## test-frontend: vitest
.PHONY: test-frontend
test-frontend:
	cd frontend && pnpm run test

## e2e: Playwright e2e против поднятого compose
.PHONY: e2e
e2e:
	cd frontend && pnpm run test:e2e

## lint: golangci-lint + eslint
.PHONY: lint
lint:
	cd backend && golangci-lint run ./...
	cd frontend && pnpm run lint

## fmt: gofmt + prettier
.PHONY: fmt
fmt:
	cd backend && $(GO) fmt ./... && goimports -w .
	cd frontend && pnpm run format

## gen-mocks: сгенерировать моки (mockery) для backend
.PHONY: gen-mocks
gen-mocks:
	cd backend && mockery --all --output ./internal/mocks --case=snake

## gen-openapi: сгенерировать ar-client из swagger Cloud.ru
.PHONY: gen-openapi
gen-openapi:
	@echo "ℹ️  Положите swagger.json от Cloud.ru AR в backend/internal/arclient/swagger.json"
	@echo "ℹ️  и запустите: oapi-codegen -package arclient ... swagger.json"

## clean: удалить артефакты сборки и БД
.PHONY: clean
clean:
	rm -rf backend/bin backend/coverage.out backend/coverage.html
	rm -rf frontend/dist frontend/coverage frontend/playwright-report
	rm -rf data/*.db data/*.db-journal

## scan: gitleaks + govulncheck + npm audit + trivy
.PHONY: scan
scan:
	gitleaks detect --no-banner --redact || true
	cd backend && govulncheck ./... || true
	cd frontend && pnpm audit --prod || true
	command -v trivy >/dev/null && trivy fs --severity HIGH,CRITICAL --no-progress . || true

## smoke: убедиться, что поднятый стек отвечает 200 OK
.PHONY: smoke
smoke:
	@curl -fsS http://localhost:8080/healthz && echo "  ✅ backend OK"
	@curl -fsS http://localhost:3000/ -o /dev/null && echo "  ✅ frontend OK"
