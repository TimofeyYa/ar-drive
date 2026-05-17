// Package config описывает конфигурацию приложения.
// Источник переменных: ENV (приоритет) и значения по умолчанию.
package config

import (
	"fmt"
	"strings"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Env        string `mapstructure:"ENV"`
	ListenAddr string `mapstructure:"LISTEN_ADDR"`
	LogLevel   string `mapstructure:"LOG_LEVEL"`
	DBPath     string `mapstructure:"DB_PATH"`

	CloudruIAMURL string `mapstructure:"CLOUDRU_IAM_URL"`
	CloudruARURL  string `mapstructure:"CLOUDRU_AR_URL"`

	CORSOrigins []string `mapstructure:"CORS_ORIGINS"`

	RateLimitGlobalRPM int `mapstructure:"RATE_LIMIT_GLOBAL_RPM"`
	RateLimitUploadRPM int `mapstructure:"RATE_LIMIT_UPLOAD_RPM"`
	RateLimitAuthRPM   int `mapstructure:"RATE_LIMIT_AUTH_RPM"`

	SessionSecret   string `mapstructure:"SESSION_SECRET"`
	ShareLinkSecret string `mapstructure:"SHARE_LINK_SECRET"`

	MaxUploadBytes int64 `mapstructure:"MAX_UPLOAD_BYTES"`

	TokenCacheSafetyMargin time.Duration `mapstructure:"-"`
	TokenCacheSafetyMarginSec int        `mapstructure:"TOKEN_CACHE_SAFETY_MARGIN_SEC"`

	MetricsEnabled bool   `mapstructure:"METRICS_ENABLED"`
	RedisAddr      string `mapstructure:"REDIS_ADDR"`
}

// Load читает ENV и возвращает заполненный Config.
// Никаких файлов конфигурации не используем — только ENV.
func Load() (*Config, error) {
	v := viper.New()
	v.AutomaticEnv()
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))

	// Defaults
	v.SetDefault("ENV", "development")
	v.SetDefault("LISTEN_ADDR", ":8080")
	v.SetDefault("LOG_LEVEL", "info")
	v.SetDefault("DB_PATH", "./data/app.db")
	v.SetDefault("CLOUDRU_IAM_URL", "https://iam.api.cloud.ru")
	v.SetDefault("CLOUDRU_AR_URL", "https://ar.api.cloud.ru")
	v.SetDefault("CORS_ORIGINS", "http://localhost:3000")
	v.SetDefault("RATE_LIMIT_GLOBAL_RPM", 100)
	v.SetDefault("RATE_LIMIT_UPLOAD_RPM", 30)
	v.SetDefault("RATE_LIMIT_AUTH_RPM", 5)
	v.SetDefault("MAX_UPLOAD_BYTES", int64(1024*1024*1024)) // 1 GiB
	v.SetDefault("TOKEN_CACHE_SAFETY_MARGIN_SEC", 60)
	v.SetDefault("METRICS_ENABLED", true)

	cfg := &Config{}
	if err := v.Unmarshal(cfg); err != nil {
		return nil, fmt.Errorf("unmarshal config: %w", err)
	}

	// CORS_ORIGINS приходит строкой "a,b,c" — разрежем.
	if raw := v.GetString("CORS_ORIGINS"); raw != "" {
		parts := strings.Split(raw, ",")
		out := make([]string, 0, len(parts))
		for _, p := range parts {
			p = strings.TrimSpace(p)
			if p != "" {
				out = append(out, p)
			}
		}
		cfg.CORSOrigins = out
	}

	cfg.TokenCacheSafetyMargin = time.Duration(cfg.TokenCacheSafetyMarginSec) * time.Second

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.SessionSecret == "" || c.SessionSecret == "change-me-please-32-bytes-hex" {
		if c.Env == "production" {
			return fmt.Errorf("SESSION_SECRET must be set in production")
		}
	}
	if c.ShareLinkSecret == "" || c.ShareLinkSecret == "change-me-please-32-bytes-hex" {
		if c.Env == "production" {
			return fmt.Errorf("SHARE_LINK_SECRET must be set in production")
		}
	}
	if c.MaxUploadBytes <= 0 || c.MaxUploadBytes > 1024*1024*1024 {
		return fmt.Errorf("MAX_UPLOAD_BYTES must be in (0, 1 GiB]")
	}
	return nil
}
