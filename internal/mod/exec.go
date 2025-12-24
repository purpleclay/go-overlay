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

	var stdout bytes.Buffer
	r, err := interp.New(
		interp.Params("-e"),
		interp.StdIO(os.Stdin, &stdout, os.Stderr),
		interp.OpenHandler(openHandler),
		interp.Env(expand.ListEnviron(env...)),
		interp.Dir(dir),
	)
	if err != nil {
		return "", err
	}

	if err := r.Run(context.Background(), p); err != nil {
		return "", err
	}

	return stdout.String(), nil
}

func openHandler(ctx context.Context, path string, flag int, perm os.FileMode) (io.ReadWriteCloser, error) {
	if path == "/dev/null" {
		return devNull{}, nil
	}

	return interp.DefaultOpenHandler()(ctx, path, flag, perm)
}
