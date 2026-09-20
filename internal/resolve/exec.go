package resolve

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Command describes one external command invocation.
type Command struct {
	// Args is the command and its arguments; Args[0] is the executable.
	Args []string
	// Dir is the working directory.
	Dir string
	// Env is appended to the process environment.
	Env []string
	// OnStderr, when non-nil, is called with each line of stderr as the
	// command produces it rather than only after it exits. Stderr is
	// captured in full either way and attached to ExecError on failure.
	OnStderr func(string)
}

// Executor runs external commands and returns their stdout. The interface
// allows the Resolver to be tested with canned responses, avoiding the need
// for a real Go toolchain or network access during unit tests.
type Executor interface {
	Run(ctx context.Context, cmd Command) (string, error)
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

func (OSExecutor) Run(ctx context.Context, c Command) (string, error) {
	if len(c.Args) == 0 {
		return "", fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, c.Args[0], c.Args[1:]...)
	cmd.Dir = c.Dir
	cmd.Env = append(os.Environ(), c.Env...)

	// exec.CommandContext defaults to SIGKILL on cancellation, giving go no
	// chance to remove its .partial cache markers or let a spawned git exit
	// cleanly. Send SIGINT first, falling back to the default kill only if
	// the process hasn't exited within WaitDelay.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.WaitDelay = 2 * time.Second

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout

	if c.OnStderr == nil {
		cmd.Stderr = &stderr
		if err := cmd.Run(); err != nil {
			return "", &ExecError{Err: err, Stderr: stderr.String()}
		}
		return stdout.String(), nil
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return "", fmt.Errorf("failed to open stderr pipe: %w", err)
	}

	// stderr is still captured in full for ExecError, independent of what
	// OnStderr does with each line — TeeReader duplicates every byte the
	// scanner reads into this buffer as it goes.
	tee := io.TeeReader(stderrPipe, &stderr)

	if err := cmd.Start(); err != nil {
		return "", &ExecError{Err: err, Stderr: stderr.String()}
	}

	// The pipe must be fully drained before Wait is called — Wait closes
	// the pipe once it sees the command exit, and if the child is still
	// blocked writing to a full pipe buffer while we're blocked in Wait
	// waiting for it to exit, neither side ever proceeds.
	scanner := bufio.NewScanner(tee)
	for scanner.Scan() {
		c.OnStderr(scanner.Text())
	}
	scanErr := scanner.Err()

	if err := cmd.Wait(); err != nil {
		return "", &ExecError{Err: err, Stderr: stderr.String()}
	}

	if scanErr != nil {
		return "", fmt.Errorf("failed to read stderr: %w", scanErr)
	}

	return stdout.String(), nil
}
