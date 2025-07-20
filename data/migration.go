package data

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Migration represents a database migration
type Migration struct {
	Version     int64
	Description string
	UpSQL       string
	DownSQL     string
}

// MigrationStatus represents the status of a migration
type MigrationStatus struct {
	Version     int64
	Description string
	AppliedAt   time.Time
}

// MigrationRunner is responsible for running database migrations
type MigrationRunner interface {
	// Initialize creates the migrations table if it doesn't exist
	Initialize(ctx context.Context) error

	// Apply applies all pending migrations up to the specified version
	// If version is 0, all pending migrations are applied
	Apply(ctx context.Context, version int64) error

	// Rollback rolls back migrations down to the specified version
	// If version is 0, the last applied migration is rolled back
	Rollback(ctx context.Context, version int64) error

	// Status returns the status of all migrations
	Status(ctx context.Context) ([]MigrationStatus, error)

	// AddMigration adds a migration to the runner
	AddMigration(migration Migration) error

	// LoadMigrations loads migrations from the specified directory
	LoadMigrations(migrationsDir string) error

	// LoadMigrationsFS loads migrations from the specified filesystem
	LoadMigrationsFS(fsys fs.FS, root string) error
}

// PostgresMigrationRunner implements MigrationRunner for PostgreSQL
type PostgresMigrationRunner struct {
	db         Database
	migrations []Migration
}

// NewPostgresMigrationRunner creates a new PostgreSQL migration runner
func NewPostgresMigrationRunner(db Database) *PostgresMigrationRunner {
	return &PostgresMigrationRunner{
		db:         db,
		migrations: []Migration{},
	}
}

