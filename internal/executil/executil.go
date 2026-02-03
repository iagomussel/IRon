package executil

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"time"
)

type Result struct {
	Stdout string
	Stderr string
	Code   int
}

func Run(ctx context.Context, name string, args []string, input []byte, env []string, timeout time.Duration, dir string) (Result, error) {
	if timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}
	cmd := exec.CommandContext(ctx, name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	if len(env) > 0 {
		cmd.Env = append(os.Environ(), env...)
	}
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}
	err := cmd.Run()
	code := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code = exitErr.ExitCode()
		} else {
			code = -1
		}
	}
	return Result{Stdout: stdout.String(), Stderr: stderr.String(), Code: code}, err
}
