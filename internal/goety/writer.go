package goety

import (
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile is the production file opener.
type WriteFile struct{}

// Create opens name for writing, creating parent directories as needed.
func (w *WriteFile) Create(name string, perm fs.FileMode) (io.WriteCloser, error) {
	cleanFile := filepath.Clean(name)
	if err := os.MkdirAll(filepath.Dir(cleanFile), 0750); err != nil {
		return nil, err
	}
	return os.OpenFile(cleanFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
}

// Open opens name for reading.
func (w *WriteFile) Open(name string) (io.ReadCloser, error) {
	cleanFile := filepath.Clean(name)
	return os.Open(cleanFile)
}
