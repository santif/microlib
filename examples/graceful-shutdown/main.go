package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"syscall"
	"time"

	"github.com/santif/microlib/core"
)

// Example dependency that simulates a database connection
type DatabaseConnection struct {
	connected bool
}

func (db *DatabaseConnection) Name() string {
	return "database"
}

func (db *DatabaseConnection) HealthCheck(ctx context.Context) error {
	if !db.connected {
		return fmt.Errorf("database not connected")
	}
	return nil
}

func (db *DatabaseConnection) Connect() {
	db.connected = true
	fmt.Println("✓ Database connected")
}

func (db *DatabaseConnection) Disconnect() {
	db.connected = false
	fmt.Println("✓ Database disconnected")
}

// Example dependency that simulates an HTTP server
type HTTPServer struct {
	running bool
}

func (s *HTTPServer) Name() string {
	return "http-server"
}

func (s *HTTPServer) HealthCheck(ctx context.Context) error {
	if !s.running {
		return fmt.Errorf("http server not running")
	}
	return nil
}

func (s *HTTPServer) Start() {
	s.running = true
	fmt.Println("✓ HTTP server started on :8080")
}

func (s *HTTPServer) Stop() {
	s.running = false
	fmt.Println("✓ HTTP server stopped")
}

func main() {
	// Check if we're running in test mode (non-blocking)
	testMode := len(os.Args) > 1 && os.Args[1] == "--test"

	// Create service metadata
	metadata := core.ServiceMetadata{
		Name:      "graceful-shutdown-example",
		Version:   "1.0.0",
		Instance:  "example-1",
		BuildHash: "abc123",
	}

	fmt.Printf("🚀 Starting service: %s v%s (instance: %s)\n", 
		metadata.Name, metadata.Version, metadata.Instance)

	// Create service
	service := core.NewService(metadata)

	// Create and add dependencies
	db := &DatabaseConnection{}
	db.Connect()
	service.AddDependency(db)

	httpServer := &HTTPServer{}
	httpServer.Start()
	service.AddDependency(httpServer)

	// Register shutdown hooks (executed in reverse order - LIFO)
	// Hook 1: Final cleanup (executed last)
	service.RegisterShutdownHook(func(ctx context.Context) error {
		fmt.Println("🧹 Performing final cleanup...")
		time.Sleep(100 * time.Millisecond) // Simulate cleanup work
		fmt.Println("✓ Final cleanup completed")
		return nil
	})

	// Hook 2: Close database connection (executed second)
	service.RegisterShutdownHook(func(ctx context.Context) error {
		fmt.Println("🗄️  Closing database connection...")
		time.Sleep(200 * time.Millisecond) // Simulate database cleanup
		db.Disconnect()
		return nil
	})

	// Hook 3: Stop HTTP server (executed first)
	service.RegisterShutdownHook(func(ctx context.Context) error {
		fmt.Println("🌐 Shutting down HTTP server...")
		time.Sleep(300 * time.Millisecond) // Simulate server shutdown
		httpServer.Stop()
		return nil
	})

	// Hook 4: Log shutdown start (executed first due to LIFO order)
	service.RegisterShutdownHook(func(ctx context.Context) error {
		fmt.Println("🛑 Graceful shutdown initiated...")
		return nil
	})

	// Register startup hooks
	service.RegisterStartupHook(func(ctx context.Context) error {
		fmt.Println("🔍 Performing startup validation...")
		time.Sleep(100 * time.Millisecond) // Simulate startup work
		fmt.Println("✓ Startup validation completed")
		return nil
	})
	
	// Add a custom health check
	service.HealthChecker().AddCheck("memory", func(ctx context.Context) error {
		// Simulate memory check
		fmt.Println("🧠 Checking memory usage...")
		return nil // Memory usage is OK
	})
	
	// Configure the service with a 10-second shutdown timeout
	service.WithShutdownTimeout(10 * time.Second)
	
	// If we're in test mode, handle differently
	if testMode {
		fmt.Println("🧪 Running in test mode - will automatically shutdown after 1 second")
		
		// Start the service (this will validate dependencies and run startup hooks)
		ctx := context.Background()
		err := service.Start(ctx)
		if err != nil {
			log.Fatalf("❌ Service failed to start: %v", err)
		}
		
		// Print health check results
		healthResults := service.CheckHealth(ctx)
		resultsJSON, _ := json.MarshalIndent(healthResults, "", "  ")
		fmt.Println("🏥 Health check results:")
		fmt.Println(string(resultsJSON))
		
		fmt.Println("✓ All dependencies validated and service ready")
		fmt.Println("📡 Service is now accepting traffic")
		
		// Wait a bit to simulate running
		time.Sleep(1 * time.Second)
		
		// Trigger shutdown programmatically
		fmt.Println("🔄 Triggering programmatic shutdown...")
		service.Trigger(syscall.SIGTERM)
		
		// Perform shutdown
		if err := service.Shutdown(service.Metadata().StartTime.Add(10 * time.Second).Sub(time.Now())); err != nil {
			log.Fatalf("❌ Service failed to shutdown: %v", err)
		}
		
		fmt.Println("✅ Service shutdown completed successfully")
		return
	}
	
	// Normal mode - wait for OS signals
	fmt.Println("💡 Press Ctrl+C (SIGINT) or send SIGTERM to trigger graceful shutdown")
	fmt.Println("⏱️  Shutdown timeout is set to 10 seconds")
	
	// Run the service
	// This method handles:
	// 1. Starting the service and validating dependencies
	// 2. Setting up signal handlers for SIGTERM and SIGINT
	// 3. Waiting for shutdown signals
	// 4. Executing shutdown hooks in reverse order
	// 5. Respecting the configured shutdown timeout
	ctx := context.Background()
	if err := service.Run(ctx); err != nil {
		log.Fatalf("❌ Service failed: %v", err)
	}

	fmt.Println("✅ Service shutdown completed successfully")
}