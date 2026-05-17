// Package middleware — HTTP middleware: security headers, CORS, rate-limit, recovery.
package middleware

import (
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
	"github.com/unrolled/secure"
	limiter "github.com/ulule/limiter/v3"
	mhttp "github.com/ulule/limiter/v3/drivers/middleware/stdlib"
	memstore "github.com/ulule/limiter/v3/drivers/store/memory"
)

// SecureHeaders ставит набор security-заголовков по умолчанию.
// CSP оставляем мягким — фронтенд отдаётся отдельным сервером (nginx),
// поэтому для API подходит default-src 'none'.
func SecureHeaders() func(http.Handler) http.Handler {
	s := secure.New(secure.Options{
		FrameDeny:             true,
		ContentTypeNosniff:    true,
		BrowserXssFilter:      true,
		ReferrerPolicy:        "strict-origin-when-cross-origin",
		PermissionsPolicy:     "geolocation=(), microphone=(), camera=()",
		ContentSecurityPolicy: "default-src 'none'; frame-ancestors 'none'",
		STSSeconds:            31536000,
		STSIncludeSubdomains:  true,
		STSPreload:            false,
		IsDevelopment:         false,
	})
	return s.Handler
}

// CORS строит CORS middleware из whitelist origin'ов.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	return cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Requested-With"},
		ExposedHeaders:   []string{"Link", "X-Total-Count"},
		AllowCredentials: true,
		MaxAge:           300,
	})
}

// RateLimit создаёт middleware с N запросов в минуту, ключуясь по IP.
func RateLimit(perMinute int) func(http.Handler) http.Handler {
	rate := limiter.Rate{Period: time.Minute, Limit: int64(perMinute)}
	store := memstore.NewStore()
	instance := limiter.New(store, rate)
	return mhttp.NewMiddleware(instance).Handler
}

// RealIP извлекает реальный IP клиента из X-Forwarded-For / X-Real-IP / RemoteAddr.
func RealIP(r *http.Request) string {
	if v := r.Header.Get("X-Real-IP"); v != "" {
		return v
	}
	if v := r.Header.Get("X-Forwarded-For"); v != "" {
		if i := strings.Index(v, ","); i >= 0 {
			return strings.TrimSpace(v[:i])
		}
		return strings.TrimSpace(v)
	}
	// host:port → host
	if i := strings.LastIndex(r.RemoteAddr, ":"); i > 0 {
		return r.RemoteAddr[:i]
	}
	return r.RemoteAddr
}

// AccessLog — структурный лог через zerolog (одна строка JSON на запрос).
func AccessLog(logger zerolog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info().
				Str("method", r.Method).
				Str("path", r.URL.Path).
				Int("status", ww.Status()).
				Int("bytes", ww.BytesWritten()).
				Dur("dur", time.Since(start)).
				Str("ip", RealIP(r)).
				Str("ua", r.UserAgent()).
				Msg("http")
		})
	}
}
