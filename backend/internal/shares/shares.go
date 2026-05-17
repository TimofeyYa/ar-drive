// Package shares — короткие подписанные ссылки на скачивание файлов
// из приватных Generic-реестров. Бэк сам аутентифицируется в Cloud.ru
// от имени владельца ссылки и стримит файл клиенту.
package shares

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"errors"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Link struct {
	ShortID      string
	UserKey      string
	ProjectID    string
	RegistryID   string
	FilePath     string
	HasPassword  bool
	MaxDownloads int64 // 0 = без лимита
	Downloads    int64
	ExpiresAt    time.Time // нулевое = бессрочно
	Revoked      bool
	CreatedAt    time.Time
}

type CreateInput struct {
	UserKey      string
	ProjectID    string
	RegistryID   string
	FilePath     string
	Password     string        // пусто = без пароля
	TTL          time.Duration // 0 = бессрочно
	MaxDownloads int64
}

var (
	ErrNotFound      = errors.New("share link not found")
	ErrRevoked       = errors.New("share link revoked")
	ErrExpired       = errors.New("share link expired")
	ErrExhausted     = errors.New("share link downloads exhausted")
	ErrPasswordWrong = errors.New("share link password incorrect")
	ErrPasswordReq   = errors.New("share link requires password")
)

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Create — генерирует short_id (12 символов base64-url), при необходимости хеширует пароль.
func (s *Store) Create(ctx context.Context, in CreateInput) (*Link, error) {
	shortID, err := generateShortID(12)
	if err != nil {
		return nil, err
	}
	var pwHash sql.NullString
	if in.Password != "" {
		h, err := bcrypt.GenerateFromPassword([]byte(in.Password), bcrypt.DefaultCost)
		if err != nil {
			return nil, err
		}
		pwHash = sql.NullString{String: string(h), Valid: true}
	}
	var expiresAt sql.NullInt64
	if in.TTL > 0 {
		expiresAt = sql.NullInt64{Int64: time.Now().Add(in.TTL).Unix(), Valid: true}
	}
	var maxDownloads sql.NullInt64
	if in.MaxDownloads > 0 {
		maxDownloads = sql.NullInt64{Int64: in.MaxDownloads, Valid: true}
	}
	now := time.Now().Unix()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO share_links (short_id, user_key, project_id, registry_id, file_path,
		                        password_hash, max_downloads, downloads, expires_at, revoked, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, 0, ?, 0, ?)`,
		shortID, in.UserKey, in.ProjectID, in.RegistryID, in.FilePath,
		pwHash, maxDownloads, expiresAt, now)
	if err != nil {
		return nil, err
	}
	link := &Link{
		ShortID:      shortID,
		UserKey:      in.UserKey,
		ProjectID:    in.ProjectID,
		RegistryID:   in.RegistryID,
		FilePath:     in.FilePath,
		HasPassword:  pwHash.Valid,
		MaxDownloads: in.MaxDownloads,
		CreatedAt:    time.Unix(now, 0),
	}
	if expiresAt.Valid {
		link.ExpiresAt = time.Unix(expiresAt.Int64, 0)
	}
	return link, nil
}

// Get возвращает ссылку по short_id без проверок состояния (используется для отображения).
func (s *Store) Get(ctx context.Context, shortID string) (*Link, string, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT short_id, user_key, project_id, registry_id, file_path,
		       password_hash, max_downloads, downloads, expires_at, revoked, created_at
		FROM share_links WHERE short_id = ?`, shortID)

	var l Link
	var pwHash sql.NullString
	var maxDownloads, expiresAt sql.NullInt64
	var revoked int
	var createdAt int64
	if err := row.Scan(&l.ShortID, &l.UserKey, &l.ProjectID, &l.RegistryID, &l.FilePath,
		&pwHash, &maxDownloads, &l.Downloads, &expiresAt, &revoked, &createdAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, "", ErrNotFound
		}
		return nil, "", err
	}
	l.HasPassword = pwHash.Valid
	if maxDownloads.Valid {
		l.MaxDownloads = maxDownloads.Int64
	}
	if expiresAt.Valid {
		l.ExpiresAt = time.Unix(expiresAt.Int64, 0)
	}
	l.Revoked = revoked != 0
	l.CreatedAt = time.Unix(createdAt, 0)
	return &l, pwHash.String, nil
}

// Validate проверяет состояние и пароль ссылки. Возвращает ErrPasswordReq, если требуется пароль.
func (s *Store) Validate(ctx context.Context, shortID, password string) (*Link, error) {
	link, pwHash, err := s.Get(ctx, shortID)
	if err != nil {
		return nil, err
	}
	if link.Revoked {
		return nil, ErrRevoked
	}
	if !link.ExpiresAt.IsZero() && time.Now().After(link.ExpiresAt) {
		return nil, ErrExpired
	}
	if link.MaxDownloads > 0 && link.Downloads >= link.MaxDownloads {
		return nil, ErrExhausted
	}
	if link.HasPassword {
		if password == "" {
			return nil, ErrPasswordReq
		}
		if err := bcrypt.CompareHashAndPassword([]byte(pwHash), []byte(password)); err != nil {
			return nil, ErrPasswordWrong
		}
	}
	return link, nil
}

// IncrementDownloads атомарно увеличивает счётчик скачиваний.
func (s *Store) IncrementDownloads(ctx context.Context, shortID string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE share_links SET downloads = downloads + 1 WHERE short_id = ?`, shortID)
	return err
}

// Revoke помечает ссылку отозванной.
func (s *Store) Revoke(ctx context.Context, userKey, shortID string) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE share_links SET revoked = 1 WHERE short_id = ? AND user_key = ?`,
		shortID, userKey)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

// ListByUser возвращает ссылки пользователя.
func (s *Store) ListByUser(ctx context.Context, userKey string) ([]Link, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT short_id, user_key, project_id, registry_id, file_path,
		       password_hash, max_downloads, downloads, expires_at, revoked, created_at
		FROM share_links WHERE user_key = ? ORDER BY created_at DESC`, userKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []Link
	for rows.Next() {
		var l Link
		var pwHash sql.NullString
		var maxDownloads, expiresAt sql.NullInt64
		var revoked int
		var createdAt int64
		if err := rows.Scan(&l.ShortID, &l.UserKey, &l.ProjectID, &l.RegistryID, &l.FilePath,
			&pwHash, &maxDownloads, &l.Downloads, &expiresAt, &revoked, &createdAt); err != nil {
			return nil, err
		}
		l.HasPassword = pwHash.Valid
		if maxDownloads.Valid {
			l.MaxDownloads = maxDownloads.Int64
		}
		if expiresAt.Valid {
			l.ExpiresAt = time.Unix(expiresAt.Int64, 0)
		}
		l.Revoked = revoked != 0
		l.CreatedAt = time.Unix(createdAt, 0)
		out = append(out, l)
	}
	return out, rows.Err()
}

func generateShortID(bytes int) (string, error) {
	b := make([]byte, bytes)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b)[:bytes], nil
}
