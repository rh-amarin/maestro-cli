package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/api/errors"

	"github.com/openshift-hyperfleet/maestro-cli/internal/maestro"
	"github.com/openshift-hyperfleet/maestro-cli/internal/manifestwork"
	"github.com/openshift-hyperfleet/maestro-cli/pkg/logger"
)

const (
	statusWaiting = "Waiting"
)

// WaitFlags contains flags for the wait command
type WaitFlags struct {
	Name     string
	Consumer string
	For      string // Condition to wait for (like kubectl --for)
	// Global flags
	GRPCEndpoint        string
	HTTPEndpoint        string
	GRPCInsecure        bool
	GRPCServerCAFile    string
	GRPCClientCertFile  string
	GRPCClientKeyFile   string
	GRPCBrokerCAFile    string
	GRPCClientToken     string
	GRPCClientTokenFile string
	ResultsPath         string
	Output              string
	Timeout             time.Duration
	Verbose             bool
}

// NewWaitCommand creates the wait command
func NewWaitCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait",
		Short: "Wait for a ManifestWork to reach a specific condition",
		Long: `Wait for a ManifestWork to reach a specific condition with optional timeout.

Examples:
  # Wait for Available condition (default, like kubectl wait --for=condition=Available)
  maestro-cli wait --name=hyperfleet-cluster-west-1-job --consumer=agent1

  # Wait for Job completion (like kubectl wait --for=condition=Complete)
  maestro-cli wait --name=hyperfleet-cluster-west-1-job --consumer=agent1 --for="Job:Complete"

  # Wait with timeout (default 5m if not specified)
  maestro-cli wait --name=hyperfleet-cluster-west-1-job --consumer=agent1 \
    --for="Job:Complete OR Job:Failed" --timeout=10m

  # Wait and write results for status-reporter
  maestro-cli wait --name=hyperfleet-cluster-west-1-job --consumer=agent1 \
    --for=Available --results-path=/tmp/wait-results.json`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			flags := &WaitFlags{
				Name:     getStringFlag(cmd, "name"),
				Consumer: getStringFlag(cmd, "consumer"),
				For:      getStringFlag(cmd, "for"),
				// Global flags
				GRPCEndpoint:        getStringFlag(cmd, "grpc-endpoint"),
				HTTPEndpoint:        getStringFlag(cmd, "http-endpoint"),
				GRPCInsecure:        getBoolFlag(cmd, "grpc-insecure"),
				GRPCServerCAFile:    getStringFlag(cmd, "grpc-server-ca-file"),
				GRPCClientCertFile:  getStringFlag(cmd, "grpc-client-cert-file"),
				GRPCClientKeyFile:   getStringFlag(cmd, "grpc-client-key-file"),
				GRPCBrokerCAFile:    getStringFlag(cmd, "grpc-broker-ca-file"),
				GRPCClientToken:     getStringFlag(cmd, "grpc-client-token"),
				GRPCClientTokenFile: getStringFlag(cmd, "grpc-client-token-file"),
				ResultsPath:         getStringFlag(cmd, "results-path"),
				Output:              getStringFlag(cmd, "output"),
				Timeout:             getDurationFlag(cmd, "timeout"),
				Verbose:             getBoolFlag(cmd, "verbose"),
			}

			return runWaitCommand(cmd.Context(), flags)
		},
	}

	// Command-specific flags
	cmd.Flags().String("name", "", "ManifestWork name (required)")
	cmd.Flags().String("consumer", "", "Target cluster name (required)")
	cmd.Flags().String(
		"for",
		"Available",
		"Condition to wait for (e.g., 'Available', 'Job:Complete', 'Job:Complete OR Job:Failed')",
	)

	// Mark required flags
	if err := cmd.MarkFlagRequired("name"); err != nil {
		panic(err)
	}
	if err := cmd.MarkFlagRequired("consumer"); err != nil {
		panic(err)
	}

	return cmd
}

// runWaitCommand executes the wait command
func runWaitCommand(ctx context.Context, flags *WaitFlags) error {
	// Initialize logger
	log := logger.New(logger.Config{
		Level:     getLogLevel(flags.Verbose),
		Format:    "text",
		Component: "maestro-cli",
		Version:   "dev",
	})

	// Create HTTP-only client (no gRPC needed for wait)
	client, err := maestro.NewHTTPClient(maestro.ClientConfig{
		HTTPEndpoint: flags.HTTPEndpoint,
		GRPCInsecure: flags.GRPCInsecure,
	})
	if err != nil {
		return fmt.Errorf("failed to create Maestro client: %w", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Warn(ctx, "Failed to close client", logger.Fields{"error": err.Error()})
		}
	}()

	// Validate consumer exists
	if err := client.ValidateConsumer(ctx, flags.Consumer); err != nil {
		return err
	}

	// Check if ManifestWork exists
	_, err = client.GetManifestWorkByNameHTTP(ctx, flags.Consumer, flags.Name)
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("ManifestWork %q not found in consumer %q", flags.Name, flags.Consumer)
		}
		return fmt.Errorf("failed to check ManifestWork existence: %w", err)
	}

	// Use timeout if specified, otherwise default to 5 minutes
	timeout := flags.Timeout
	if timeout == 0 {
		timeout = DefaultWaitTimeout
	}

	log.Info(ctx, "Waiting for condition", logger.Fields{
		"name":     flags.Name,
		"consumer": flags.Consumer,
		"for":      flags.For,
		"timeout":  timeout.String(),
	})

	// Create wait context with timeout
	waitCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Create callback to update results file on each poll
	var callback maestro.WaitCallback
	if flags.ResultsPath != "" || os.Getenv("RESULTS_PATH") != "" {
		callback = func(details *maestro.ManifestWorkDetails, conditionMet bool) error {
			status := statusWaiting
			message := fmt.Sprintf("Waiting for condition '%s'", flags.For)
			if conditionMet {
				status = flags.For
				message = fmt.Sprintf("Condition '%s' met", flags.For)
			}
			result := manifestwork.BuildStatusResult(flags.Name, flags.Consumer, status, message, details)
			return manifestwork.WriteResult(flags.ResultsPath, result)
		}
	}

	// Wait for condition (poll every 1 second by default)
	if err := client.WaitForCondition(
		waitCtx,
		flags.Consumer,
		flags.Name,
		flags.For,
		maestro.DefaultPollInterval,
		log,
		callback,
	); err != nil {
		return fmt.Errorf("error waiting for condition '%s': %w", flags.For, err)
	}

	log.Info(ctx, "Condition met", logger.Fields{
		"name":     flags.Name,
		"consumer": flags.Consumer,
		"for":      flags.For,
	})

	return nil
}
