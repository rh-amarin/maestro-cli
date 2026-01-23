package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

// Version information - these should be set at build time using ldflags
//
// Build with version info:
//
//	make build                    # Uses Makefile defaults
//	make build GIT_COMMIT=abc123  # Override commit
//	make build VERSION=1.2.3      # Override version
//
// Manual build with ldflags:
//
//	go build -ldflags="-X github.com/openshift-hyperfleet/maestro-cli/cmd.Version=1.2.3 \
//	                   -X github.com/openshift-hyperfleet/maestro-cli/cmd.Commit=abc123 \
//	                   -X github.com/openshift-hyperfleet/maestro-cli/cmd.Date=2024-01-01T10:00:00Z" \
//	  ./cmd/maestro-cli
//
// The ldflags inject these values at compile time, replacing the default values below.
var (
	Version   = "dev"             // Set via -X github.com/openshift-hyperfleet/maestro-cli/cmd.Version
	Commit    = "unknown"         // Set via -X github.com/openshift-hyperfleet/maestro-cli/cmd.Commit
	Date      = "unknown"         // Set via -X github.com/openshift-hyperfleet/maestro-cli/cmd.Date
	Tag       = "none"            // Set via -X github.com/openshift-hyperfleet/maestro-cli/cmd.Tag
	GoVersion = runtime.Version() // Runtime Go version (not set via ldflags)
)

// NewVersionCommand creates the version command
func NewVersionCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Show version information",
		Long:  "Display version, build information, and runtime details for maestro-cli.",
		Run: func(_ *cobra.Command, _ []string) {
			fmt.Printf("maestro-cli version %s\n", Version)
			if Tag != "none" && Tag != "" {
				fmt.Printf("Git tag: %s\n", Tag)
			}
			fmt.Printf("Git commit: %s\n", Commit)
			fmt.Printf("Built: %s\n", Date)
			fmt.Printf("Go version: %s\n", GoVersion)
			fmt.Printf("OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		},
	}

	return cmd
}
