package filestore

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Local struct{ Root string }

func (l Local) Save(_ context.Context, dir, name, _ string, r io.Reader) (string, error) {
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

func (l Local) inside(path string) bool {
	root, err := filepath.Abs(l.Root)
	if err != nil {
		return false
	}
	abs, err := filepath.Abs(path)
	return err == nil && strings.HasPrefix(abs, root+string(os.PathSeparator))
}

func (l Local) Read(_ context.Context, path string) ([]byte, error) {
	if !l.inside(path) {
		return nil, errors.New("path outside store")
	}
	return os.ReadFile(path)
}

func (l Local) Remove(_ context.Context, path string) error {
	if !l.inside(path) {
		return errors.New("path outside store")
	}
	return os.Remove(path)
}
