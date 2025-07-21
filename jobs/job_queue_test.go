package jobs

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/santif/microlib/observability"
)

// TestJobQueue_Enqueue tests the Enqueue method
func TestJobQueue_Enqueue(t *testing.T) {
	queue := NewMemoryJobQueue(DefaultJobQueueConfig(), observability.NewMetrics())
	defer queue.Close()

	// Create a test job
	job := NewTestJob("test-job", "Test Job", nil)

	// Test enqueuing a job
	err := queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Test enqueuing the same job again (should fail)
	err = queue.Enqueue(context.Background(), job)
	if err == nil {
		t.Fatal("Expected error when enqueuing the same job twice, got nil")
	}
	if err != ErrJobAlreadyExists {
		t.Fatalf("Expected ErrJobAlreadyExists, got %v", err)
	}

	// Test enqueuing a nil job (should fail)
	err = queue.Enqueue(context.Background(), nil)
	if err == nil {
		t.Fatal("Expected error when enqueuing nil job, got nil")
	}
	if err != ErrInvalidJobID {
		t.Fatalf("Expected ErrInvalidJobID, got %v", err)
	}
}

// TestJobQueue_Process tests the Process method
func TestJobQueue_Process(t *testing.T) {
	queue := NewMemoryJobQueue(DefaultJobQueueConfig(), observability.NewMetrics())
	defer queue.Close()

	// Create a channel to signal job completion
	done := make(chan struct{})

	// Create a test job that signals completion
	job := NewTestJob("test-job", "Test Job", func(ctx context.Context) error {
		close(done)
		return nil
	})

	// Enqueue the job
	err := queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Start processing jobs
	err = queue.Process(context.Background(), func(ctx context.Context, j Job) error {
		return j.Execute(ctx)
	})
	if err != nil {
		t.Fatalf("Failed to start processing jobs: %v", err)
	}

	// Wait for job to complete or timeout
	select {
	case <-done:
		// Job completed successfully
	case <-time.After(2 * time.Second):
		t.Fatal("Timed out waiting for job to complete")
	}

	// Check job status
	info, err := queue.Get(context.Background(), job.ID())
	if err != nil {
		t.Fatalf("Failed to get job info: %v", err)
	}

	if info.Status != JobStatusCompleted {
		t.Fatalf("Expected job status to be %s, got %s", JobStatusCompleted, info.Status)
	}
}

// TestJobQueue_Cancel tests the Cancel method
func TestJobQueue_Cancel(t *testing.T) {
	queue := NewMemoryJobQueue(DefaultJobQueueConfig(), observability.NewMetrics())
	defer queue.Close()

	// Create a test job
	job := NewTestJob("test-job", "Test Job", nil)

	// Enqueue the job
	err := queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Cancel the job
	err = queue.Cancel(context.Background(), job.ID())
	if err != nil {
		t.Fatalf("Failed to cancel job: %v", err)
	}

	// Check job status
	info, err := queue.Get(context.Background(), job.ID())
	if err != nil {
		t.Fatalf("Failed to get job info: %v", err)
	}

	if info.Status != JobStatusCancelled {
		t.Fatalf("Expected job status to be %s, got %s", JobStatusCancelled, info.Status)
	}

	// Try to cancel a non-existent job
	err = queue.Cancel(context.Background(), "non-existent-job")
	if err == nil {
		t.Fatal("Expected error when cancelling non-existent job, got nil")
	}
	if err != ErrJobNotFound {
		t.Fatalf("Expected ErrJobNotFound, got %v", err)
	}
}

// TestJobQueue_Get tests the Get method
func TestJobQueue_Get(t *testing.T) {
	queue := NewMemoryJobQueue(DefaultJobQueueConfig(), observability.NewMetrics())
	defer queue.Close()

	// Create a test job
	job := NewTestJob("test-job", "Test Job", nil)

	// Enqueue the job
	err := queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Get job info
	info, err := queue.Get(context.Background(), job.ID())
	if err != nil {
		t.Fatalf("Failed to get job info: %v", err)
	}

	// Check job info
	if info.ID != job.ID() {
		t.Fatalf("Expected job ID to be %s, got %s", job.ID(), info.ID)
	}
	if info.Name != job.Name() {
		t.Fatalf("Expected job name to be %s, got %s", job.Name(), info.Name)
	}
	if info.Status != JobStatusPending {
		t.Fatalf("Expected job status to be %s, got %s", JobStatusPending, info.Status)
	}

	// Try to get a non-existent job
	_, err = queue.Get(context.Background(), "non-existent-job")
	if err == nil {
		t.Fatal("Expected error when getting non-existent job, got nil")
	}
	if err != ErrJobNotFound {
		t.Fatalf("Expected ErrJobNotFound, got %v", err)
	}
}

