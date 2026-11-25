package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/vparat/pubcrawl/internal/pubcrawl"
)

// Version is the source revision supplied by the build.
var Version = "dev"


// main exits with the CLI status.
func main() {
	os.Exit(run())
}


// run executes the CLI and returns its process exit status.
func run() int {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	restoreSignals := context.AfterFunc(ctx, stop)
	defer restoreSignals()

	if err := pubcrawl.NewCommand(Version).ExecuteContext(ctx); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)

		return 1
	}

	return 0
}
