// Package process owns the direct argv boundary. No command is run through a shell.
package process

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"syscall"
	"time"
)

func Command(ctx context.Context, argv []string, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, argv[0], append(append([]string{}, argv[1:]...), args...)...)
	c.Cancel = func() error { return c.Process.Signal(os.Interrupt) }
	c.WaitDelay = 2 * time.Second
	return c
}
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var e *exec.ExitError
	if errors.As(err, &e) {
		return e.ExitCode()
	}
	return -1
}

// Background gives noninteractive subprocesses a signal group, including hooks.
func Background(ctx context.Context, argv []string, args ...string) *exec.Cmd {
	c := Command(ctx, argv, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGINT) }
	return c
}
