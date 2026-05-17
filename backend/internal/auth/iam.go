// Package auth реализует аутентификацию в Cloud.ru IAM
// (https://iam.api.cloud.ru/api/v1/auth/token) и кеширование access-токенов.
//
// API IAM Cloud.ru:
//
//	POST /api/v1/auth/token
//	Content-Type: application/json
//	{ "keyId": "...", "secret": "..." }
//
// Срок жизни токена — 1 час, не настраивается. Используется как
// `Authorization: Bearer <access_token>` для всех вызовов AR API.
//
// Документация: https://cloud.ru/docs/console_api/ug/topics/guides__auth_api
package auth

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
)

// Credentials — учётные данные пользователя (Key ID + Key Secret сервисного аккаунта)
// плюс идентификатор проекта (нужен для всех запросов AR).
type Credentials struct {
	ClientID     string `json:"client_id"     validate:"required,min=3"`
	ClientSecret string `json:"client_secret" validate:"required,min=3"`
	ProjectID    string `json:"project_id"    validate:"required,uuid"`
}

// UserKey — стабильный идентификатор пользователя (для логов и сессий).
// Не раскрывает секрет.
func (c Credentials) UserKey() string {
	h := sha256.Sum256([]byte(c.ClientID + "|" + c.ProjectID))
	return hex.EncodeToString(h[:])
}

// Token — кешируемый IAM access-token.
type Token struct {
	AccessToken string
	ExpiresAt   time.Time
}

func (t Token) Valid() bool {
	return t.AccessToken != "" && time.Now().Before(t.ExpiresAt)
}

// IAMClient — клиент к IAM Cloud.ru.
type IAMClient struct {
	baseURL string
	http    *http.Client
}

func NewIAMClient(baseURL string, httpClient *http.Client) *IAMClient {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	return &IAMClient{baseURL: baseURL, http: httpClient}
}

type tokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresIn   int    `json:"expires_in"`
	TokenType   string `json:"token_type"`
}

type iamErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// FetchToken обменивает (clientID, secret) на access-token.
// Возвращает ErrInvalidCredentials для 401/403.
var ErrInvalidCredentials = errors.New("invalid Cloud.ru credentials")

func (c *IAMClient) FetchToken(ctx context.Context, creds Credentials, safetyMargin time.Duration) (Token, error) {
	body, _ := json.Marshal(map[string]string{
		"keyId":  creds.ClientID,
		"secret": creds.ClientSecret,
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.baseURL+"/api/v1/auth/token", bytes.NewReader(body))
	if err != nil {
		return Token{}, err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return Token{}, fmt.Errorf("call IAM: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return Token{}, ErrInvalidCredentials
	}
	if resp.StatusCode != http.StatusOK {
		var ie iamErrorResponse
		_ = json.Unmarshal(respBody, &ie)
		return Token{}, fmt.Errorf("IAM error %d: %s", resp.StatusCode, ie.Message)
	}

	var tr tokenResponse
	if err := json.Unmarshal(respBody, &tr); err != nil {
		return Token{}, fmt.Errorf("decode token response: %w", err)
	}
	if tr.AccessToken == "" {
		return Token{}, errors.New("empty access_token from IAM")
	}
	// expires_in может отсутствовать — fallback на 1 час по докам.
	ttl := time.Duration(tr.ExpiresIn) * time.Second
	if ttl <= 0 {
		ttl = time.Hour
	}
	return Token{
		AccessToken: tr.AccessToken,
		ExpiresAt:   time.Now().Add(ttl - safetyMargin),
	}, nil
}

// TokenCache — потокобезопасный in-memory кеш токенов с lazy refresh.
type TokenCache struct {
	mu     sync.Mutex
	tokens map[string]Token
	client *IAMClient
	margin time.Duration
}

func NewTokenCache(client *IAMClient, safetyMargin time.Duration) *TokenCache {
	return &TokenCache{
		tokens: make(map[string]Token),
		client: client,
		margin: safetyMargin,
	}
}

// Get возвращает валидный токен. Если в кеше его нет / он истёк — запрашивает новый.
func (c *TokenCache) Get(ctx context.Context, creds Credentials) (string, error) {
	key := creds.UserKey()

	c.mu.Lock()
	if t, ok := c.tokens[key]; ok && t.Valid() {
		c.mu.Unlock()
		return t.AccessToken, nil
	}
	c.mu.Unlock()

	t, err := c.client.FetchToken(ctx, creds, c.margin)
	if err != nil {
		return "", err
	}

	c.mu.Lock()
	c.tokens[key] = t
	c.mu.Unlock()
	return t.AccessToken, nil
}

// Invalidate удаляет токен из кеша (например, при logout).
func (c *TokenCache) Invalidate(creds Credentials) {
	c.mu.Lock()
	delete(c.tokens, creds.UserKey())
	c.mu.Unlock()
}

// Size — для метрик.
func (c *TokenCache) Size() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.tokens)
}
