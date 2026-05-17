# Архитектура AR Drive

## 1. Высокоуровневая схема

```
┌─────────────────┐       ┌────────────────────┐       ┌──────────────────────┐
│  React SPA      │──────▶│  Go API (chi)      │──────▶│  Cloud.ru AR API     │
│  snack-uikit    │ HTTPS │  + SQLite + cache  │ HTTPS │  iam.api.cloud.ru    │
│  localStorage   │       │                    │       │  ar.api.cloud.ru     │
└─────────────────┘       └────────────────────┘       └──────────────────────┘
```

Компоненты:

- **SPA (`frontend/`)** — React 18 + TS + Vite, скомпилирован в статику и
  отдаётся Nginx-контейнером (`frontend/Dockerfile`).
- **API (`backend/`)** — Go 1.22, chi-роутер, прокси-слой между SPA и Cloud.ru AR.
- **SQLite (`backend/migrations/`)** — состояние приложения: маркеры виртуальных
  папок, шеринг-ссылки, аудит-лог, серверные сессии, fallback-бакеты rate-limit.

## 2. Поток аутентификации

```
SPA                        Backend                IAM Cloud.ru
 │                            │                          │
 │ POST /api/v1/auth/verify   │                          │
 │  {client_id, secret, pid}  │                          │
 │───────────────────────────▶│                          │
 │                            │ POST /api/v1/auth/token  │
 │                            │  {keyId, secret}         │
 │                            │─────────────────────────▶│
 │                            │  200 {access_token, ...} │
 │                            │◀─────────────────────────│
 │                            │ cache[user_key] = token  │
 │  200 {ok: true, user_key}  │ (TTL = expires_in − 60s) │
 │◀───────────────────────────│                          │
 │ store encrypted creds in   │                          │
 │ localStorage (AES-GCM)     │                          │
```

Для последующих запросов SPA шлёт `X-Cloudru-Client-Id/Secret/Project-Id`. Бэк
обращается к `TokenCache.Get(creds)` — кеш сам обновит токен, если он истёк.

## 3. Поток загрузки файла

```
SPA                       Backend                     Cloud.ru AR
 │ POST /api/v1/registries/{id}/files (multipart)
 │ + X-Cloudru-* headers
 │────────────────────▶│
 │                     │ TokenCache.Get(creds)
 │                     │ ──▶ IAM (если кеш пуст)
 │                     │
 │                     │ PUT /v1/registries/{id}/generic/files/{path}
 │                     │ Authorization: Bearer <token>
 │                     │────────────────────────────▶│
 │                     │ 201 Created                 │
 │                     │◀────────────────────────────│
 │                     │ FolderStore.Delete(folder)
 │                     │ (снимаем виртуальный маркер)
 │                     │ AuditLog.Record(...)
 │ 201 {file metadata} │
 │◀────────────────────│
```

Прогресс загрузки прокидывается через `XMLHttpRequest.upload.onprogress` — SPA
показывает прогресс-бар. На бэке используется потоковая запись
(`http.MaxBytesReader` ограничивает размер). Файлы > 100 MB передаются стримом
через `io.Pipe` без буферизации.

## 4. Эмуляция виртуальных папок

```
            ┌────────────────────────────────────┐
            │       SQLite: folder_markers       │
            │  (project_id, registry_id, path)   │
            └────────────────────────────────────┘
                        ▲                     │
                        │ Create on demand    │ List + merge
                        │                     ▼
SPA  ──▶  Backend  ──▶  ListFiles(prefix)  ──▶  Cloud.ru AR
                        │ +
                        │ FolderStore.List(prefix)
                        ▼
            ┌────────────────────────────────────┐
            │ Объединённый ответ: реальные файлы │
            │ + виртуальные папки (с флагом ✦)   │
            └────────────────────────────────────┘
```

При загрузке первого файла в виртуальную папку backend атомарно удаляет маркер —
папка становится «реальной». При явном удалении папки backend постранично
итерирует все файлы под префиксом и удаляет их через AR API, затем стирает маркер.

## 5. Шеринг-ссылки

`share_links (short_id, user_key, project_id, registry_id, file_path,
password_hash, max_downloads, downloads, expires_at, revoked, created_at)`.

`POST /api/v1/shares` создаёт запись и возвращает URL `https://<host>/s/<short_id>`.
Ручка `GET /s/{shortID}` валидирует пароль/TTL/лимит, обращается к Cloud.ru AR
(токен берётся из кеша владельца ссылки — в MVP активная сессия обязательна,
см. [decisions.md](decisions.md)) и стримит ответ клиенту с правильным
`Content-Disposition`.

## 6. Безопасность

См. подробно [docs/security.md](security.md). Кратко: secure-headers, CORS-allowlist,
rate-limit (global / upload / auth), AES-GCM в localStorage, path-traversal валидация,
аудит-лог, MIME-sniffing, prepared statements везде.

## 7. Наблюдаемость

- `/healthz` (liveness), `/readyz` (readiness).
- `/metrics` (Prometheus) — длительность HTTP, размер аплоадов, попадание в
  кеш токенов, активные загрузки.
- `zerolog` JSON, одна строка на запрос.
