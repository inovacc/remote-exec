// Package identity derives a stable per-agent identifier. The ID is generated
// once and persisted so it survives restarts, giving controllers a durable
// handle to "the other instance".
package identity

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
)

// AgentID returns the agent's stable ID, reading it from path or generating and
// persisting a new UUID when the file is absent.
func AgentID(path string) (string, error) {
	data, err := os.ReadFile(path)
	switch {
	case err == nil:
		id := strings.TrimSpace(string(data))
		if id == "" {
			return "", fmt.Errorf("identity: empty id file %q", path)
		}
		return id, nil
	case errors.Is(err, os.ErrNotExist):
		return generate(path)
	default:
		return "", fmt.Errorf("identity: read id: %w", err)
	}
}

func generate(path string) (string, error) {
	id := uuid.NewString()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", fmt.Errorf("identity: mkdir: %w", err)
	}
	if err := os.WriteFile(path, []byte(id+"\n"), 0o600); err != nil {
		return "", fmt.Errorf("identity: write id: %w", err)
	}
	return id, nil
}
