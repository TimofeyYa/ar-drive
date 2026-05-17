# Модель угроз и меры защиты AR Drive

Структурируем по [STRIDE](https://learn.microsoft.com/en-us/azure/security/develop/threat-modeling-tool-threats).

## Spoofing — подмена идентичности

| Угроза | Меры защиты |
| --- | --- |
| Утечка ключей Cloud.ru | AES-GCM-256 шифрование в localStorage; ключ выводится из парольной фразы / device-fingerprint через PBKDF2-SHA256 (200 000 итераций). Передаются только в запросе HTTPS. |
| Подмена IAM ответа | Только HTTPS, TLS 1.2+. `ReadHeaderTimeout=10s` чтобы избегать slow-loris. |
| Спуфинг шер-ссылки | `short_id` — 12 символов `base64-url` из `crypto/rand` (≈72 бит энтропии). Опциональный пароль (bcrypt). HMAC-сигнатура планируется (см. decisions.md). |

## Tampering — модификация

| Угроза | Меры защиты |
| --- | --- |
| Подмена файла «на лету» | `Content-Length` валидируется; `MaxBytesReader`. SHA-256 опционально (Cloud.ru AR возвращает). |
| Изменение записей в SQLite через SQL-инъекции | Только параметризованные запросы (`database/sql` с `?`-плейсхолдерами). Линтер `sqlvet` в CI. |
| Path traversal (загрузка / скачивание / удаление) | `validation.CleanFilePath` / `CleanFolderPath`: запрет `..`, `\0`, `\`, ведущих `/`, абсолютных путей. Юнит-тесты в `internal/validation`. |

## Repudiation — отрицание действий

| Угроза | Меры защиты |
| --- | --- |
| Пользователь отрицает удаление файла | Полный аудит-лог в SQLite (`audit_log`): user_key, IP, UA, action, target, статус, ошибка. |

## Information disclosure — утечка данных

| Угроза | Меры защиты |
| --- | --- |
| Утечка секретов в логах | `zerolog` структурный JSON; нет логирования заголовков `X-Cloudru-*`. Секреты не попадают в audit_log (хранится только `user_key = sha256(client_id+project_id)`). |
| Утечка через CORS | Whitelist origin'ов через ENV (`CORS_ORIGINS`). |
| XSS в имени файла / Markdown | На фронте — `DOMPurify` для имён; `rehype-sanitize` для Markdown-просмотрщика. CSP `default-src 'none'` на API. |
| Secret leak в git | `gitleaks` в CI. |
| Открытый листинг чужих share-ссылок | `/api/v1/shares` фильтруется по `user_key` (запрос сравнивает с заголовками текущей сессии). |

## Denial of service

| Угроза | Меры защиты |
| --- | --- |
| Flood-атаки | `ulule/limiter`: 100 req/min глобально на IP, 30 req/min на upload, 5 req/min на `/auth/verify`. `netutil.LimitListener(1024)` ограничивает coном. Таймауты: `ReadHeaderTimeout=10s`, `ReadTimeout=60s`, `WriteTimeout=120s`. |
| Загрузка очень больших файлов | `http.MaxBytesReader(body, 1 GiB + 1 MiB)` + проверка `header.Size`. `MAX_UPLOAD_BYTES` валидируется в config. |
| Long-running операции (удаление папки) | Постраничные итерации (по 500 файлов на страницу), таймаут запроса 120 секунд + рекомендация выносить за nginx/Cloudflare для глобальных лимитов. |
| Exhaustion SQLite | `SetMaxOpenConns(1)`, WAL-режим, busy_timeout=5s. Аудит-лог можно ротировать (TODO). |

## Elevation of privilege

| Угроза | Меры защиты |
| --- | --- |
| Использование чужой шер-ссылки | Проверка `revoked`, `expires_at`, `max_downloads`, bcrypt-сравнение пароля. |
| Доступ к чужим виртуальным папкам | Маркеры привязаны к `(project_id, registry_id)` — другой пользователь увидит только свои папки по своим кредам. |
| Обход rate-limit через X-Forwarded-For | В `middleware.RealIP` берём первый IP из заголовка; в production обязательно поставить за nginx/Cloudflare и принимать заголовок только от доверенного прокси (TODO). |

## Дополнительно

- **Security headers** (`unrolled/secure`): `Strict-Transport-Security`,
  `X-Frame-Options=DENY`, `X-Content-Type-Options=nosniff`,
  `Referrer-Policy=strict-origin-when-cross-origin`, минимальный CSP.
- **MIME-sniffing**: на бэке используется `http.DetectContentType` при отдаче
  файлов, чтобы не доверять заголовку клиента.
- **Никаких сторонних eval / unsafe-inline** на фронте — Vite-сборка с CSP-совместимыми бандлами.
- **Зависимости**: `govulncheck` (Go), `pnpm audit` (Node), `trivy fs` (контейнеры) в `make scan`.

## Что ещё нужно сделать

См. [decisions.md](decisions.md) — TODO #4 (HMAC-подпись шер-ссылок), #5
(долгоживущий server-side токен владельца для оффлайн-шеринга), #6 (ротация audit_log).
