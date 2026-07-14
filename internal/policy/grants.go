package policy

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Errors returned by Grants.Consume.
var (
	ErrNoGrant         = errors.New("policy: unknown approval id")
	ErrGrantExpired    = errors.New("policy: approval expired")
	ErrCommandMismatch = errors.New("policy: approval does not match command")
)

type grant struct {
	command string
	expiry  time.Time
}

// Grants tracks pending one-time approvals for destructive commands. It lives in
// the running daemon, so an in-memory store is correct: a grant is issued and
// consumed within the same process.
type Grants struct {
	mu  sync.Mutex
	m   map[string]grant
	now func() time.Time
}

// NewGrants returns an empty approval store.
func NewGrants() *Grants {
	return &Grants{m: make(map[string]grant), now: time.Now}
}

// Issue records a pending approval for command, valid for ttl, and returns its
// one-time id.
func (g *Grants) Issue(command string, ttl time.Duration) (string, error) {
	id, err := randID()
	if err != nil {
		return "", err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	g.m[id] = grant{command: command, expiry: g.now().Add(ttl)}
	return id, nil
}

// Consume validates and permanently removes the approval identified by id,
// requiring it to match command. A grant is removed even when expired so it can
// never be reused.
func (g *Grants) Consume(id, command string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	rec, ok := g.m[id]
	if !ok {
		return ErrNoGrant
	}
	delete(g.m, id)
	if g.now().After(rec.expiry) {
		return ErrGrantExpired
	}
	if rec.command != command {
		return ErrCommandMismatch
	}
	return nil
}

func randID() (string, error) {
	b := make([]byte, 18)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("policy: rand: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
