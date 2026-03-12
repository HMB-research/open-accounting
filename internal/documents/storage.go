package documents

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

type Store interface {
	Save(ctx context.Context, key string, content io.Reader) error
	Open(ctx context.Context, key string) (io.ReadCloser, error)
	Delete(ctx context.Context, key string) error
}

type LocalStore struct {
	rootDir string
}

func NewLocalStore(rootDir string) (*LocalStore, error) {
	if strings.TrimSpace(rootDir) == "" {
		return nil, fmt.Errorf("documents root directory is required")
	}
	if err := os.MkdirAll(rootDir, 0o755); err != nil {
		return nil, fmt.Errorf("create documents root: %w", err)
	}
	return &LocalStore{rootDir: rootDir}, nil
}

func (s *LocalStore) Save(_ context.Context, key string, content io.Reader) error {
	targetPath, err := s.resolvePath(key)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(targetPath), 0o755); err != nil {
		return fmt.Errorf("create documents directory: %w", err)
	}

	tempPath := targetPath + ".tmp"
	file, err := os.Create(tempPath)
	if err != nil {
		return fmt.Errorf("create temp document: %w", err)
	}

	if _, err := io.Copy(file, content); err != nil {
		_ = file.Close()
		_ = os.Remove(tempPath)
		return fmt.Errorf("write document: %w", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("close document: %w", err)
	}

	if err := os.Rename(tempPath, targetPath); err != nil {
		_ = os.Remove(tempPath)
		return fmt.Errorf("move document into place: %w", err)
	}

	return nil
}

func (s *LocalStore) Open(_ context.Context, key string) (io.ReadCloser, error) {
	targetPath, err := s.resolvePath(key)
	if err != nil {
		return nil, err
	}

	file, err := os.Open(targetPath)
	if err != nil {
		return nil, fmt.Errorf("open document: %w", err)
	}
	return file, nil
}

func (s *LocalStore) Delete(_ context.Context, key string) error {
	targetPath, err := s.resolvePath(key)
	if err != nil {
		return err
	}

	if err := os.Remove(targetPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("delete document: %w", err)
	}
	return nil
}

func (s *LocalStore) resolvePath(key string) (string, error) {
	cleanKey := filepath.Clean(strings.TrimSpace(key))
	if cleanKey == "." || cleanKey == "" {
		return "", fmt.Errorf("document storage key is required")
	}

	targetPath := filepath.Join(s.rootDir, cleanKey)
	rootClean := filepath.Clean(s.rootDir)
	if targetPath != rootClean && !strings.HasPrefix(targetPath, rootClean+string(filepath.Separator)) {
		return "", fmt.Errorf("invalid document storage key")
	}

	return targetPath, nil
}
