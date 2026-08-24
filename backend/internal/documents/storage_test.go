package documents

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/google/uuid"
)

func TestLocalStorageSaveOpenDelete(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir(), 1024)
	if err != nil {
		t.Fatalf("new storage: %v", err)
	}
	png := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 32)...)
	saved, err := storage.Save(context.Background(), uuid.New(), bytes.NewReader(png), "../passport?.png")
	if err != nil {
		t.Fatalf("save: %v", err)
	}
	if saved.MediaType != "image/png" || saved.Size != int64(len(png)) || saved.SHA256 == "" {
		t.Fatalf("unexpected metadata: %+v", saved)
	}
	if filepath.IsAbs(saved.StorageKey) || filepath.Base(saved.Name) != saved.Name {
		t.Fatalf("unsafe saved metadata: %+v", saved)
	}
	storedPath := filepath.Join(storage.root, filepath.FromSlash(saved.StorageKey))
	storedInfo, err := os.Stat(storedPath)
	if err != nil {
		t.Fatalf("stat stored document: %v", err)
	}
	if storedInfo.Mode().Perm() != sharedPrivateFileMode {
		t.Fatalf("stored document mode = %04o, want %04o", storedInfo.Mode().Perm(), sharedPrivateFileMode)
	}
	directoryInfo, err := os.Stat(filepath.Dir(storedPath))
	if err != nil {
		t.Fatalf("stat document directory: %v", err)
	}
	if directoryInfo.Mode().Perm() != sharedPrivateDirectoryMode {
		t.Fatalf("document directory mode = %04o, want %04o", directoryInfo.Mode().Perm(), sharedPrivateDirectoryMode)
	}
	file, err := storage.Open(context.Background(), saved.StorageKey)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	content, _ := io.ReadAll(file)
	_ = file.Close()
	if !bytes.Equal(content, png) {
		t.Fatal("stored content changed")
	}
	if err := storage.Delete(context.Background(), saved.StorageKey); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := storage.Open(context.Background(), saved.StorageKey); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected file removal, got %v", err)
	}
}

func TestLocalStorageRejectsTraversalTypeAndSize(t *testing.T) {
	storage, _ := NewLocalStorage(t.TempDir(), 1024)
	if _, err := storage.Open(context.Background(), "../secret"); !errors.Is(err, ErrInvalidKey) {
		t.Fatalf("expected traversal rejection, got %v", err)
	}
	if _, err := storage.Save(context.Background(), uuid.New(), bytes.NewReader([]byte("plain text")), "file.txt"); !errors.Is(err, ErrUnsupportedType) {
		t.Fatalf("expected type rejection, got %v", err)
	}
	oversized := append([]byte{0x89, 'P', 'N', 'G', 0x0d, 0x0a, 0x1a, 0x0a}, make([]byte, 1024)...)
	if _, err := storage.Save(context.Background(), uuid.New(), bytes.NewReader(oversized), "file.png"); !errors.Is(err, ErrTooLarge) {
		t.Fatalf("expected size rejection, got %v", err)
	}
}

func TestLocalStorageRejectsActivePDFAndMalwareSignature(t *testing.T) {
	storage, err := NewLocalStorage(t.TempDir(), 1024*1024)
	if err != nil {
		t.Fatalf("create storage: %v", err)
	}
	for name, body := range map[string][]byte{
		"active.pdf": []byte("%PDF-1.7\\n1 0 obj <</OpenAction 2 0 R /JavaScript (alert)>>"),
		"eicar.pdf":  []byte("%PDF-1.7\\nEICAR-STANDARD-ANTIVIRUS-TEST-FILE"),
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := storage.Save(context.Background(), uuid.New(), bytes.NewReader(body), name); !errors.Is(err, ErrUnsafeContent) {
				t.Fatalf("unsafe document error = %v, want %v", err, ErrUnsafeContent)
			}
		})
	}
}
