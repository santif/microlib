package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "microlib",
	Short: "MicroLib CLI - Tools for MicroLib framework",
	Long: `MicroLib CLI provides tools for working with the MicroLib framework,
including database migrations, service scaffolding, and more.

The CLI supports plugins and can be extended with custom commands.
Use 'microlib plugin discover' to find available plugins.`,
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize CLI configuration
		return initConfig()
	},
}

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