// Initialize creates the migrations table if it doesn't exist
func (r *PostgresMigrationRunner) Initialize(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_migrations (
		version BIGINT PRIMARY KEY,
		description TEXT NOT NULL,
		applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);
	`

	_, err := r.db.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create migrations table: %w", err)
	}

	return nil
}

// Apply applies all pending migrations up to the specified version
func (r *PostgresMigrationRunner) Apply(ctx context.Context, version int64) error {
	if err := r.Initialize(ctx); err != nil {
		return err
	}

	// Get applied migrations
	appliedVersions, err := r.getAppliedVersions(ctx)
	if err != nil {
		return err
	}

	// Sort migrations by version
	sort.Slice(r.migrations, func(i, j int) bool {
		return r.migrations[i].Version < r.migrations[j].Version
	})

	// Apply pending migrations
	for _, migration := range r.migrations {
		// Skip if already applied
		if _, ok := appliedVersions[migration.Version]; ok {
			continue
		}

		// Skip if version is higher than target version (unless target is 0)
		if version > 0 && migration.Version > version {
			continue
		}

		// Apply migration within a transaction
		err := r.db.Transaction(ctx, func(tx Transaction) error {
			// Execute migration
			_, err := tx.Exec(ctx, migration.UpSQL)
			if err != nil {
				return fmt.Errorf("failed to apply migration %d: %w", migration.Version, err)
			}

			// Record migration
			_, err = tx.Exec(
				ctx,
				"INSERT INTO schema_migrations (version, description) VALUES ($1, $2)",
				migration.Version,
				migration.Description,
			)
			if err != nil {
				return fmt.Errorf("failed to record migration %d: %w", migration.Version, err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		fmt.Printf("Applied migration %d: %s\n", migration.Version, migration.Description)
	}

	return nil
}

// Rollback rolls back migrations down to the specified version
func (r *PostgresMigrationRunner) Rollback(ctx context.Context, version int64) error {
	if err := r.Initialize(ctx); err != nil {
		return err
	}

	// Get applied migrations
	appliedVersions, err := r.getAppliedVersions(ctx)
	if err != nil {
		return err
	}

	// Sort migrations by version in descending order
	sort.Slice(r.migrations, func(i, j int) bool {
		return r.migrations[i].Version > r.migrations[j].Version
	})

	// If version is 0, rollback only the last migration
	if version == 0 {
		var lastVersion int64
		for v := range appliedVersions {
			if v > lastVersion {
				lastVersion = v
			}
		}
		version = lastVersion - 1
	}

	// Rollback migrations
	for _, migration := range r.migrations {
		// Skip if not applied
		if _, ok := appliedVersions[migration.Version]; !ok {
			continue
		}

		// Skip if version is less than or equal to target version
		if migration.Version <= version {
			continue
		}

		// Rollback migration within a transaction
		err := r.db.Transaction(ctx, func(tx Transaction) error {
			// Execute rollback
			_, err := tx.Exec(ctx, migration.DownSQL)
			if err != nil {
				return fmt.Errorf("failed to rollback migration %d: %w", migration.Version, err)
			}

			// Remove migration record
			_, err = tx.Exec(
				ctx,
				"DELETE FROM schema_migrations WHERE version = $1",
				migration.Version,
			)
			if err != nil {
				return fmt.Errorf("failed to remove migration record %d: %w", migration.Version, err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		fmt.Printf("Rolled back migration %d: %s\n", migration.Version, migration.Description)
	}

	return nil
}

// Status returns the status of all migrations
func (r *PostgresMigrationRunner) Status(ctx context.Context) ([]MigrationStatus, error) {
	if err := r.Initialize(ctx); err != nil {
		return nil, err
	}

	// Get applied migrations
	rows, err := r.db.Query(ctx, `
		SELECT version, description, applied_at
		FROM schema_migrations
		ORDER BY version ASC
	`)
	if err != nil {
		return nil, fmt.Errorf("failed to query migrations: %w", err)
	}
	defer rows.Close()

	var statuses []MigrationStatus
	for rows.Next() {
		var status MigrationStatus
		var appliedAt time.Time
		if err := rows.Scan(&status.Version, &status.Description, &appliedAt); err != nil {
			return nil, fmt.Errorf("failed to scan migration status: %w", err)
		}
		status.AppliedAt = appliedAt
		statuses = append(statuses, status)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migration rows: %w", err)
	}

	return statuses, nil
}

// AddMigration adds a migration to the runner
func (r *PostgresMigrationRunner) AddMigration(migration Migration) error {
	// Validate migration
	if migration.Version <= 0 {
		return errors.New("migration version must be positive")
	}
	if migration.Description == "" {
		return errors.New("migration description cannot be empty")
	}
	if migration.UpSQL == "" {
		return errors.New("up migration SQL cannot be empty")
	}
	if migration.DownSQL == "" {
		return errors.New("down migration SQL cannot be empty")
	}

	// Check for duplicate version
	for _, m := range r.migrations {
		if m.Version == migration.Version {
			return fmt.Errorf("migration with version %d already exists", migration.Version)
		}
	}

	r.migrations = append(r.migrations, migration)
	return nil
}

// LoadMigrations loads migrations from the specified directory
func (r *PostgresMigrationRunner) LoadMigrations(migrationsDir string) error {
	return filepath.WalkDir(migrationsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path != migrationsDir {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".sql") {
			return nil
		}

		return r.loadMigrationFile(path)
	})
}

// LoadMigrationsFS loads migrations from the specified filesystem
func (r *PostgresMigrationRunner) LoadMigrationsFS(fsys fs.FS, root string) error {
	return fs.WalkDir(fsys, root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path != root {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".sql") {
			return nil
		}

		content, err := fs.ReadFile(fsys, path)
		if err != nil {
			return fmt.Errorf("failed to read migration file %s: %w", path, err)
		}

		return r.parseMigrationContent(filepath.Base(path), string(content))
	})
}

// Helper methods

// getAppliedVersions returns a map of applied migration versions
func (r *PostgresMigrationRunner) getAppliedVersions(ctx context.Context) (map[int64]struct{}, error) {
	rows, err := r.db.Query(ctx, "SELECT version FROM schema_migrations")
	if err != nil {
		return nil, fmt.Errorf("failed to query applied migrations: %w", err)
	}
	defer rows.Close()

	versions := make(map[int64]struct{})
	for rows.Next() {
		var version int64
		if err := rows.Scan(&version); err != nil {
			return nil, fmt.Errorf("failed to scan migration version: %w", err)
		}
		versions[version] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating migration versions: %w", err)
	}

	return versions, nil
}

// loadMigrationFile loads a migration from a file
func (r *PostgresMigrationRunner) loadMigrationFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read migration file %s: %w", path, err)
	}

	return r.parseMigrationContent(filepath.Base(path), string(content))
}

// parseMigrationContent parses migration content from a file
func (r *PostgresMigrationRunner) parseMigrationContent(filename, content string) error {
	// Parse filename to extract version and description
	// Expected format: V{version}__{description}.sql
	// Example: V1__create_users_table.sql
	parts := strings.SplitN(filename, "__", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid migration filename format: %s", filename)
	}

	versionStr := strings.TrimPrefix(parts[0], "V")
	var version int64
	_, err := fmt.Sscanf(versionStr, "%d", &version)
	if err != nil {
		return fmt.Errorf("invalid migration version in filename: %s", filename)
	}

	description := strings.TrimSuffix(parts[1], ".sql")

	// Split content into up and down migrations
	sections := strings.Split(content, "-- DOWN")
	if len(sections) != 2 {
		return fmt.Errorf("migration file must contain '-- DOWN' separator: %s", filename)
	}

	upSQL := strings.TrimSpace(sections[0])
	downSQL := strings.TrimSpace(sections[1])

	// Add migration
	return r.AddMigration(Migration{
		Version:     version,
		Description: description,
		UpSQL:       upSQL,
		DownSQL:     downSQL,
	})
}
