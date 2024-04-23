package goety

import (
	"io/fs"
	"os"
	"path/filepath"
)

// WriteFile saves data to a file.
func (w *WriteFile) WriteFile(name string, data []byte, perm fs.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(name), 0750); err != nil {
		return err
	}
	return os.WriteFile(name, data, perm)
}
