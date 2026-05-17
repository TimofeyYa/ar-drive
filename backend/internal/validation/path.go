// Package validation — общие валидаторы (path traversal, имена, и т.п.).
package validation

import (
	"errors"
	"path"
	"strings"
)

var (
	ErrEmptyPath        = errors.New("path is empty")
	ErrPathTraversal    = errors.New("path contains traversal segments (..)")
	ErrNullByte         = errors.New("path contains null byte")
	ErrBackslash        = errors.New("path contains backslash")
	ErrLeadingSlash     = errors.New("path must not start with /")
	ErrAbsolute         = errors.New("path must be relative")
	ErrInvalidComponent = errors.New("path contains invalid component")
)

// CleanFilePath санитизирует относительный путь файла. Возвращает
// нормализованный путь без ведущего "/", разделителем "/" и без traversal.
//
// Допускаются UTF-8 имена, точки в названиях, дефисы, подчёркивания, пробелы.
// Запрещены: "..", "\0", "\\", абсолютные пути, пустые компоненты.
func CleanFilePath(p string) (string, error) {
	if p == "" {
		return "", ErrEmptyPath
	}
	if strings.ContainsRune(p, '\x00') {
		return "", ErrNullByte
	}
	if strings.Contains(p, "\\") {
		return "", ErrBackslash
	}
	if strings.HasPrefix(p, "/") {
		return "", ErrLeadingSlash
	}

	// path.Clean нормализует «./», «//», но не запрещает "..".
	cleaned := path.Clean(p)
	if cleaned == "." || cleaned == "" {
		return "", ErrEmptyPath
	}
	if path.IsAbs(cleaned) {
		return "", ErrAbsolute
	}
	for _, part := range strings.Split(cleaned, "/") {
		if part == ".." {
			return "", ErrPathTraversal
		}
		if part == "" {
			return "", ErrInvalidComponent
		}
	}
	return cleaned, nil
}

// CleanFolderPath работает как CleanFilePath, но добавляет завершающий "/"
// — папка в Generic-реестре определяется именно как префикс.
func CleanFolderPath(p string) (string, error) {
	p = strings.TrimSuffix(p, "/")
	cleaned, err := CleanFilePath(p)
	if err != nil {
		return "", err
	}
	return cleaned + "/", nil
}
