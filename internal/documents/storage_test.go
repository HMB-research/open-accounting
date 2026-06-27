package documents

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type failingDocumentReader struct{}

func (failingDocumentReader) Read([]byte) (int, error) {
	return 0, errors.New("read failed")
}

func TestLocalStoreValidationAndFailureEdges(t *testing.T) {
	t.Parallel()

	if store, err := NewLocalStore(" "); err == nil || store != nil || !strings.Contains(err.Error(), "root directory is required") {
		t.Fatalf("expected blank root error, store=%#v err=%v", store, err)
	}

	fileRoot := filepath.Join(t.TempDir(), "documents-root")
	if err := os.WriteFile(fileRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write file root: %v", err)
	}
	if store, err := NewLocalStore(fileRoot); err == nil || store != nil || !strings.Contains(err.Error(), "create documents root") {
		t.Fatalf("expected file root error, store=%#v err=%v", store, err)
	}

	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}

	if err := store.Save(context.Background(), "../escape.txt", strings.NewReader("x")); err == nil || !strings.Contains(err.Error(), "invalid document storage key") {
		t.Fatalf("expected invalid key save error, got %v", err)
	}
	if err := store.Save(context.Background(), "tenant/read-error.txt", failingDocumentReader{}); err == nil || !strings.Contains(err.Error(), "write document") {
		t.Fatalf("expected read failure from save, got %v", err)
	}
	if _, err := os.Stat(filepath.Join(store.rootDir, "tenant", "read-error.txt.tmp")); !os.IsNotExist(err) {
		t.Fatalf("expected failed temp file cleanup, stat err=%v", err)
	}

	if err := os.Mkdir(filepath.Join(store.rootDir, "tenant", "directory-target"), 0o750); err != nil {
		t.Fatalf("create directory target: %v", err)
	}
	if err := store.Save(context.Background(), "tenant/directory-target", strings.NewReader("payload")); err == nil || !strings.Contains(err.Error(), "move document into place") {
		t.Fatalf("expected rename over directory error, got %v", err)
	}

	blockedRoot := filepath.Join(t.TempDir(), "blocked-root")
	if err := os.WriteFile(blockedRoot, []byte("not a directory"), 0o600); err != nil {
		t.Fatalf("write blocked root: %v", err)
	}
	blockedStore := &LocalStore{rootDir: blockedRoot}
	if err := blockedStore.Save(context.Background(), "child/doc.txt", strings.NewReader("payload")); err == nil || !strings.Contains(err.Error(), "create documents directory") {
		t.Fatalf("expected create directory failure, got %v", err)
	}

	if reader, err := store.Open(context.Background(), "../escape.txt"); err == nil || reader != nil || !strings.Contains(err.Error(), "invalid document storage key") {
		t.Fatalf("expected invalid key open error, reader=%#v err=%v", reader, err)
	}
	if reader, err := store.Open(context.Background(), "tenant/missing.txt"); err == nil || reader != nil || !strings.Contains(err.Error(), "open document") {
		t.Fatalf("expected missing document open error, reader=%#v err=%v", reader, err)
	}

	if err := store.Delete(context.Background(), "../escape.txt"); err == nil || !strings.Contains(err.Error(), "invalid document storage key") {
		t.Fatalf("expected invalid key delete error, got %v", err)
	}
	if err := store.Delete(context.Background(), "tenant/missing.txt"); err != nil {
		t.Fatalf("delete missing document should be idempotent, got %v", err)
	}
	nonEmptyDir := filepath.Join(store.rootDir, "tenant", "non-empty")
	if err := os.MkdirAll(nonEmptyDir, 0o750); err != nil {
		t.Fatalf("create non-empty directory: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nonEmptyDir, "child.txt"), []byte("payload"), 0o600); err != nil {
		t.Fatalf("write non-empty child: %v", err)
	}
	if err := store.Delete(context.Background(), "tenant/non-empty"); err == nil || !strings.Contains(err.Error(), "delete document") {
		t.Fatalf("expected delete failure for non-empty directory, got %v", err)
	}
}

func TestLocalStoreRoundTrip(t *testing.T) {
	t.Parallel()

	store, err := NewLocalStore(t.TempDir())
	if err != nil {
		t.Fatalf("NewLocalStore failed: %v", err)
	}
	if _, err := store.resolvePath(" "); err == nil || !strings.Contains(err.Error(), "storage key is required") {
		t.Fatalf("expected blank key error, got %v", err)
	}

	key := "tenant-1/2026/03/document.txt"
	if err := store.Save(context.Background(), key, strings.NewReader("payload")); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	reader, err := store.Open(context.Background(), key)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	payload, err := io.ReadAll(reader)
	if closeErr := reader.Close(); closeErr != nil && err == nil {
		err = closeErr
	}
	if err != nil {
		t.Fatalf("read stored document: %v", err)
	}
	if string(payload) != "payload" {
		t.Fatalf("unexpected stored payload %q", string(payload))
	}

	if err := store.Delete(context.Background(), key); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if _, err := store.Open(context.Background(), key); err == nil {
		t.Fatal("expected deleted document to be missing")
	}
}
