package cmd

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"

	"github.com/openshift-hyperfleet/maestro-cli/internal/maestro"
	"github.com/openshift-hyperfleet/maestro-cli/internal/tui"
)

// NewTUICommand creates the `tui` subcommand for maestro-cli.
func NewTUICommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tui",
		Short: "Launch interactive terminal UI",
		Long: `Launch an interactive terminal UI to browse Maestro consumers and
ManifestWorks, with live watch mode, filtering, create, and delete actions.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			config := maestro.ClientConfig{
				HTTPEndpoint:        getPersistentStringFlag(cmd, "http-endpoint"),
				GRPCEndpoint:        getPersistentStringFlag(cmd, "grpc-endpoint"),
				GRPCInsecure:        getPersistentBoolFlag(cmd, "grpc-insecure"),
				GRPCServerCAFile:    getPersistentStringFlag(cmd, "grpc-server-ca-file"),
				GRPCBrokerCAFile:    getPersistentStringFlag(cmd, "grpc-broker-ca-file"),
				GRPCClientCertFile:  getPersistentStringFlag(cmd, "grpc-client-cert-file"),
				GRPCClientKeyFile:   getPersistentStringFlag(cmd, "grpc-client-key-file"),
				GRPCClientToken:     getPersistentStringFlag(cmd, "grpc-client-token"),
				GRPCClientTokenFile: getPersistentStringFlag(cmd, "grpc-client-token-file"),
				SourceID:            getPersistentStringFlag(cmd, "source-id"),
			}

			m := tui.New(config)
			p := tea.NewProgram(m, tea.WithAltScreen(), tea.WithMouseCellMotion())
			_, err := p.Run()
			return err
		},
	}
	return cmd
}

// getPersistentStringFlag reads a string flag from the command or its parents.
func getPersistentStringFlag(cmd *cobra.Command, name string) string {
	val, _ := cmd.Flags().GetString(name)
	if val != "" {
		return val
	}
	// Try persistent flags from parent
	if cmd.HasParent() {
		val, _ = cmd.Root().PersistentFlags().GetString(name)
	}
	return val
}

// getPersistentBoolFlag reads a bool flag from the command or its parents.
func getPersistentBoolFlag(cmd *cobra.Command, name string) bool {
	// Try root persistent flags
	val, _ := cmd.Root().PersistentFlags().GetBool(name)
	return val
}
