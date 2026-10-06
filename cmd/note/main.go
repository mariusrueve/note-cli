package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/mariusrueve/note-cli/internal/cli"
	"github.com/mariusrueve/note-cli/internal/errs"
)

var version = "dev"
var commit = "unknown"

func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err := cli.New(cli.Options{Version: version, Commit: commit}).ExecuteContext(ctx)
	if ctx.Err() != nil {
		err = ctx.Err()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "note:", err)
	}
	return errs.Code(err)
}
func main() { os.Exit(run()) }
