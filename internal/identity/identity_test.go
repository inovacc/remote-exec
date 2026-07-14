package identity

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAgentID_GeneratesThenPersists(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "agent.id")

	first, err := AgentID(path)
	require.NoError(t, err)
	require.NotEmpty(t, first)
	require.FileExists(t, path)

	second, err := AgentID(path)
	require.NoError(t, err)
	require.Equal(t, first, second, "id must be stable across calls")
}

func TestAgentID_EmptyFileIsError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "agent.id")
	require.NoError(t, os.WriteFile(path, []byte("  \n"), 0o600))

	_, err := AgentID(path)
	require.Error(t, err)
}
