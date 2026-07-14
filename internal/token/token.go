// Package token issues and consumes short-lived, single-use join tokens that
// bootstrap controller enrollment (the Talos trustd pattern). Tokens are held
// in a JSON file guarded by an OS file lock so the `token new` CLI and the
// running daemon can share one store across processes.
package token

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

// Errors returned by Consume.
var (
	ErrUnknownToken = errors.New("token: unknown token")
	ErrExpired      = errors.New("token: expired")
)

type record struct {
	Role   string    `json:"role"`
	Expiry time.Time `json:"expiry"`
}

// FileStore is a cross-process, single-use token store backed by a JSON file.
type FileStore struct {
	path string
	lock *flock.Flock
	now  func() time.Time
}

// NewFileStore returns a store persisting tokens at path.
func NewFileStore(path string) *FileStore {
	return &FileStore{path: path, lock: flock.New(path + ".lock"), now: time.Now}
}

// Issue mints a new single-use token granting role, valid for ttl.
func (s *FileStore) Issue(role string, ttl time.Duration) (string, error) {
	value, err := randToken()
	if err != nil {
		return "", err
	}
	err = s.mutate(func(m map[string]record) error {
		m[value] = record{Role: role, Expiry: s.now().Add(ttl)}
		return nil
	})
	if err != nil {
		return "", err
	}
	return value, nil
}

// Consume validates and permanently removes a token, returning the role it
// granted. A token is consumed even when expired, so it can never be reused.
func (s *FileStore) Consume(value string) (string, error) {
	var role string
	err := s.mutate(func(m map[string]record) error {
		rec, ok := m[value]
		if !ok {
			return ErrUnknownToken
		}
		delete(m, value)
		if s.now().After(rec.Expiry) {
			return ErrExpired
		}
		role = rec.Role
		return nil
	})
	if err != nil {
		return "", err
	}
	return role, nil
}

func (s *FileStore) mutate(fn func(map[string]record) error) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return fmt.Errorf("token: mkdir: %w", err)
	}
	if err := s.lock.Lock(); err != nil {
		return fmt.Errorf("token: lock: %w", err)
	}
	defer func() { _ = s.lock.Unlock() }()

	m, err := s.read()
	if err != nil {
		return err
	}
	fnErr := fn(m)
	// Persist even when fn reports an error, so a consumed/expired token's
	// deletion is durable (single-use guarantee).
	if writeErr := s.write(m); writeErr != nil {
		return errors.Join(fnErr, writeErr)
	}
	return fnErr
}

func (s *FileStore) read() (map[string]record, error) {
	m := map[string]record{}
	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) || len(data) == 0 {
		return m, nil
	}
	if err != nil {
		return nil, fmt.Errorf("token: read: %w", err)
	}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("token: decode: %w", err)
	}
	return m, nil
}

func (s *FileStore) write(m map[string]record) error {
	data, err := json.Marshal(m)
	if err != nil {
		return fmt.Errorf("token: encode: %w", err)
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("token: write tmp: %w", err)
	}
	if err := os.Rename(tmp, s.path); err != nil {
		return fmt.Errorf("token: rename: %w", err)
	}
	return nil
}

func randToken() (string, error) {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("token: rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
