package mod

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
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

type devNull struct{}

func (devNull) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (devNull) Write(p []byte) (int, error) {
	return len(p), nil
}

func (devNull) Close() error {
	return nil
}

func exec(args []string, dir string) (string, error) {
	return execWithEnv(args, dir, nil)
}

func execWithEnv(args []string, dir string, extraEnv []string) (string, error) {
	cmd := strings.Join(args, " ")
	p, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		return "", err
	}

	env := append(os.Environ(), extraEnv...)

	var stdout, stderr bytes.Buffer
	r, err := interp.New(
		interp.Params("-e"),
		interp.StdIO(os.Stdin, &stdout, &stderr),
		interp.OpenHandler(openHandler),
		interp.Env(expand.ListEnviron(env...)),
		interp.Dir(dir),
	)
	if err != nil {
		return "", err
	}

	if err := r.Run(context.Background(), p); err != nil {
		return "", &execError{err: err, stderr: stderr.String()}
	}

	return stdout.String(), nil
}

func openHandler(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	if path == "/dev/null" {
		return devNull{}, nil
	}

	return interp.DefaultOpenHandler()(ctx, path, flag, perm)
}
