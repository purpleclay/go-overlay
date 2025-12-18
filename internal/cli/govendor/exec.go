package govendor

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

type DevNull struct{}

func (DevNull) Read(_ []byte) (int, error) {
	return 0, io.EOF
}

func (DevNull) Write(p []byte) (int, error) {
	return len(p), nil
}

func (DevNull) Close() error {
	return nil
}

func exec(args []string) (string, error) {
	cmd := strings.Join(args, " ")
	p, err := syntax.NewParser().Parse(strings.NewReader(cmd), "")
	if err != nil {
		return "", err
	}

	var stdout bytes.Buffer
	r, err := interp.New(
		interp.Params("-e"),
		interp.StdIO(os.Stdin, &stdout, os.Stderr),
		interp.OpenHandler(openHandler),
		interp.Env(expand.ListEnviron(os.Environ()...)),
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
		return DevNull{}, nil
	}

	return interp.DefaultOpenHandler()(ctx, path, flag, perm)
}
