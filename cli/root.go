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
including database migrations, service scaffolding, and more.`,
}

// Execute executes the root command
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
