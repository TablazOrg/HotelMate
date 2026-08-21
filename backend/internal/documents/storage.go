package documents

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/google/uuid"
)

var (
	ErrTooLarge        = errors.New("document exceeds size limit")
	ErrUnsupportedType = errors.New("document type is not allowed")
	ErrInvalidKey      = errors.New("document storage key is invalid")
)

type SavedDocument struct {
	StorageKey string
	Name       string
	MediaType  string
	Size       int64
	SHA256     string
}

type ReadSeekCloser interface {
	io.Reader
	io.Seeker
	io.Closer
}

type Storage interface {
	Save(context.Context, uuid.UUID, io.Reader, string) (SavedDocument, error)
	Open(context.Context, string) (ReadSeekCloser, error)
	Delete(context.Context, string) error
}

type LocalStorage struct {
	root     string
	maxBytes int64
}

func NewLocalStorage(root string, maxBytes int64) (*LocalStorage, error) {
	absolute, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve uploads directory: %w", err)
	}
	if maxBytes < 1024 {
		return nil, errors.New("document size limit is too small")
	}
	if err := os.MkdirAll(absolute, 0o700); err != nil {
		return nil, fmt.Errorf("create uploads directory: %w", err)
	}
	return &LocalStorage{root: absolute, maxBytes: maxBytes}, nil
}

func (s *LocalStorage) Save(ctx context.Context, hotelID uuid.UUID, source io.Reader, originalName string) (SavedDocument, error) {
	if err := ctx.Err(); err != nil {
		return SavedDocument{}, err
	}
	data, err := io.ReadAll(io.LimitReader(source, s.maxBytes+1))
	if err != nil {
		return SavedDocument{}, fmt.Errorf("read document: %w", err)
	}
	if int64(len(data)) > s.maxBytes {
		return SavedDocument{}, ErrTooLarge
	}
	if len(data) == 0 {
		return SavedDocument{}, ErrUnsupportedType
	}
	mediaType := http.DetectContentType(data)
	extension, ok := allowedExtension(mediaType, data)
	if !ok {
		return SavedDocument{}, ErrUnsupportedType
	}
	key := filepath.ToSlash(filepath.Join("check-in", hotelID.String(), uuid.NewString()+extension))
	target, err := s.resolve(key)
	if err != nil {
		return SavedDocument{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return SavedDocument{}, fmt.Errorf("create document directory: %w", err)
	}
	if err := os.WriteFile(target, data, 0o600); err != nil {
		return SavedDocument{}, fmt.Errorf("write document: %w", err)
	}
	digest := sha256.Sum256(data)
	return SavedDocument{
		StorageKey: key,
		Name:       safeName(originalName, extension),
		MediaType:  mediaType,
		Size:       int64(len(data)),
		SHA256:     hex.EncodeToString(digest[:]),
	}, nil
}

func (s *LocalStorage) Open(ctx context.Context, key string) (ReadSeekCloser, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	target, err := s.resolve(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if err != nil {
		return nil, err
	}
	return file, nil
}

func (s *LocalStorage) Delete(ctx context.Context, key string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	target, err := s.resolve(key)
	if err != nil {
		return err
	}
	if err := os.Remove(target); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

func (s *LocalStorage) resolve(key string) (string, error) {
	if key == "" || filepath.IsAbs(key) {
		return "", ErrInvalidKey
	}
	target := filepath.Clean(filepath.Join(s.root, filepath.FromSlash(key)))
	relative, err := filepath.Rel(s.root, target)
	if err != nil || relative == "." || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", ErrInvalidKey
	}
	return target, nil
}

func allowedExtension(mediaType string, data []byte) (string, bool) {
	switch mediaType {
	case "image/jpeg":
		return ".jpg", true
	case "image/png":
		return ".png", true
	case "application/pdf":
		if bytes.HasPrefix(data, []byte("%PDF-")) {
			return ".pdf", true
		}
	}
	return "", false
}

func safeName(originalName, extension string) string {
	name := filepath.Base(strings.TrimSpace(originalName))
	name = strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '.' || r == '-' || r == '_' || r == ' ' {
			return r
		}
		return '_'
	}, name)
	if name == "" || name == "." {
		name = "identity-document" + extension
	}
	runes := []rune(name)
	if len(runes) > 180 {
		name = string(runes[:180])
	}
	return name
}
