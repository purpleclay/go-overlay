package resolve

import (
	"context"
	"os/exec"
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
		_, err := OSExecutor{}.Run(ctx, []string{"sleep", "30"}, "", nil)
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
		_, _ = OSExecutor{}.Run(ctx, []string{"sh", "-c", script}, "", nil)
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
