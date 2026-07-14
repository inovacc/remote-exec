// Package execute runs a command and streams its stdout/stderr live through an
// emit callback, returning the process exit code. It is the execution primitive
// behind the Agent Exec/Deploy RPCs; authorization is enforced upstream.
package execute

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sync"
)

// Spec describes a command to run.
type Spec struct {
	Command    string
	Args       []string
	WorkingDir string
	Env        map[string]string
}

// Chunk is one streamed unit of output. Exactly one of Stdout/Stderr is set.
type Chunk struct {
	Stdout []byte
	Stderr []byte
}

// Run executes spec, calling emit for each output chunk, and returns the exit
// code. emit is serialized, so it is safe to send on a gRPC stream from it. A
// non-zero exit code is returned with a nil error; only failures to start or
// stream produce an error.
func Run(ctx context.Context, spec Spec, emit func(Chunk) error) (int, error) {
	if spec.Command == "" {
		return -1, errors.New("execute: empty command")
	}
	cmd := exec.CommandContext(ctx, spec.Command, spec.Args...)
	cmd.Dir = spec.WorkingDir
	cmd.Env = mergeEnv(spec.Env)

	var mu sync.Mutex
	safeEmit := func(c Chunk) error {
		mu.Lock()
		defer mu.Unlock()
		return emit(c)
	}
	cmd.Stdout = &chunkWriter{emit: safeEmit}
	cmd.Stderr = &chunkWriter{emit: safeEmit, stderr: true}

	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), nil
	}
	return -1, fmt.Errorf("execute: run %q: %w", spec.Command, err)
}

func mergeEnv(extra map[string]string) []string {
	env := os.Environ()
	for k, v := range extra {
		env = append(env, k+"="+v)
	}
	return env
}

type chunkWriter struct {
	emit   func(Chunk) error
	stderr bool
}

func (w *chunkWriter) Write(p []byte) (int, error) {
	buf := make([]byte, len(p))
	copy(buf, p)
	c := Chunk{Stdout: buf}
	if w.stderr {
		c = Chunk{Stderr: buf}
	}
	if err := w.emit(c); err != nil {
		return 0, err
	}
	return len(p), nil
}
