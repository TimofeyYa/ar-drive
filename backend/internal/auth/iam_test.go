package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUserKey_StableAndOpaque(t *testing.T) {
	c := Credentials{ClientID: "abc", ClientSecret: "secret", ProjectID: "p1"}
	k1 := c.UserKey()
	k2 := c.UserKey()
	assert.Equal(t, k1, k2)
	assert.Len(t, k1, 64)              // sha256 hex
	assert.NotContains(t, k1, "secret") // не утекает
}

func TestFetchToken_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, "/api/v1/auth/token", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var body map[string]string
		_ = json.NewDecoder(r.Body).Decode(&body)
		assert.Equal(t, "k", body["keyId"])
		assert.Equal(t, "s", body["secret"])

		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "TOKEN",
			"expires_in":   3600,
			"token_type":   "Bearer",
		})
	}))
	defer srv.Close()

	client := NewIAMClient(srv.URL, nil)
	tok, err := client.FetchToken(context.Background(),
		Credentials{ClientID: "k", ClientSecret: "s", ProjectID: "11111111-1111-1111-1111-111111111111"},
		60*time.Second,
	)
	require.NoError(t, err)
	assert.Equal(t, "TOKEN", tok.AccessToken)
	assert.True(t, tok.Valid())
}

func TestFetchToken_Invalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	client := NewIAMClient(srv.URL, nil)
	_, err := client.FetchToken(context.Background(),
		Credentials{ClientID: "k", ClientSecret: "bad", ProjectID: "p"},
		60*time.Second)
	require.ErrorIs(t, err, ErrInvalidCredentials)
}

func TestTokenCache_Reuses(t *testing.T) {
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": "T", "expires_in": 3600, "token_type": "Bearer",
		})
	}))
	defer srv.Close()

	cache := NewTokenCache(NewIAMClient(srv.URL, nil), 60*time.Second)
	creds := Credentials{ClientID: "k", ClientSecret: "s", ProjectID: "p"}

	for i := 0; i < 5; i++ {
		_, err := cache.Get(context.Background(), creds)
		require.NoError(t, err)
	}
	assert.Equal(t, 1, calls, "должен закешировать токен и не дергать IAM повторно")
}
