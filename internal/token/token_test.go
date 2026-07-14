package token

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func newTestStore(t *testing.T) *FileStore {
	t.Helper()
	return NewFileStore(filepath.Join(t.TempDir(), "tokens.json"))
}

func TestIssueThenConsume_ReturnsRole(t *testing.T) {
	s := newTestStore(t)

	value, err := s.Issue("rex:operator", time.Hour)
	require.NoError(t, err)
	require.NotEmpty(t, value)

	role, err := s.Consume(value)
	require.NoError(t, err)
	require.Equal(t, "rex:operator", role)
}

func TestConsume_SingleUse(t *testing.T) {
	s := newTestStore(t)
	value, err := s.Issue("rex:reader", time.Hour)
	require.NoError(t, err)

	_, err = s.Consume(value)
	require.NoError(t, err)

	_, err = s.Consume(value)
	require.ErrorIs(t, err, ErrUnknownToken, "a token must not be reusable")
}

func TestConsume_Expired(t *testing.T) {
	s := newTestStore(t)
	base := time.Now()
	s.now = func() time.Time { return base }

	value, err := s.Issue("rex:admin", time.Minute)
	require.NoError(t, err)

	s.now = func() time.Time { return base.Add(2 * time.Minute) }
	_, err = s.Consume(value)
	require.ErrorIs(t, err, ErrExpired)

	// Expired token is also removed, not left dangling.
	_, err = s.Consume(value)
	require.ErrorIs(t, err, ErrUnknownToken)
}

func TestConsume_Unknown(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Consume("nope")
	require.ErrorIs(t, err, ErrUnknownToken)
}
