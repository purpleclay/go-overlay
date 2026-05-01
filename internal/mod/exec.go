package mod

import (
	"bytes"
	"os"
	"os/exec"
	"strings"
)

type execError struct {
	err    error
	stderr string
}

func (e *execError) Error() string {
	if e.stderr != "" {
		return strings.TrimSpace(e.stderr)
	}
	return e.err.Error()
}

func (e *execError) Unwrap() error {
	return e.err
}

func execCmd(args []string, dir string) (string, error) {
	return execWithEnv(args, dir, nil)
}

func execWithEnv(args []string, dir string, extraEnv []string) (string, error) {
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), extraEnv...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", &execError{err: err, stderr: stderr.String()}
	}
	return stdout.String(), nil
}
