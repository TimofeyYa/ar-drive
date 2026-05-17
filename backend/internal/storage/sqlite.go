// Package storage — слой работы с SQLite. Использует pure-Go драйвер
// modernc.org/sqlite — CGO не требуется.
package storage

import (
	"database/sql"
	"embed"
	"fmt"
	"path/filepath"

	"github.com/pressly/goose/v3"
	_ "modernc.org/sqlite" // регистрирует драйвер "sqlite"
)

//go:embed schema.sql
var embedSchema embed.FS

// Open открывает базу и применяет миграции из директории migrationsDir,
// если она существует. Иначе применяет встроенную schema.sql.
func Open(dbPath, migrationsDir string) (*sql.DB, error) {
	if err := ensureDir(dbPath); err != nil {
		return nil, err
	}

	dsn := fmt.Sprintf("%s?_pragma=journal_mode(WAL)&_pragma=foreign_keys(on)&_pragma=busy_timeout(5000)", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}

	db.SetMaxOpenConns(1) // SQLite — один писатель
	db.SetMaxIdleConns(1)

	if err := migrate(db, migrationsDir); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}
	return db, nil
}

func migrate(db *sql.DB, dir string) error {
	goose.SetBaseFS(nil)
	if err := goose.SetDialect("sqlite3"); err != nil {
		return err
	}
	if dir != "" {
		if err := goose.Up(db, dir); err == nil {
			return nil
		}
		// fallthrough to embedded
	}
	// Fallback: применяем встроенную схему как единую миграцию.
	schema, err := embedSchema.ReadFile("schema.sql")
	if err != nil {
		return fmt.Errorf("read embedded schema: %w", err)
	}
	if _, err := db.Exec(string(schema)); err != nil {
		return fmt.Errorf("apply embedded schema: %w", err)
	}
	return nil
}

func ensureDir(dbPath string) error {
	return mkdirAll(filepath.Dir(dbPath), 0o750)
}
