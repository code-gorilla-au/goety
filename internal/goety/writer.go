package goety

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	ddb "github.com/code-gorilla-au/goety/internal/dynamodb"
)

// OSFile is the production file opener.
type OSFile struct{}

// Create opens name for writing, creating parent directories as needed.
func (w *OSFile) Create(name string, perm fs.FileMode) (io.WriteCloser, error) {
	cleanFile := filepath.Clean(name)
	if err := os.MkdirAll(filepath.Dir(cleanFile), 0750); err != nil {
		return nil, err
	}
	return os.OpenFile(cleanFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, perm)
}

// Open opens name for reading.
func (w *OSFile) Open(name string) (io.ReadCloser, error) {
	cleanFile := filepath.Clean(name)
	return os.Open(cleanFile)
}

type dumpWriter struct {
	w      io.WriteCloser
	closed bool
	first  bool
}

func (d *dumpWriter) Init(frw FileReadWriter, path string) (err error) {
	d.closed = false
	d.first = true

	d.w, err = frw.Create(path, 0644)
	if err != nil {
		return fmt.Errorf("create dump file: %w", err)
	}

	if _, err = d.w.Write([]byte("[")); err != nil {
		cErr := d.w.Close()
		if cErr != nil {
			return errors.Join(err, fmt.Errorf("error closing file: %w", cErr))
		}

		return fmt.Errorf("write opening bracket to dump file: %w", err)
	}

	return
}

func (d *dumpWriter) WriteItems(items []map[string]types.AttributeValue, rawOutput bool) error {
	for _, item := range items {
		var v any
		var err error
		if rawOutput {
			v, err = ddb.ConvertAVValue(item)
		} else {
			v, err = ddb.FlattenAttrValue(item)
		}
		if err != nil {
			return err
		}

		data, err := json.Marshal(v)
		if err != nil {
			return err
		}

		if !d.first {
			if _, err = d.w.Write([]byte(",")); err != nil {
				return err
			}
		}

		d.first = false

		if _, err = d.w.Write(data); err != nil {
			return err
		}
	}
	return nil
}

func (d *dumpWriter) Close() error {
	if !d.closed {

		_, err := d.w.Write([]byte("]"))
		if err != nil {
			cErr := d.w.Close()
			if cErr != nil {
				return errors.Join(err, fmt.Errorf("error closing file: %w", cErr))
			}

			return fmt.Errorf("write closing bracket to dump file: %w", err)
		}

		d.closed = true
	}

	return d.w.Close()
}
