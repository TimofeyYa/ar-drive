# syntax=docker/dockerfile:1.6
# ---- Build stage ----
FROM golang:1.22-alpine AS builder
WORKDIR /src

# Кешируем зависимости отдельно
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Pure-Go SQLite (modernc.org/sqlite) — CGO не нужен.
ENV CGO_ENABLED=0 GOOS=linux GOARCH=amd64
RUN go build -trimpath -ldflags="-s -w" -o /out/ar-drive-server ./cmd/server

# ---- Runtime stage ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && \
    addgroup -S app && adduser -S app -G app && \
    mkdir -p /data && chown app:app /data

COPY --from=builder /out/ar-drive-server /usr/local/bin/ar-drive-server
COPY --from=builder /src/migrations /migrations

USER app
WORKDIR /home/app
EXPOSE 8080
VOLUME ["/data"]

HEALTHCHECK --interval=10s --timeout=3s --retries=5 \
    CMD wget -qO- http://127.0.0.1:8080/healthz || exit 1

ENTRYPOINT ["/usr/local/bin/ar-drive-server"]
