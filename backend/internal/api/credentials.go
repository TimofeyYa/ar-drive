// Извлечение пользовательских учётных данных Cloud.ru из заголовков запроса.
// SPA хранит зашифрованные креды в localStorage и при каждом запросе шлёт
// расшифрованные в заголовках. Это упрощает MVP и не требует серверного state.
package api

import (
	"errors"
	"net/http"

	"github.com/cloud-ru-tech/ar-drive/backend/internal/auth"
)

const (
	HeaderClientID     = "X-Cloudru-Client-Id"
	HeaderClientSecret = "X-Cloudru-Client-Secret"
	HeaderProjectID    = "X-Cloudru-Project-Id"
)

var errMissingCreds = errors.New("missing Cloud.ru credential headers")

func credsFromRequest(r *http.Request) (auth.Credentials, error) {
	c := auth.Credentials{
		ClientID:     r.Header.Get(HeaderClientID),
		ClientSecret: r.Header.Get(HeaderClientSecret),
		ProjectID:    r.Header.Get(HeaderProjectID),
	}
	if c.ClientID == "" || c.ClientSecret == "" || c.ProjectID == "" {
		return c, errMissingCreds
	}
	return c, nil
}