// TestJobQueue_List tests the List method
func TestJobQueue_List(t *testing.T) {
	queue := NewMemoryJobQueue(DefaultJobQueueConfig(), observability.NewMetrics())
	defer queue.Close()

	// Create and enqueue multiple jobs
	for i := 0; i < 5; i++ {
		job := NewTestJob(fmt.Sprintf("test-job-%d", i), fmt.Sprintf("Test Job %d", i), nil)
		err := queue.Enqueue(context.Background(), job)
		if err != nil {
			t.Fatalf("Failed to enqueue job: %v", err)
		}
	}

	// List all jobs
	jobs, err := queue.List(context.Background(), JobFilter{})
	if err != nil {
		t.Fatalf("Failed to list jobs: %v", err)
	}

	// Check job count
	if len(jobs) != 5 {
		t.Fatalf("Expected 5 jobs, got %d", len(jobs))
	}

	// List with limit
	jobs, err = queue.List(context.Background(), JobFilter{Limit: 3})
	if err != nil {
		t.Fatalf("Failed to list jobs with limit: %v", err)
	}

	// Check job count with limit
	if len(jobs) != 3 {
		t.Fatalf("Expected 3 jobs with limit, got %d", len(jobs))
	}

	// List with offset
	jobs, err = queue.List(context.Background(), JobFilter{Offset: 2})
	if err != nil {
		t.Fatalf("Failed to list jobs with offset: %v", err)
	}

	// Check job count with offset
	if len(jobs) != 3 {
		t.Fatalf("Expected 3 jobs with offset, got %d", len(jobs))
	}

	// List with status filter
	jobs, err = queue.List(context.Background(), JobFilter{Status: []JobStatus{JobStatusPending}})
	if err != nil {
		t.Fatalf("Failed to list jobs with status filter: %v", err)
	}

	// Check all jobs have the correct status
	for _, job := range jobs {
		if job.Status != JobStatusPending {
			t.Fatalf("Expected job status to be %s, got %s", JobStatusPending, job.Status)
		}
	}
}

// TestJobQueue_Retry tests the retry functionality
func TestJobQueue_Retry(t *testing.T) {
	config := DefaultJobQueueConfig()
	config.WorkerCount = 1
	queue := NewMemoryJobQueue(config, observability.NewMetrics())
	defer queue.Close()

	// Create a mutex to protect the retry count
	var mu sync.Mutex
	retryCount := 0

	// Create a test job that fails the first two times
	job := NewTestJob("retry-job", "Retry Job", func(ctx context.Context) error {
		mu.Lock()
		defer mu.Unlock()

		if retryCount < 2 {
			retryCount++
			return errors.New("simulated failure")
		}
		return nil
	})

	// Set retry policy
	job.metadata.RetryPolicy = RetryPolicy{
		MaxRetries:          3,
		InitialInterval:     50 * time.Millisecond,
		MaxInterval:         1 * time.Second,
		Multiplier:          2.0,
		RandomizationFactor: 0.1,
	}

	// Enqueue the job
	err := queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Create a channel to signal job completion
	done := make(chan struct{})

	// Start processing jobs
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	err = queue.Process(ctx, func(ctx context.Context, j Job) error {
		err := j.Execute(ctx)
		if err == nil {
			// Job succeeded, signal completion
			close(done)
		}
		return err
	})
	if err != nil {
		t.Fatalf("Failed to start processing jobs: %v", err)
	}

	// Wait for job to complete or timeout
	select {
	case <-done:
		// Job completed successfully
	case <-time.After(5 * time.Second):
		t.Fatal("Timed out waiting for job to complete")
	}

	// Check retry count
	mu.Lock()
	if retryCount != 2 {
		t.Fatalf("Expected retry count to be 2, got %d", retryCount)
	}
	mu.Unlock()

	// Check job status
	info, err := queue.Get(context.Background(), job.ID())
	if err != nil {
		t.Fatalf("Failed to get job info: %v", err)
	}

	if info.Status != JobStatusCompleted {
		t.Fatalf("Expected job status to be %s, got %s", JobStatusCompleted, info.Status)
	}
}

// TestJobQueue_Close tests the Close method
func TestJobQueue_Close(t *testing.T) {
	queue := NewMemoryJobQueue(DefaultJobQueueConfig(), observability.NewMetrics())

	// Close the queue
	err := queue.Close()
	if err != nil {
		t.Fatalf("Failed to close queue: %v", err)
	}

	// Try to enqueue a job after closing (should fail)
	job := NewTestJob("test-job", "Test Job", nil)
	err = queue.Enqueue(context.Background(), job)
	if err == nil {
		t.Fatal("Expected error when enqueuing job after closing, got nil")
	}
	if err != ErrQueueClosed {
		t.Fatalf("Expected ErrQueueClosed, got %v", err)
	}

	// Try to process jobs after closing (should fail)
	err = queue.Process(context.Background(), func(ctx context.Context, j Job) error {
		return nil
	})
	if err == nil {
		t.Fatal("Expected error when processing jobs after closing, got nil")
	}
	if err != ErrQueueClosed {
		t.Fatalf("Expected ErrQueueClosed, got %v", err)
	}
}
