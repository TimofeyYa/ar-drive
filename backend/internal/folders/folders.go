// Package folders реализует эмуляцию «пустых папок» поверх Generic-реестра.
// Cloud.ru AR не поддерживает создание пустой папки или её удаление как сущности —
// папка существует только как префикс пути файла. Мы храним «маркеры» в SQLite
// и подмешиваем их к ответам ListFiles.
package folders

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/validation"
)

type Marker struct {
	ID         int64
	UserKey    string
	ProjectID  string
	RegistryID string
	FullPath   string // например "dir/sub/"
	CreatedBy  string
	CreatedAt  time.Time
}

var ErrAlreadyExists = errors.New("folder marker already exists")

type Store struct {
	db *sql.DB
}

func NewStore(db *sql.DB) *Store { return &Store{db: db} }

// Create добавляет маркер. Путь обязан быть валидным относительным с завершающим "/".
func (s *Store) Create(ctx context.Context, m Marker) error {
	cleaned, err := validation.CleanFolderPath(m.FullPath)
	if err != nil {
		return err
	}
	m.FullPath = cleaned
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO folder_markers (user_key, project_id, registry_id, full_path, created_by, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		m.UserKey, m.ProjectID, m.RegistryID, m.FullPath, m.CreatedBy, time.Now().Unix())
	if err != nil {
		// Уникальный constraint
		if isUniqueViolation(err) {
			return ErrAlreadyExists
		}
		return err
	}
	return nil
}

// List возвращает все маркеры в реестре с фильтром по префиксу (опционально).
func (s *Store) List(ctx context.Context, projectID, registryID, prefix string) ([]Marker, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, user_key, project_id, registry_id, full_path, created_by, created_at
		FROM folder_markers
		WHERE project_id = ? AND registry_id = ? AND full_path LIKE ?
		ORDER BY full_path`,
		projectID, registryID, prefix+"%")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Marker
	for rows.Next() {
		var m Marker
		var ts int64
		if err := rows.Scan(&m.ID, &m.UserKey, &m.ProjectID, &m.RegistryID, &m.FullPath, &m.CreatedBy, &ts); err != nil {
			return nil, err
		}
		m.CreatedAt = time.Unix(ts, 0)
		out = append(out, m)
	}
	return out, rows.Err()
}

// Delete удаляет маркер. Используется когда:
//   - в папку загружен первый файл (она стала реальной),
//   - пользователь удаляет папку через UI.
func (s *Store) Delete(ctx context.Context, projectID, registryID, fullPath string) error {
	cleaned, err := validation.CleanFolderPath(fullPath)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx,
		`DELETE FROM folder_markers WHERE project_id = ? AND registry_id = ? AND full_path = ?`,
		projectID, registryID, cleaned)
	return err
}

// DeletePrefix удаляет все маркеры под указанным префиксом (для рекурсивного удаления папки).
func (s *Store) DeletePrefix(ctx context.Context, projectID, registryID, prefix string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM folder_markers WHERE project_id = ? AND registry_id = ? AND full_path LIKE ?`,
		projectID, registryID, prefix+"%")
	return err
}

func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	// modernc.org/sqlite не оборачивает в типизированную ошибку — проверяем строку.
	msg := err.Error()
	return contains(msg, "UNIQUE constraint failed") || contains(msg, "constraint failed: UNIQUE")
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
