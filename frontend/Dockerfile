# syntax=docker/dockerfile:1.6
# ---- Build stage ----
FROM node:20-alpine AS builder
WORKDIR /app

RUN corepack enable && corepack prepare pnpm@9.0.0 --activate

COPY package.json pnpm-lock.yaml* ./
# Если lock-файл есть — используем его для воспроизводимых сборок; иначе резолвим версии с нуля.
RUN if [ -f pnpm-lock.yaml ]; then pnpm install --frozen-lockfile; else pnpm install --no-frozen-lockfile; fi

COPY . .

ARG VITE_API_BASE_URL=/api/v1
ARG VITE_DEFAULT_LOCALE=ru
ENV VITE_API_BASE_URL=$VITE_API_BASE_URL
ENV VITE_DEFAULT_LOCALE=$VITE_DEFAULT_LOCALE

RUN pnpm run build

# ---- Runtime stage ----
FROM nginx:1.27-alpine
COPY --from=builder /app/dist /usr/share/nginx/html
COPY nginx.conf /etc/nginx/conf.d/default.conf
EXPOSE 80
