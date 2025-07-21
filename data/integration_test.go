package data

// func TestPostgresIntegration(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping integration test in short mode")
// 	}

// 	ctx := context.Background()

// 	// Start PostgreSQL container
// 	postgresContainer, err := postgres.RunContainer(ctx,
// 		testcontainers.WithImage("postgres:15-alpine"),
// 		postgres.WithDatabase("testdb"),
// 		postgres.WithUsername("testuser"),
// 		postgres.WithPassword("testpass"),
// 		testcontainers.WithWaitStrategy(
// 			wait.ForLog("database system is ready to accept connections").
// 				WithOccurrence(2).
// 				WithStartupTimeout(5*time.Minute)),
// 	)
// 	if err != nil {
// 		t.Fatalf("Failed to start postgres container: %v", err)
// 	}
// 	defer func() {
// 		if err := postgresContainer.Terminate(ctx); err != nil {
// 			t.Fatalf("Failed to terminate postgres container: %v", err)
// 		}
// 	}()

// 	// Get connection details
// 	host, err := postgresContainer.Host(ctx)
// 	if err != nil {
// 		t.Fatalf("Failed to get postgres host: %v", err)
// 	}

// 	port, err := postgresContainer.MappedPort(ctx, "5432")
// 	if err != nil {
// 		t.Fatalf("Failed to get postgres port: %v", err)
// 	}

// 	// Create database configuration
// 	config := &PostgresConfig{
// 		Host:     host,
// 		Port:     port.Int(),
// 		Database: "testdb",
// 		Username: "testuser",
// 		Password: "testpass",
// 		SSLMode:  "disable",
// 		MaxConns: 10,
// 		MinConns: 1,
// 	}

// 	// Test database connection
// 	db, err := NewPostgresDatabase(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create postgres database: %v", err)
// 	}
// 	defer db.Close()

// 	// Test basic operations
// 	t.Run("BasicOperations", func(t *testing.T) {
// 		// Create a test table
// 		_, err := db.Exec(ctx, `
// 			CREATE TABLE test_users (
// 				id SERIAL PRIMARY KEY,
// 				name VARCHAR(100) NOT NULL,
// 				email VARCHAR(100) UNIQUE NOT NULL,
// 				created_at TIMESTAMP DEFAULT NOW()
// 			)
// 		`)
// 		if err != nil {
// 			t.Fatalf("Failed to create table: %v", err)
// 		}

// 		// Insert test data
// 		result, err := db.Exec(ctx, "INSERT INTO test_users (name, email) VALUES ($1, $2)", "John Doe", "john@example.com")
// 		if err != nil {
// 			t.Fatalf("Failed to insert data: %v", err)
// 		}

// 		rowsAffected := result.RowsAffected()
// 		if rowsAffected != 1 {
// 			t.Errorf("Expected 1 row affected, got %d", rowsAffected)
// 		}

// 		// Query data
// 		rows, err := db.Query(ctx, "SELECT id, name, email FROM test_users WHERE email = $1", "john@example.com")
// 		if err != nil {
// 			t.Fatalf("Failed to query data: %v", err)
// 		}
// 		defer rows.Close()

// 		var id int
// 		var name, email string
// 		if !rows.Next() {
// 			t.Fatal("Expected at least one row")
// 		}

// 		err = rows.Scan(&id, &name, &email)
// 		if err != nil {
// 			t.Fatalf("Failed to scan row: %v", err)
// 		}

// 		if name != "John Doe" || email != "john@example.com" {
// 			t.Errorf("Unexpected data: name=%s, email=%s", name, email)
// 		}
// 	})

// 	// Test transactions
// 	t.Run("Transactions", func(t *testing.T) {
// 		err := db.Transaction(ctx, func(tx Transaction) error {
// 			// Insert data within transaction
// 			_, err := tx.Exec(ctx, "INSERT INTO test_users (name, email) VALUES ($1, $2)", "Jane Doe", "jane@example.com")
// 			if err != nil {
// 				return err
// 			}

// 			// Query within transaction
// 			rows, err := tx.Query(ctx, "SELECT COUNT(*) FROM test_users")
// 			if err != nil {
// 				return err
// 			}
// 			defer rows.Close()

// 			var count int
// 			if rows.Next() {
// 				err = rows.Scan(&count)
// 				if err != nil {
// 					return err
// 				}
// 			}

// 			if count < 2 {
// 				t.Errorf("Expected at least 2 users, got %d", count)
// 			}

// 			return nil
// 		})

// 		if err != nil {
// 			t.Fatalf("Transaction failed: %v", err)
// 		}
// 	})

// 	// Test transaction rollback
// 	t.Run("TransactionRollback", func(t *testing.T) {
// 		// Get initial count
// 		rows, err := db.Query(ctx, "SELECT COUNT(*) FROM test_users")
// 		if err != nil {
// 			t.Fatalf("Failed to get initial count: %v", err)
// 		}
// 		defer rows.Close()

// 		var initialCount int
// 		if rows.Next() {
// 			err = rows.Scan(&initialCount)
// 			if err != nil {
// 				t.Fatalf("Failed to scan initial count: %v", err)
// 			}
// 		}

