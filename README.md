# AR Drive — file manager поверх Cloud.ru Artifact Registry (Generic)

> Веб-приложение в духе Yandex Disk / Google Drive / macOS Finder, использующее
> Generic-реестры [Cloud.ru Artifact Registry](https://cloud.ru/docs/artifact-registry-evolution/ug/index?source-platform=Evolution)
> как backend-хранилище. Цель — удобный обменник и архив для DevOps-инженеров и
> разработчиков, с быстрым шерингом ссылок и встроенным редактором JSON/YAML/кода.

[Русский](#русский) · [English](#english)

---

## Русский

### Возможности

- **Просмотр всех ваших Generic-реестров** и переключение между ними.
- **Двухпанельный файловый менеджер** с деревом папок, таблицей файлов, хлебными
  крошками и drag-and-drop загрузкой (включая drop папок целиком).
- **Эмуляция «пустых папок»** поверх Cloud.ru AR (см. [docs/decisions.md](docs/decisions.md)) —
  Generic-реестр не поддерживает их нативно.
- **Предпросмотр**: изображения, видео, аудио, PDF, JSON/YAML/Markdown/код с
  подсветкой синтаксиса.
- **Встроенный редактор** для текстовых файлов с авто-форматированием JSON.
- **Шеринг ссылок в один клик** — короткие подписанные URL с настраиваемым TTL,
  паролем и лимитом скачиваний.
- **Локализация** RU / EN, темы light / dark с токенами Cloud.ru.

### Быстрый старт

```bash
cp .env.example .env
# (опц.) поменяйте секреты SESSION_SECRET и SHARE_LINK_SECRET
make up
# UI: http://localhost:3000   API: http://localhost:8080
```

При первом входе откроется экран `/setup`, где нужно ввести **Key ID**,
**Key Secret** сервисного аккаунта Cloud.ru и **Project ID**. Инструкция по
получению ключей: [Cloud.ru Quickstart](https://cloud.ru/docs/console_api/ug/topics/quickstart).

Ключи хранятся **только в вашем браузере** (зашифровано AES-GCM-256, ключ
выводится из парольной фразы / device-fingerprint через PBKDF2). Backend получает
их в заголовках запроса и не сохраняет на диск.

### Полезные команды

| Команда               | Что делает                                       |
| --------------------- | ------------------------------------------------ |
| `make help`           | Перечень целей                                   |
| `make up`             | Поднять весь стек в Docker                       |
| `make down`           | Остановить и удалить контейнеры и тома           |
| `make logs`           | Логи всех сервисов                               |
| `make test`           | Backend `go test` + frontend `vitest`            |
| `make e2e`            | Playwright против поднятого compose              |
| `make lint`           | `golangci-lint` + `eslint`                       |
| `make scan`           | `gitleaks`, `govulncheck`, `npm audit`, `trivy`  |
| `make smoke`          | Проверить, что оба сервиса отвечают 200          |

### Документация

- [docs/architecture.md](docs/architecture.md) — архитектура, sequence-диаграммы.
- [docs/api.md](docs/api.md) — описание REST API (OpenAPI 3.1).
- [docs/security.md](docs/security.md) — модель угроз STRIDE и принятые меры.
- [docs/decisions.md](docs/decisions.md) — открытые архитектурные вопросы.

### Стек

- **Frontend**: React 18 + TypeScript + Vite, [`@snack-uikit/*`](https://github.com/cloud-ru-tech/snack-uikit), TanStack Query, Zustand.
- **Backend**: Go 1.22, `chi`, `zerolog`, `viper`, SQLite через `modernc.org/sqlite` (без CGO), миграции `goose`.
- **Контейнеризация**: multi-stage Docker, единый `docker-compose.yml`.
- **Тесты**: `testify` + `httptest`, Vitest + React Testing Library, Playwright.
- **Безопасность**: `unrolled/secure`, `ulule/limiter`, `go-playground/validator`,
  AES-GCM шифрование localStorage, аудит-лог всех операций.

### Текущий статус

Это **MVP-каркас**. Реализованы все базовые ручки бэкенда, фронтенд, безопасность,
тесты, документация. Открытые задачи на следующие итерации — в
[docs/decisions.md](docs/decisions.md) (актуальные пути file-эндпоинтов Cloud.ru AR
из swagger, presigned URL, история версий, расширение e2e против реального API).

### Лицензия

[Apache 2.0](LICENSE) — совместима со snack-uikit.

---

## English

### Features

- Browse and switch between your Cloud.ru **Generic** registries.
- **Two-pane file manager** with folder tree, file table, breadcrumbs, drag-and-drop uploads (including OS folders).
- **Emulated empty folders** on top of Cloud.ru AR (see [docs/decisions.md](docs/decisions.md))
  since Generic registries do not support them natively.
- **Preview**: images, video, audio, PDF, JSON/YAML/Markdown/code with syntax highlighting.
- **Built-in editor** for text files with JSON auto-format.
- **One-click share links** — short signed URLs with configurable TTL, password, and download limits.
- **Localization** RU/EN, **light/dark** themes using Cloud.ru design tokens.

### Quick start

```bash
cp .env.example .env
make up
# UI: http://localhost:3000   API: http://localhost:8080
```

On the first visit the app prompts for Cloud.ru **Key ID**, **Key Secret** and
**Project ID** at `/setup`. Keys are kept encrypted (AES-GCM-256, key derived via
PBKDF2) in your browser's localStorage — the backend receives them in request
headers and never persists them.

### Stack

React 18 + TS + Vite + `@snack-uikit/*`, TanStack Query, Zustand · Go 1.22 + chi
+ SQLite (`modernc.org/sqlite`, no CGO) + goose · Docker Compose · `testify` /
Vitest / Playwright.

### License

[Apache 2.0](LICENSE).
