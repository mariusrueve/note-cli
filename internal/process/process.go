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

type backgroundCommand struct {
	*exec.Cmd
	ctx context.Context
}

// finish removes descendants that ignored the group's graceful interrupt. The
// command's WaitDelay has already allowed cleanup before a hard cancellation.
func (c *backgroundCommand) finish() {
	if c.ctx.Err() != nil && c.Process != nil {
		_ = syscall.Kill(-c.Process.Pid, syscall.SIGKILL)
	}
}
func (c *backgroundCommand) Run() error {
	defer c.finish()
	return c.Cmd.Run()
}
func (c *backgroundCommand) Output() ([]byte, error) {
	defer c.finish()
	return c.Cmd.Output()
}
func (c *backgroundCommand) CombinedOutput() ([]byte, error) {
	defer c.finish()
	return c.Cmd.CombinedOutput()
}

// Background gives noninteractive subprocesses a signal group, including hooks.
// Cancellation first sends SIGINT; after the bounded wait, any surviving group
// members are killed so they cannot keep output pipes or repository locks alive.
func Background(ctx context.Context, argv []string, args ...string) *backgroundCommand {
	c := Command(ctx, argv, args...)
	c.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	c.Cancel = func() error { return syscall.Kill(-c.Process.Pid, syscall.SIGINT) }
	return &backgroundCommand{Cmd: c, ctx: ctx}
}
