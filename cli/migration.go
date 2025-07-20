package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/santif/microlib/data"
	"github.com/spf13/cobra"
)

var (
	migrationsDir string
	dbConfig      string
)

func init() {
	// Migration command
	migrationCmd := &cobra.Command{
		Use:   "migration",
		Short: "Manage database migrations",
		Long:  `Commands for creating, applying, and rolling back database migrations.`,
	}

	// Create migration command
	createCmd := &cobra.Command{
		Use:   "create [name]",
		Short: "Create a new migration",
		Long:  `Create a new migration file with up and down SQL sections.`,
		Args:  cobra.ExactArgs(1),
		Run:   createMigration,
	}

	// Apply migrations command
	applyCmd := &cobra.Command{
		Use:   "apply [version]",
		Short: "Apply pending migrations",
		Long:  `Apply all pending migrations or up to a specific version.`,
		Args:  cobra.MaximumNArgs(1),
		Run:   applyMigrations,
	}

	// Rollback migrations command
	rollbackCmd := &cobra.Command{
		Use:   "rollback [version]",
		Short: "Rollback migrations",
		Long:  `Rollback the last migration or down to a specific version.`,
		Args:  cobra.MaximumNArgs(1),
		Run:   rollbackMigrations,
	}

	// Status command
	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show migration status",
		Long:  `Show the status of all migrations.`,
		Run:   migrationStatus,
	}

	// Add flags
	migrationCmd.PersistentFlags().StringVar(&migrationsDir, "dir", "migrations", "Directory containing migration files")
	migrationCmd.PersistentFlags().StringVar(&dbConfig, "config", "", "Database configuration file")

	// Add commands
	migrationCmd.AddCommand(createCmd)
	migrationCmd.AddCommand(applyCmd)
	migrationCmd.AddCommand(rollbackCmd)
	migrationCmd.AddCommand(statusCmd)

	rootCmd.AddCommand(migrationCmd)
}

// createMigration creates a new migration file
func createMigration(cmd *cobra.Command, args []string) {
	name := args[0]
	// Sanitize name for filename
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') {
			return r
		}
		return '_'
	}, name)

	// Create migrations directory if it doesn't exist
	if err := os.MkdirAll(migrationsDir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating migrations directory: %v\n", err)
		os.Exit(1)
	}

	// Generate version based on timestamp
	version := time.Now().Unix()
	filename := fmt.Sprintf("V%d__%s.sql", version, name)
	filepath := filepath.Join(migrationsDir, filename)

	// Create migration file
	content := `-- Migration: ` + name + `
-- Created at: ` + time.Now().Format(time.RFC3339) + `

-- Write your UP migration SQL here

-- DOWN

-- Write your DOWN migration SQL here
`

	if err := os.WriteFile(filepath, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating migration file: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("Created migration file: %s\n", filepath)
}

// applyMigrations applies pending migrations
func applyMigrations(cmd *cobra.Command, args []string) {
	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create migration runner
	runner := data.NewPostgresMigrationRunner(db)

	// Load migrations
	if err := runner.LoadMigrations(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	// Parse version
	version := int64(0)
	if len(args) > 0 {
		v, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid version number: %v\n", err)
			os.Exit(1)
		}
		version = v
	}

	// Apply migrations
	ctx := context.Background()
	if err := runner.Apply(ctx, version); err != nil {
		fmt.Fprintf(os.Stderr, "Error applying migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migrations applied successfully")
}

// rollbackMigrations rolls back migrations
func rollbackMigrations(cmd *cobra.Command, args []string) {
	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create migration runner
	runner := data.NewPostgresMigrationRunner(db)

	// Load migrations
	if err := runner.LoadMigrations(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	// Parse version
	version := int64(0)
	if len(args) > 0 {
		v, err := strconv.ParseInt(args[0], 10, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Invalid version number: %v\n", err)
			os.Exit(1)
		}
		version = v
	}

	// Rollback migrations
	ctx := context.Background()
	if err := runner.Rollback(ctx, version); err != nil {
		fmt.Fprintf(os.Stderr, "Error rolling back migrations: %v\n", err)
		os.Exit(1)
	}

	fmt.Println("Migrations rolled back successfully")
}

// migrationStatus shows the status of all migrations
func migrationStatus(cmd *cobra.Command, args []string) {
	// Connect to database
	db, err := connectToDatabase()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	// Create migration runner
	runner := data.NewPostgresMigrationRunner(db)

	// Load migrations
	if err := runner.LoadMigrations(migrationsDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error loading migrations: %v\n", err)
		os.Exit(1)
	}

	// Get migration status
	ctx := context.Background()
	statuses, err := runner.Status(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error getting migration status: %v\n", err)
		os.Exit(1)
	}

	// Display status
	fmt.Println("Migration Status:")
	fmt.Println("=================")
	if len(statuses) == 0 {
		fmt.Println("No migrations have been applied")
	} else {
		fmt.Printf("%-15s | %-30s | %s\n", "Version", "Description", "Applied At")
		fmt.Println(strings.Repeat("-", 80))
		for _, status := range statuses {
			fmt.Printf("%-15d | %-30s | %s\n", status.Version, status.Description, status.AppliedAt.Format(time.RFC3339))
		}
	}
}

// Helper function to connect to the database
func connectToDatabase() (data.Database, error) {
	// In a real implementation, this would load configuration from a file
	// For now, we'll use environment variables or defaults
	cfg := data.PostgresConfig{
		Host:     getEnv("DB_HOST", "localhost"),
		Port:     getEnvInt("DB_PORT", 5432),
		Username: getEnv("DB_USER", "postgres"),
		Password: getEnv("DB_PASSWORD", "postgres"),
		Database: getEnv("DB_NAME", "postgres"),
		SSLMode:  getEnv("DB_SSLMODE", "disable"),
	}

	return data.NewPostgresDB(context.Background(), cfg)
}

// Helper function to get environment variable with default
func getEnv(key, defaultValue string) string {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	return value
}

// Helper function to get integer environment variable with default
func getEnvInt(key string, defaultValue int) int {
	value := os.Getenv(key)
	if value == "" {
		return defaultValue
	}
	intValue, err := strconv.Atoi(value)
	if err != nil {
		return defaultValue
	}
	return intValue
}
