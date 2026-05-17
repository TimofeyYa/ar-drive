// Package audit — журналирование действий пользователей в SQLite.
package audit

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"
)

type Event struct {
	UserKey      string
	ProjectID    string
	RegistryID   string
	Action       string
	Target       string
	IP           string
	UserAgent    string
	Status       string // "success" | "error"
	ErrorMessage string
	Meta         map[string]any
}

type Logger struct {
	db *sql.DB
}

func New(db *sql.DB) *Logger { return &Logger{db: db} }

func (l *Logger) Record(ctx context.Context, e Event) error {
	var metaJSON sql.NullString
	if len(e.Meta) > 0 {
		b, err := json.Marshal(e.Meta)
		if err == nil {
			metaJSON = sql.NullString{String: string(b), Valid: true}
		}
	}
	_, err := l.db.ExecContext(ctx, `
		INSERT INTO audit_log (user_key, project_id, registry_id, action, target, ip, user_agent, status, error_message, meta, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		e.UserKey, nullString(e.ProjectID), nullString(e.RegistryID), e.Action,
		nullString(e.Target), nullString(e.IP), nullString(e.UserAgent), e.Status,
		nullString(e.ErrorMessage), metaJSON, time.Now().Unix(),
	)
	return err
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
