package resolve

import (
	"context"
	"fmt"
	"os/exec"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestOSExecutorRunSendsInterruptOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	resultCh := make(chan error, 1)
	go func() {
		_, err := OSExecutor{}.Run(ctx, Command{Args: []string{"sleep", "30"}, Dir: "", Env: nil})
		resultCh <- err
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	var runErr error
	select {
	case runErr = <-resultCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return after context cancellation")
	}

	assert.Less(t, time.Since(start), 2*time.Second,
		"process should have exited promptly on SIGINT, not waited out the WaitDelay kill fallback")

	var execErr *ExecError
	require.ErrorAs(t, runErr, &execErr)
	var exitErr *exec.ExitError
	require.ErrorAs(t, execErr.Err, &exitErr)
	ws, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
	require.True(t, ok, "expected a syscall.WaitStatus")
	require.True(t, ws.Signaled(), "expected the process to have been terminated by a signal")
	assert.Equal(t, syscall.SIGINT, ws.Signal(), "cmd.Cancel should send SIGINT, not the default SIGKILL")
}

func TestOSExecutorRunKillsAfterWaitDelayIfInterruptIgnored(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	script := "trap '' INT; sleep 30"

	start := time.Now()
	done := make(chan struct{})
	go func() {
		_, _ = OSExecutor{}.Run(ctx, Command{Args: []string{"sh", "-c", script}, Dir: "", Env: nil})
		close(done)
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Run did not return even after the WaitDelay kill fallback")
	}

	elapsed := time.Since(start)
	assert.GreaterOrEqual(t, elapsed, 2*time.Second, "should wait out WaitDelay before killing")
	assert.Less(t, elapsed, 4*time.Second, "should not wait much beyond WaitDelay")
}

func TestOSExecutorStreamCallsOnStderrPerLine(t *testing.T) {
	var mu sync.Mutex
	var lines []string
	onStderr := func(line string) {
		mu.Lock()
		defer mu.Unlock()
		lines = append(lines, line)
	}

	script := "echo one >&2; echo two >&2; echo three >&2; echo stdout-only"
	stdout, err := OSExecutor{}.Run(context.Background(), Command{Args: []string{"sh", "-c", script}, OnStderr: onStderr})
	require.NoError(t, err)

	assert.Equal(t, []string{"one", "two", "three"}, lines)
	assert.Equal(t, "stdout-only\n", stdout, "Stream must still capture stdout separately from the streamed stderr")
}

func TestOSExecutorStreamCapturesFullStderrOnFailure(t *testing.T) {
	script := "echo one >&2; echo two >&2; exit 1"
	_, err := OSExecutor{}.Run(context.Background(), Command{Args: []string{"sh", "-c", script}, OnStderr: func(string) {}})
	require.Error(t, err)

	var execErr *ExecError
	require.ErrorAs(t, err, &execErr)
	assert.Equal(t, "one\ntwo\n", execErr.Stderr,
		"ExecError.Stderr must contain everything, independent of what onStderr did with it")
}

func TestOSExecutorStreamDrainsLargeOutputWithoutDeadlock(t *testing.T) {
	const lineCount = 20000 // ~20000 * ~9 bytes ≈ 180KiB, well past any pipe buffer

	// A shell loop rather than 20000 literal echo lines: the unrolled form
	// makes the script itself ~180KiB of argv, which trips ARG_MAX on some
	// systems (the sandbox this was found on among them) and fails the test
	// with "argument list too long" before the pipe is ever exercised.
	script := fmt.Sprintf("i=0; while [ $i -lt %d ]; do echo line-$i >&2; i=$((i+1)); done", lineCount)

	var mu sync.Mutex
	count := 0
	onStderr := func(string) {
		mu.Lock()
		count++
		mu.Unlock()
	}

	done := make(chan error, 1)
	go func() {
		_, err := OSExecutor{}.Run(context.Background(), Command{Args: []string{"sh", "-c", script}, OnStderr: onStderr})
		done <- err
	}()

	select {
	case err := <-done:
		require.NoError(t, err)
		assert.Equal(t, lineCount, count)
	case <-time.After(10 * time.Second):
		t.Fatal("Stream did not return — likely deadlocked on an undrained stderr pipe")
	}
}

func TestOSExecutorStreamSendsInterruptOnCancel(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	start := time.Now()
	resultCh := make(chan error, 1)
	go func() {
		_, err := OSExecutor{}.Run(ctx, Command{Args: []string{"sleep", "30"}, OnStderr: func(string) {}})
		resultCh <- err
	}()

	time.Sleep(200 * time.Millisecond)
	cancel()

	var streamErr error
	select {
	case streamErr = <-resultCh:
	case <-time.After(5 * time.Second):
		t.Fatal("Stream did not return after context cancellation")
	}

	assert.Less(t, time.Since(start), 2*time.Second,
		"process should have exited promptly on SIGINT, not waited out the WaitDelay kill fallback")

	var execErr *ExecError
	require.ErrorAs(t, streamErr, &execErr)
	var exitErr *exec.ExitError
	require.ErrorAs(t, execErr.Err, &exitErr)
	ws, ok := exitErr.ProcessState.Sys().(syscall.WaitStatus)
	require.True(t, ok, "expected a syscall.WaitStatus")
	require.True(t, ws.Signaled(), "expected the process to have been terminated by a signal")
	assert.Equal(t, syscall.SIGINT, ws.Signal(), "cmd.Cancel should send SIGINT, not the default SIGKILL")
}