// 		// Transaction that should rollback
// 		err = db.Transaction(ctx, func(tx Transaction) error {
// 			_, err := tx.Exec(ctx, "INSERT INTO test_users (name, email) VALUES ($1, $2)", "Bob Smith", "bob@example.com")
// 			if err != nil {
// 				return err
// 			}

// 			// Force rollback by returning an error
// 			return fmt.Errorf("forced rollback")
// 		})

// 		if err == nil {
// 			t.Fatal("Expected transaction to fail")
// 		}

// 		// Verify count is unchanged
// 		rows, err = db.Query(ctx, "SELECT COUNT(*) FROM test_users")
// 		if err != nil {
// 			t.Fatalf("Failed to get final count: %v", err)
// 		}
// 		defer rows.Close()

// 		var finalCount int
// 		if rows.Next() {
// 			err = rows.Scan(&finalCount)
// 			if err != nil {
// 				t.Fatalf("Failed to scan final count: %v", err)
// 			}
// 		}

// 		if finalCount != initialCount {
// 			t.Errorf("Expected count to be unchanged: initial=%d, final=%d", initialCount, finalCount)
// 		}
// 	})
// }

// func TestPostgresMigrationIntegration(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping integration test in short mode")
// 	}

// 	ctx := context.Background()

// 	// Start PostgreSQL container
// 	postgresContainer, err := postgres.RunContainer(ctx,
// 		testcontainers.WithImage("postgres:15-alpine"),
// 		postgres.WithDatabase("testdb"),
// 		postgres.WithUsername("testuser"),
// 		postgres.WithPassword("testpass"),
// 		testcontainers.WithWaitStrategy(
// 			wait.ForLog("database system is ready to accept connections").
// 				WithOccurrence(2).
// 				WithStartupTimeout(5*time.Minute)),
// 	)
// 	if err != nil {
// 		t.Fatalf("Failed to start postgres container: %v", err)
// 	}
// 	defer func() {
// 		if err := postgresContainer.Terminate(ctx); err != nil {
// 			t.Fatalf("Failed to terminate postgres container: %v", err)
// 		}
// 	}()

// 	// Get connection details
// 	host, err := postgresContainer.Host(ctx)
// 	if err != nil {
// 		t.Fatalf("Failed to get postgres host: %v", err)
// 	}

// 	port, err := postgresContainer.MappedPort(ctx, "5432")
// 	if err != nil {
// 		t.Fatalf("Failed to get postgres port: %v", err)
// 	}

// 	// Create database configuration
// 	config := &PostgresConfig{
// 		Host:     host,
// 		Port:     port.Int(),
// 		Database: "testdb",
// 		Username: "testuser",
// 		Password: "testpass",
// 		SSLMode:  "disable",
// 		MaxConns: 10,
// 		MinConns: 1,
// 	}

// 	// Create database connection
// 	db, err := NewPostgresDatabase(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create postgres database: %v", err)
// 	}
// 	defer db.Close()

// 	// Test migration runner
// 	runner := NewPostgresMigrationRunner(db)

// 	// Initialize migration system
// 	err = runner.Initialize(ctx)
// 	if err != nil {
// 		t.Fatalf("Failed to initialize migration runner: %v", err)
// 	}

// 	// Add test migrations
// 	migration1 := &Migration{
// 		Version:     1,
// 		Description: "Create users table",
// 		UpSQL: `
// 			CREATE TABLE users (
// 				id SERIAL PRIMARY KEY,
// 				name VARCHAR(100) NOT NULL,
// 				email VARCHAR(100) UNIQUE NOT NULL,
// 				created_at TIMESTAMP DEFAULT NOW()
// 			);
// 		`,
// 		DownSQL: `DROP TABLE users;`,
// 	}

// 	migration2 := &Migration{
// 		Version:     2,
// 		Description: "Add index on email",
// 		UpSQL:       `CREATE INDEX idx_users_email ON users(email);`,
// 		DownSQL:     `DROP INDEX idx_users_email;`,
// 	}

// 	runner.AddMigration(migration1)
// 	runner.AddMigration(migration2)

// 	// Apply migrations
// 	err = runner.Apply(ctx)
// 	if err != nil {
// 		t.Fatalf("Failed to apply migrations: %v", err)
// 	}

// 	// Verify table exists
// 	var exists bool
// 	err = db.QueryRow(ctx, `
// 		SELECT EXISTS (
// 			SELECT FROM information_schema.tables
// 			WHERE table_schema = 'public'
// 			AND table_name = 'users'
// 		)
// 	`).Scan(&exists)
// 	if err != nil {
// 		t.Fatalf("Failed to check table existence: %v", err)
// 	}

// 	if !exists {
// 		t.Error("Users table should exist after migration")
// 	}

// 	// Test rollback
// 	err = runner.Rollback(ctx, 1)
// 	if err != nil {
// 		t.Fatalf("Failed to rollback migration: %v", err)
// 	}

