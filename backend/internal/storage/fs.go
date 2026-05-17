package storage

import "os"

func mkdirAll(path string, mode os.FileMode) error {
	if path == "" || path == "." {
		return nil
	}
	return os.MkdirAll(path, mode)
}
