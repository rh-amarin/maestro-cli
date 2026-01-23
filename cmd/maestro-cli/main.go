// Package main provides the entry point for the maestro-cli application.
package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/openshift-hyperfleet/maestro-cli/cmd"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

	rootCmd := cmd.NewRootCommand()
	if err := rootCmd.ExecuteContext(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		cancel() // Clean up signal context
		os.Exit(1)
	}

	cancel() // Clean up signal context
}
