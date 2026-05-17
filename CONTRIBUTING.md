# Contributing to AR Drive

## Развёртывание dev-окружения

```bash
git clone <repo>
cd ar-drive
cp .env.example .env
make up        # docker-compose
make smoke     # проверка health
```

Альтернатива без Docker:
```bash
# Backend
cd backend
go mod download
go run ./cmd/server

# Frontend (другое окно)
cd frontend
pnpm install
pnpm dev
```

## Style guide

- Go: `gofmt`, `goimports`, `golangci-lint` (errcheck, govet, staticcheck, gosec).
- TS/React: `eslint` + `prettier`. Запрещены `any` без явного комментария-обоснования.
- Коммиты — Conventional Commits (`feat:`, `fix:`, `chore:`, `docs:`).

## Тесты

PR не мержится без прохождения CI (lint + unit + integration + security scan).
Покрытие backend — целевой минимум 75%.

## Чек-лист безопасности на PR

- [ ] Никаких новых `fmt.Sprintf` в SQL-запросах — только `?`-плейсхолдеры.
- [ ] Все пути файлов проходят `validation.CleanFilePath` / `CleanFolderPath`.
- [ ] Никаких новых endpoints без rate-limit.
- [ ] Никаких секретов в логах / репозитории (проверяет `gitleaks`).

## Архитектурные решения

Любое существенное архитектурное изменение — отдельный ADR в `docs/decisions.md`.
