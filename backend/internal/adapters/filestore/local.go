package filestore

import (
	"context"
	"io"
	"os"
	"path/filepath"
)

type Local struct{ Root string }

func (l Local) Save(_ context.Context, dir, name string, r io.Reader) (string, error) {
	folder := filepath.Join(l.Root, filepath.Base(dir))
	if err := os.MkdirAll(folder, 0o750); err != nil {
		return "", err
	}
	path := filepath.Join(folder, filepath.Base(name))
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o640)
	if err != nil {
		return "", err
	}
	defer f.Close()
	if _, err := io.Copy(f, io.LimitReader(r, 8<<20+1)); err != nil {
		return "", err
	}
	return path, nil
}

func (l Local) Remove(path string) error { return os.Remove(path) }
