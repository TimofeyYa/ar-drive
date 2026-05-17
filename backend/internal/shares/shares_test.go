package shares

import (
	"context"
	"database/sql"
	"testing"
	"time"

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
		CREATE TABLE share_links (
			short_id TEXT PRIMARY KEY,
			user_key TEXT NOT NULL,
			project_id TEXT NOT NULL,
			registry_id TEXT NOT NULL,
			file_path TEXT NOT NULL,
			password_hash TEXT,
			max_downloads INTEGER,
			downloads INTEGER NOT NULL DEFAULT 0,
			expires_at INTEGER,
			revoked INTEGER NOT NULL DEFAULT 0,
			created_at INTEGER NOT NULL
		)`)
	require.NoError(t, err)
	return NewStore(db)
}

func TestShares_HappyPath(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	l, err := s.Create(ctx, CreateInput{
		UserKey: "u", ProjectID: "p", RegistryID: "r",
		FilePath: "dir/file.txt", TTL: time.Hour, MaxDownloads: 5,
	})
	require.NoError(t, err)
	assert.Len(t, l.ShortID, 12)

	got, err := s.Validate(ctx, l.ShortID, "")
	require.NoError(t, err)
	assert.Equal(t, l.ShortID, got.ShortID)
}

func TestShares_PasswordProtected(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	l, err := s.Create(ctx, CreateInput{
		UserKey: "u", ProjectID: "p", RegistryID: "r",
		FilePath: "f.txt", Password: "hunter2", TTL: time.Hour,
	})
	require.NoError(t, err)

	_, err = s.Validate(ctx, l.ShortID, "")
	require.ErrorIs(t, err, ErrPasswordReq)

	_, err = s.Validate(ctx, l.ShortID, "wrong")
	require.ErrorIs(t, err, ErrPasswordWrong)

	_, err = s.Validate(ctx, l.ShortID, "hunter2")
	require.NoError(t, err)
}

func TestShares_RevokeAndExpire(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	l, err := s.Create(ctx, CreateInput{
		UserKey: "u", ProjectID: "p", RegistryID: "r",
		FilePath: "f.txt", TTL: time.Hour,
	})
	require.NoError(t, err)

	require.NoError(t, s.Revoke(ctx, "u", l.ShortID))
	_, err = s.Validate(ctx, l.ShortID, "")
	require.ErrorIs(t, err, ErrRevoked)

	require.ErrorIs(t, s.Revoke(ctx, "u", "nonexistent"), ErrNotFound)
}

func TestShares_DownloadsExhausted(t *testing.T) {
	s := newTestStore(t)
	ctx := context.Background()

	l, err := s.Create(ctx, CreateInput{
		UserKey: "u", ProjectID: "p", RegistryID: "r",
		FilePath: "f.txt", MaxDownloads: 2,
	})
	require.NoError(t, err)

	require.NoError(t, s.IncrementDownloads(ctx, l.ShortID))
	require.NoError(t, s.IncrementDownloads(ctx, l.ShortID))

	_, err = s.Validate(ctx, l.ShortID, "")
	require.ErrorIs(t, err, ErrExhausted)
}
