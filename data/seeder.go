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
)

// Seed represents a database seed
type Seed struct {
	Name     string
	SQL      string
	Priority int // Lower priority seeds are executed first
}

// Seeder is responsible for seeding the database
type Seeder interface {
	// Initialize creates the seeds table if it doesn't exist
	Initialize(ctx context.Context) error

	// Apply applies all seeds that haven't been applied yet
	Apply(ctx context.Context) error

	// Reset removes all seed records and allows reapplying seeds
	Reset(ctx context.Context) error

	// AddSeed adds a seed to the seeder
	AddSeed(seed Seed) error

	// LoadSeeds loads seeds from the specified directory
	LoadSeeds(seedsDir string) error

	// LoadSeedsFS loads seeds from the specified filesystem
	LoadSeedsFS(fsys fs.FS, root string) error
}

// PostgresSeeder implements Seeder for PostgreSQL
type PostgresSeeder struct {
	db    Database
	seeds []Seed
}

// NewPostgresSeeder creates a new PostgreSQL seeder
func NewPostgresSeeder(db Database) *PostgresSeeder {
	return &PostgresSeeder{
		db:    db,
		seeds: []Seed{},
	}
}

// Initialize creates the seeds table if it doesn't exist
func (s *PostgresSeeder) Initialize(ctx context.Context) error {
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS schema_seeds (
		name TEXT PRIMARY KEY,
		applied_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
	);
	`

	_, err := s.db.Exec(ctx, createTableSQL)
	if err != nil {
		return fmt.Errorf("failed to create seeds table: %w", err)
	}

	return nil
}

// Apply applies all seeds that haven't been applied yet
func (s *PostgresSeeder) Apply(ctx context.Context) error {
	if err := s.Initialize(ctx); err != nil {
		return err
	}

	// Get applied seeds
	appliedSeeds, err := s.getAppliedSeeds(ctx)
	if err != nil {
		return err
	}

	// Sort seeds by priority
	sort.Slice(s.seeds, func(i, j int) bool {
		return s.seeds[i].Priority < s.seeds[j].Priority
	})

	// Apply pending seeds
	for _, seed := range s.seeds {
		// Skip if already applied
		if _, ok := appliedSeeds[seed.Name]; ok {
			continue
		}

		// Apply seed within a transaction
		err := s.db.Transaction(ctx, func(tx Transaction) error {
			// Execute seed
			_, err := tx.Exec(ctx, seed.SQL)
			if err != nil {
				return fmt.Errorf("failed to apply seed %s: %w", seed.Name, err)
			}

			// Record seed
			_, err = tx.Exec(
				ctx,
				"INSERT INTO schema_seeds (name) VALUES ($1)",
				seed.Name,
			)
			if err != nil {
				return fmt.Errorf("failed to record seed %s: %w", seed.Name, err)
			}

			return nil
		})

		if err != nil {
			return err
		}

		fmt.Printf("Applied seed: %s\n", seed.Name)
	}

	return nil
}

// Reset removes all seed records and allows reapplying seeds
func (s *PostgresSeeder) Reset(ctx context.Context) error {
	if err := s.Initialize(ctx); err != nil {
		return err
	}

	_, err := s.db.Exec(ctx, "DELETE FROM schema_seeds")
	if err != nil {
		return fmt.Errorf("failed to reset seeds: %w", err)
	}

	fmt.Println("Reset all seed records. Seeds can be reapplied.")
	return nil
}

// AddSeed adds a seed to the seeder
func (s *PostgresSeeder) AddSeed(seed Seed) error {
	// Validate seed
	if seed.Name == "" {
		return errors.New("seed name cannot be empty")
	}
	if seed.SQL == "" {
		return errors.New("seed SQL cannot be empty")
	}

	// Check for duplicate name
	for _, s := range s.seeds {
		if s.Name == seed.Name {
			return fmt.Errorf("seed with name %s already exists", seed.Name)
		}
	}

	s.seeds = append(s.seeds, seed)
	return nil
}

// LoadSeeds loads seeds from the specified directory
func (s *PostgresSeeder) LoadSeeds(seedsDir string) error {
	return filepath.WalkDir(seedsDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() && path != seedsDir {
			return filepath.SkipDir
		}

		if !strings.HasSuffix(path, ".sql") {
			return nil
		}

		return s.loadSeedFile(path)
	})
}

// LoadSeedsFS loads seeds from the specified filesystem
func (s *PostgresSeeder) LoadSeedsFS(fsys fs.FS, root string) error {
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
			return fmt.Errorf("failed to read seed file %s: %w", path, err)
		}

		return s.parseSeedContent(filepath.Base(path), string(content))
	})
}

// Helper methods

// getAppliedSeeds returns a map of applied seed names
func (s *PostgresSeeder) getAppliedSeeds(ctx context.Context) (map[string]struct{}, error) {
	rows, err := s.db.Query(ctx, "SELECT name FROM schema_seeds")
	if err != nil {
		return nil, fmt.Errorf("failed to query applied seeds: %w", err)
	}
	defer rows.Close()

	seeds := make(map[string]struct{})
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, fmt.Errorf("failed to scan seed name: %w", err)
		}
		seeds[name] = struct{}{}
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating seed names: %w", err)
	}

	return seeds, nil
}

// loadSeedFile loads a seed from a file
func (s *PostgresSeeder) loadSeedFile(path string) error {
	content, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read seed file %s: %w", path, err)
	}

	return s.parseSeedContent(filepath.Base(path), string(content))
}

// parseSeedContent parses seed content from a file
func (s *PostgresSeeder) parseSeedContent(filename, content string) error {
	// Parse filename to extract priority and name
	// Expected format: S{priority}_{name}.sql
	// Example: S01_users.sql
	parts := strings.SplitN(filename, "_", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid seed filename format: %s", filename)
	}

	priorityStr := strings.TrimPrefix(parts[0], "S")
	var priority int
	_, err := fmt.Sscanf(priorityStr, "%d", &priority)
	if err != nil {
		return fmt.Errorf("invalid seed priority in filename: %s", filename)
	}

	name := strings.TrimSuffix(parts[1], ".sql")

	// Add seed
	return s.AddSeed(Seed{
		Name:     name,
		SQL:      strings.TrimSpace(content),
		Priority: priority,
	})
}
