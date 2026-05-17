// Точка входа AR Drive backend.
package main

import (
	"context"
	"errors"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"golang.org/x/net/netutil"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/api"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/arclient"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/audit"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/auth"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/config"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/folders"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/shares"
	"github.com/cloud-ru-tech/ar-drive/backend/internal/storage"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		zerolog.New(os.Stderr).Fatal().Err(err).Msg("load config")
	}

	logger := buildLogger(cfg.LogLevel)
	logger.Info().Str("env", cfg.Env).Str("listen", cfg.ListenAddr).Msg("starting AR Drive")

	db, err := storage.Open(cfg.DBPath, "migrations")
	if err != nil {
		logger.Fatal().Err(err).Msg("open sqlite")
	}
	defer db.Close()

	httpClient := &http.Client{Timeout: 5 * time.Minute}
	iam := auth.NewIAMClient(cfg.CloudruIAMURL, httpClient)
	tokenCache := auth.NewTokenCache(iam, cfg.TokenCacheSafetyMargin)
	arClient := arclient.New(cfg.CloudruARURL, httpClient)

	srv := api.New(cfg, logger, tokenCache, arClient,
		folders.NewStore(db), shares.NewStore(db), audit.New(db))

	mux := http.NewServeMux()
	mux.Handle("/", srv.Router())
	if cfg.MetricsEnabled {
		mux.Handle("/metrics", promhttp.Handler())
	}

	httpSrv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      120 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 14, // 16 KiB
	}

	// Ограничиваем количество одновременных соединений (защита от DDoS).
	listener, err := net.Listen("tcp", cfg.ListenAddr)
	if err != nil {
		logger.Fatal().Err(err).Msg("listen")
	}
	listener = netutil.LimitListener(listener, 1024)

	go func() {
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal().Err(err).Msg("serve")
		}
	}()

	// graceful shutdown
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig
	logger.Info().Msg("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)
}

func buildLogger(level string) zerolog.Logger {
	lvl, err := zerolog.ParseLevel(level)
	if err != nil {
		lvl = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(lvl)
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMs
	return zerolog.New(os.Stdout).With().Timestamp().Logger()
}