// 	// Verify index was removed
// 	var indexExists bool
// 	err = db.QueryRow(ctx, `
// 		SELECT EXISTS (
// 			SELECT FROM pg_indexes
// 			WHERE tablename = 'users'
// 			AND indexname = 'idx_users_email'
// 		)
// 	`).Scan(&indexExists)
// 	if err != nil {
// 		t.Fatalf("Failed to check index existence: %v", err)
// 	}

// 	if indexExists {
// 		t.Error("Index should not exist after rollback")
// 	}
// }

// func TestRedisIntegration(t *testing.T) {
// 	if testing.Short() {
// 		t.Skip("Skipping integration test in short mode")
// 	}

// 	ctx := context.Background()

// 	// Start Redis container
// 	redisContainer, err := redis.RunContainer(ctx,
// 		testcontainers.WithImage("redis:7-alpine"),
// 		redis.WithSnapshotting(10, 1),
// 		redis.WithLogLevel(redis.LogLevelVerbose),
// 	)
// 	if err != nil {
// 		t.Fatalf("Failed to start redis container: %v", err)
// 	}
// 	defer func() {
// 		if err := redisContainer.Terminate(ctx); err != nil {
// 			t.Fatalf("Failed to terminate redis container: %v", err)
// 		}
// 	}()

// 	// Get connection details
// 	host, err := redisContainer.Host(ctx)
// 	if err != nil {
// 		t.Fatalf("Failed to get redis host: %v", err)
// 	}

// 	port, err := redisContainer.MappedPort(ctx, "6379")
// 	if err != nil {
// 		t.Fatalf("Failed to get redis port: %v", err)
// 	}

// 	// Create cache configuration
// 	config := &RedisConfig{
// 		Address:  fmt.Sprintf("%s:%s", host, port.Port()),
// 		Password: "",
// 		DB:       0,
// 		PoolSize: 10,
// 	}

// 	// Test cache operations
// 	cache, err := NewRedisCache(config)
// 	if err != nil {
// 		t.Fatalf("Failed to create redis cache: %v", err)
// 	}
// 	defer cache.Close()

// 	t.Run("BasicOperations", func(t *testing.T) {
// 		key := "test:key"
// 		value := []byte("test value")

// 		// Set value
// 		err := cache.Set(ctx, key, value, time.Minute)
// 		if err != nil {
// 			t.Fatalf("Failed to set value: %v", err)
// 		}

// 		// Get value
// 		retrieved, err := cache.Get(ctx, key)
// 		if err != nil {
// 			t.Fatalf("Failed to get value: %v", err)
// 		}

// 		if string(retrieved) != string(value) {
// 			t.Errorf("Expected %s, got %s", string(value), string(retrieved))
// 		}

// 		// Delete value
// 		err = cache.Delete(ctx, key)
// 		if err != nil {
// 			t.Fatalf("Failed to delete value: %v", err)
// 		}

// 		// Verify deletion
// 		_, err = cache.Get(ctx, key)
// 		if err == nil {
// 			t.Error("Expected error when getting deleted key")
// 		}
// 	})

// 	t.Run("TTLExpiration", func(t *testing.T) {
// 		key := "test:ttl"
// 		value := []byte("ttl test")

// 		// Set value with short TTL
// 		err := cache.Set(ctx, key, value, 100*time.Millisecond)
// 		if err != nil {
// 			t.Fatalf("Failed to set value with TTL: %v", err)
// 		}

// 		// Verify value exists
// 		retrieved, err := cache.Get(ctx, key)
// 		if err != nil {
// 			t.Fatalf("Failed to get value before expiration: %v", err)
// 		}

// 		if string(retrieved) != string(value) {
// 			t.Errorf("Expected %s, got %s", string(value), string(retrieved))
// 		}

// 		// Wait for expiration
// 		time.Sleep(200 * time.Millisecond)

// 		// Verify value expired
// 		_, err = cache.Get(ctx, key)
// 		if err == nil {
// 			t.Error("Expected error when getting expired key")
// 		}
// 	})

// 	t.Run("Namespace", func(t *testing.T) {
// 		// Create cache with namespace
// 		namespacedConfig := *config
// 		namespacedConfig.Namespace = "test"

// 		namespacedCache, err := NewRedisCache(&namespacedConfig)
// 		if err != nil {
// 			t.Fatalf("Failed to create namespaced cache: %v", err)
// 		}
// 		defer namespacedCache.Close()

// 		key := "namespaced:key"
// 		value := []byte("namespaced value")

// 		// Set value in namespaced cache
// 		err = namespacedCache.Set(ctx, key, value, time.Minute)
// 		if err != nil {
// 			t.Fatalf("Failed to set namespaced value: %v", err)
// 		}

// 		// Try to get from non-namespaced cache (should not exist)
// 		_, err = cache.Get(ctx, key)
// 		if err == nil {
// 			t.Error("Expected error when getting namespaced key from non-namespaced cache")
// 		}

// 		// Get from namespaced cache (should exist)
// 		retrieved, err := namespacedCache.Get(ctx, key)
// 		if err != nil {
// 			t.Fatalf("Failed to get namespaced value: %v", err)
// 		}

// 		if string(retrieved) != string(value) {
// 			t.Errorf("Expected %s, got %s", string(value), string(retrieved))
// 		}
// 	})
// }
