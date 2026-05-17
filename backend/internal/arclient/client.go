// Package arclient — REST-обёртка над Cloud.ru Artifact Registry API.
//
// Документация-источник:
//   - https://cloud.ru/docs/artifact-registry-evolution/ug/topics/api__artifact-registry?source-platform=Evolution
//   - https://cloud.ru/docs/artifact-registry-evolution/ug/topics/guides__generic
//
// ВНИМАНИЕ: Точные пути file-эндпоинтов (push/pull/list/delete внутри Generic-реестра)
// в публичной документации не раскрыты — их следует подтвердить из актуального swagger
// личного кабинета Cloud.ru и при необходимости перегенерировать клиент через
// `make gen-openapi`. Здесь использованы наиболее вероятные REST-конвенции:
//
//   GET    /v1/registries?projectId=...
//   GET    /v1/registries/{registryId}/generic/files?path=...&pageSize=...&pageToken=...
//   PUT    /v1/registries/{registryId}/generic/files/{path}    (upload, body = octet-stream)
//   GET    /v1/registries/{registryId}/generic/files/{path}    (download)
//   DELETE /v1/registries/{registryId}/generic/files/{path}    (delete)
//
// Если в реальном API пути отличаются — поменять в одном месте, в этом файле.
// См. docs/decisions.md.
package arclient

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

type Client struct {
	baseURL string
	http    *http.Client
}

func New(baseURL string, httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 5 * time.Minute}
	}
	return &Client{baseURL: strings.TrimRight(baseURL, "/"), http: httpClient}
}

// --- Domain types (модели Cloud.ru AR) ---

type Registry struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	ProjectID    string    `json:"projectId"`
	RegistryType string    `json:"registryType"` // DOCKER, DEBIAN, RPM, GENERIC
	Status       string    `json:"status"`       // CREATING, ACTIVE, ERROR
	IsPublic     bool      `json:"isPublic"`
	Tariff       string    `json:"tariff"`       // BASIC, PREMIUM
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}

type ListRegistriesResponse struct {
	Items         []Registry `json:"items"`
	NextPageToken string     `json:"nextPageToken"`
}

type File struct {
	Path        string    `json:"path"`        // полный путь внутри реестра
	Name        string    `json:"name"`        // базовое имя
	Size        int64     `json:"size"`        // байты
	SHA256      string    `json:"sha256,omitempty"`
	ContentType string    `json:"contentType,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

type ListFilesResponse struct {
	Items         []File `json:"items"`
	NextPageToken string `json:"nextPageToken"`
}

// --- Ошибки ---

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string { return fmt.Sprintf("AR API %d: %s", e.StatusCode, e.Message) }

var (
	ErrNotFound     = errors.New("not found")
	ErrUnauthorized = errors.New("unauthorized")
	ErrConflict     = errors.New("conflict")
)

// --- HTTP helpers ---

func (c *Client) do(ctx context.Context, method, urlPath string, token string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+urlPath, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AR HTTP: %w", err)
	}
	return resp, nil
}

func decodeError(resp *http.Response) error {
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return ErrUnauthorized
	case http.StatusNotFound:
		return ErrNotFound
	case http.StatusConflict:
		return ErrConflict
	}
	return &APIError{StatusCode: resp.StatusCode, Message: string(body)}
}

// --- Registries ---

func (c *Client) ListRegistries(ctx context.Context, token, projectID, pageToken string, pageSize int) (*ListRegistriesResponse, error) {
	q := url.Values{}
	q.Set("projectId", projectID)
	if pageSize > 0 {
		q.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	resp, err := c.do(ctx, http.MethodGet, "/v1/registries?"+q.Encode(), token, nil, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}
	defer resp.Body.Close()
	var out ListRegistriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode list registries: %w", err)
	}
	return &out, nil
}

// --- Generic files ---

// ListFiles возвращает файлы внутри реестра, опционально фильтруя по префиксу path.
func (c *Client) ListFiles(ctx context.Context, token, registryID, pathPrefix, pageToken string, pageSize int) (*ListFilesResponse, error) {
	q := url.Values{}
	if pathPrefix != "" {
		q.Set("path", pathPrefix)
	}
	if pageSize > 0 {
		q.Set("pageSize", strconv.Itoa(pageSize))
	}
	if pageToken != "" {
		q.Set("pageToken", pageToken)
	}
	urlPath := fmt.Sprintf("/v1/registries/%s/generic/files", url.PathEscape(registryID))
	if encoded := q.Encode(); encoded != "" {
		urlPath += "?" + encoded
	}
	resp, err := c.do(ctx, http.MethodGet, urlPath, token, nil, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, decodeError(resp)
	}
	defer resp.Body.Close()
	var out ListFilesResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode list files: %w", err)
	}
	return &out, nil
}

// UploadFile загружает файл по пути filePath в реестр.
// Тело — поток (io.Reader). Возвращает метаданные созданного файла.
func (c *Client) UploadFile(ctx context.Context, token, registryID, filePath string, contentType string, body io.Reader, size int64) (*File, error) {
	urlPath := fmt.Sprintf("/v1/registries/%s/generic/files/%s",
		url.PathEscape(registryID), pathEscapeMulti(filePath))

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.baseURL+urlPath, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	req.Header.Set("Content-Type", contentType)
	if size > 0 {
		req.ContentLength = size
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("AR upload: %w", err)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, decodeError(resp)
	}
	defer resp.Body.Close()
	var out File
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		// API может вернуть пустое тело — это не критично.
		return &File{Path: filePath, Size: size, ContentType: contentType, UpdatedAt: time.Now()}, nil
	}
	return &out, nil
}

// DownloadFile возвращает поток с содержимым файла.
// Обязательно вызвать resp.Body.Close() после использования.
func (c *Client) DownloadFile(ctx context.Context, token, registryID, filePath string) (*http.Response, error) {
	urlPath := fmt.Sprintf("/v1/registries/%s/generic/files/%s",
		url.PathEscape(registryID), pathEscapeMulti(filePath))
	resp, err := c.do(ctx, http.MethodGet, urlPath, token, nil, "")
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		err := decodeError(resp)
		return nil, err
	}
	return resp, nil
}

// DeleteFile удаляет файл из Generic-реестра.
func (c *Client) DeleteFile(ctx context.Context, token, registryID, filePath string) error {
	urlPath := fmt.Sprintf("/v1/registries/%s/generic/files/%s",
		url.PathEscape(registryID), pathEscapeMulti(filePath))
	resp, err := c.do(ctx, http.MethodDelete, urlPath, token, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return decodeError(resp)
	}
	return nil
}

// pathEscapeMulti кодирует каждый сегмент пути, сохраняя разделители "/".
func pathEscapeMulti(p string) string {
	parts := strings.Split(p, "/")
	for i, s := range parts {
		parts[i] = url.PathEscape(s)
	}
	return strings.Join(parts, "/")
}

// MustEncodeJSON — мини-хелпер для тестов и фикстур.
func MustEncodeJSON(v any) []byte {
	var buf bytes.Buffer
	_ = json.NewEncoder(&buf).Encode(v)
	return buf.Bytes()
}
