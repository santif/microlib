package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/santif/microlib/data"
	"github.com/spf13/cobra"
)

var (
	seedsDir string
)

func init() {
	// Seed command
	seedCmd := &cobra.Command{
		Use:   "seed",
		Short: "Manage database seeds",
		Long:  `Commands for creating, applying, and resetting database seeds.`,
	}

	// Create seed command
	createSeedCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new seed",
		Long:  `Create a new seed file with SQL data.`,
		Args:  cobra.ExactArgs(1),
		Run:   createSeed,
	}

	// Apply seeds command
	applySeedCmd := &cobra.Command{
		Use:   "apply",
		Short: "Apply pending seeds",
		Long:  `Apply all pending seeds to the database.`,
		Run:   applySeeds,
	}

	// Reset seeds command
	resetSeedCmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset seed records",
		Long:  `Reset seed records to allow reapplying seeds.`,
		Run:   resetSeeds,
	}

	// Add flags
	seedCmd.PersistentFlags().StringVar(&seedsDir, "dir", "seeds", "Directory containing seed files")
	seedCmd.PersistentFlags().StringVar(&dbConfig, "config", "", "Database configuration file")

	// Add commands
	seedCmd.AddCommand(createSeedCmd)
	seedCmd.AddCommand(applySeedCmd)
	seedCmd.AddCommand(resetSeedCmd)

	rootCmd.AddCommand(seedCmd)
}

// createSeed creates a new seed file
func createSeed(cmd *cobra.Command, args []string) {
	name := args[0]
	// Sanitize name for filename
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	// Create seeds directory if it doesn't exist
	if err := os.MkdirAll(seedsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating seeds directory: %v\n", err)
		os.Exit(1)
	}

	// Find the next priority number
	priority := 1
	entries, err := os.ReadDir(seedsDir)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasPrefix(entry.Name(), "S") {
				parts := strings.SplitN(entry.Name(), "_", 2)
				if len(parts) == 2 {
					priorityStr := strings.TrimPrefix(parts[0], "S")
					if p, err := fmt.Sscanf(priorityStr, "%d", &priority); err == nil && p > 0 {
						priority = p + 1
					}
				}
			}
		}
	}

	// Format priority with leading zeros
	priorityStr := fmt.Sprintf("%02d", priority)
	filename := fmt.Sprintf("S%s_%s.sql", priorityStr, name)
	filepath := filepath.Join(seedsDir, filename)

	// Create seed file
	content := `-- Seed: ` + name + `
-- Created at: ` + time.Now().Format(time.RFC3339) + `

-- Write your seed SQL here
-- Example:
-- INSERT INTO users (name, email) VALUES ('Admin', 'admin@example.com');
`

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating seed file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created seed file: %s\n", filepath)
}

// applySeeds applies pending seeds
func applySeeds(cmd *cobra.Command, args []string) {
	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create seeder
	seeder := data.NewPostgresSeeder(db)

	// Load seeds
	if err := seeder.LoadSeeds(seedsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading seeds: %v\n", err)
		os.Exit(1)
	}

	// Apply seeds
	ctx := context.Background()
	if err := seeder.Apply(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying seeds: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Seeds applied successfully")
}

// resetSeeds resets seed records
func resetSeeds(cmd *cobra.Command, args []string) {
	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create seeder
	seeder := data.NewPostgresSeeder(db)

	// Reset seeds
	ctx := context.Background()
	if err := seeder.Reset(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error resetting seeds: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Seed records reset successfully")
}
