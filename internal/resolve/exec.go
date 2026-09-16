package resolve

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Executor runs external commands and returns their stdout. The interface
// allows the Resolver to be tested with canned responses, avoiding the need
// for a real Go toolchain or network access during unit tests.
type Executor interface {
	Run(ctx context.Context, args []string, dir string, env []string) (string, error)
}

// ExecError wraps a command failure with its stderr output for diagnostics.
type ExecError struct {
	Err    error
	Stderr string
}

func (e *ExecError) Error() string {
	if stderr := strings.TrimSpace(e.Stderr); stderr != "" {
		return stderr
	}
	return e.Err.Error()
}

func (e *ExecError) Unwrap() error {
	return e.Err
}

// OSExecutor runs commands using os/exec.
type OSExecutor struct{}

func (OSExecutor) Run(ctx context.Context, args []string, dir string, env []string) (string, error) {
	if len(args) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)

	// exec.CommandContext defaults to SIGKILL on cancellation, giving go no
	// chance to remove its .partial cache markers or let a spawned git exit
	// cleanly. Send SIGINT first, falling back to the default kill only if
	// the process hasn't exited within WaitDelay.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 2 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return "", &ExecError{Err: err, Stderr: stderr.String()}
	}

	return stdout.String(), nil
}
