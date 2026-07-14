package execute_test

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/inovacc/remote-exec/internal/execute"
)

// TestHelperProcess is not a real test: when EXECUTE_HELPER=1 the test binary
// re-execs itself as the command under test, giving a portable subprocess.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("EXECUTE_HELPER") != "1" {
		return
	}
	fmt.Fprint(os.Stdout, "hello-stdout")
	fmt.Fprint(os.Stderr, "hello-stderr")
	code, _ := strconv.Atoi(os.Getenv("EXECUTE_EXIT"))
	os.Exit(code)
}

func helperSpec(exit int) execute.Spec {
	return execute.Spec{
		Command: os.Args[0],
		Args:    []string{"-test.run=TestHelperProcess"},
		Env: map[string]string{
			"EXECUTE_HELPER": "1",
			"EXECUTE_EXIT":   strconv.Itoa(exit),
		},
	}
}

func collect(t *testing.T, spec execute.Spec) (string, string, int) {
	t.Helper()
	var mu sync.Mutex
	var out, errBuf bytes.Buffer
	code, err := execute.Run(context.Background(), spec, func(c execute.Chunk) error {
		mu.Lock()
		defer mu.Unlock()
		out.Write(c.Stdout)
		errBuf.Write(c.Stderr)
		return nil
	})
	require.NoError(t, err)
	return out.String(), errBuf.String(), code
}

func TestRun_StreamsStdoutStderrAndZeroExit(t *testing.T) {
	out, errOut, code := collect(t, helperSpec(0))
	require.Equal(t, 0, code)
	require.Contains(t, out, "hello-stdout")
	require.Contains(t, errOut, "hello-stderr")
}

func TestRun_PropagatesNonZeroExit(t *testing.T) {
	_, _, code := collect(t, helperSpec(3))
	require.Equal(t, 3, code)
}

func TestRun_EmptyCommandErrors(t *testing.T) {
	_, err := execute.Run(context.Background(), execute.Spec{}, func(execute.Chunk) error { return nil })
	require.Error(t, err)
}

func TestRun_MissingBinaryErrors(t *testing.T) {
	_, err := execute.Run(context.Background(), execute.Spec{Command: "definitely-not-a-real-binary-xyz"}, func(execute.Chunk) error { return nil })
	require.Error(t, err)
}
