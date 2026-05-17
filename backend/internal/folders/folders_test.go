package folders

import (
	"context"
	"database/sql"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	_ "modernc.org/sqlite"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	_, err = db.Exec(`
		CREATE TABLE folder_markers (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			user_key TEXT NOT NULL,
			project_id TEXT NOT NULL,
			registry_id TEXT NOT NULL,
			full_path TEXT NOT NULL,
			created_by TEXT NOT NULL,
			created_at INTEGER NOT NULL,
			UNIQUE (project_id, registry_id, full_path)
		)`)
	require.NoError(t, err)
	return NewStore(db)
}

func TestStore_CreateListDelete(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	m := Marker{
		UserKey: "u", ProjectID: "p", RegistryID: "r",
		FullPath: "docs/architecture", CreatedBy: "alice",
	}
	require.NoError(t, s.Create(ctx, m))

	list, err := s.List(ctx, "p", "r", "")
	require.NoError(t, err)
	require.Len(t, list, 1)
	assert.Equal(t, "docs/architecture/", list[0].FullPath)

	// Дубликат — конфликт.
	err = s.Create(ctx, m)
	require.ErrorIs(t, err, ErrAlreadyExists)

	require.NoError(t, s.Delete(ctx, "p", "r", "docs/architecture/"))
	list, err = s.List(ctx, "p", "r", "")
	require.NoError(t, err)
	assert.Empty(t, list)
}

func TestStore_RejectsTraversal(t *testing.T) {
	s := newTestStore(t)
	err := s.Create(context.Background(), Marker{
		UserKey: "u", ProjectID: "p", RegistryID: "r",
		FullPath: "../etc/passwd", CreatedBy: "evil",
	})
	require.Error(t, err)
}

func TestStore_DeletePrefix(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	for _, p := range []string{"a", "a/b", "a/b/c", "x"} {
		require.NoError(t, s.Create(ctx, Marker{
			UserKey: "u", ProjectID: "p", RegistryID: "r", FullPath: p, CreatedBy: "alice",
		}))
	}
	require.NoError(t, s.DeletePrefix(ctx, "p", "r", "a/"))

	list, err := s.List(ctx, "p", "r", "")
	require.NoError(t, err)
	// должны остаться "a/" и "x/"
	require.Len(t, list, 2)
}
