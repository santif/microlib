package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/santif/microlib/observability"
)

func TestDeadLetterQueue(t *testing.T) {
	config := DefaultJobQueueConfig()
	config.WorkerCount = 1
	config.DeadLetterEnabled = true
	queue := NewMemoryJobQueue(config, observability.NewMetrics())
	defer queue.Close()

	// Create a job that will always fail
	job := &TestJob{
		id:   "failing-job",
		name: "Failing Job",
		metadata: JobMetadata{
			RetryPolicy: RetryPolicy{
				Type:            RetryPolicyTypeFixed,
				MaxRetries:      2,
				InitialInterval: 10 * time.Millisecond,
			},
		},
		execFunc: func(ctx context.Context) error {
			return errors.New("job always fails")
		},
	}

	// Enqueue the job
	err := queue.Enqueue(context.Background(), job)
	if err != nil {
		t.Fatalf("Failed to enqueue job: %v", err)
	}

	// Start processing
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	err = queue.Process(ctx, func(ctx context.Context, job Job) error {
		return job.Execute(ctx)
	})
	if err != nil {
		t.Fatalf("Failed to start processing: %v", err)
	}

	// Wait for the job to be processed and moved to dead letter queue
	time.Sleep(500 * time.Millisecond)

	// Check that the job is in the dead letter queue
	deadJobs, err := queue.GetDeadLetteredJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("Failed to get dead lettered jobs: %v", err)
	}

	if len(deadJobs) != 1 {
		t.Fatalf("Expected 1 dead lettered job, got %d", len(deadJobs))
	}

	deadJob := deadJobs[0]
	if deadJob.ID != "failing-job" {
		t.Errorf("Expected dead job ID to be 'failing-job', got %s", deadJob.ID)
	}

	if deadJob.Status != JobStatusDeadLettered {
		t.Errorf("Expected dead job status to be %s, got %s", JobStatusDeadLettered, deadJob.Status)
	}

	if deadJob.RetryCount != 2 {
		t.Errorf("Expected dead job retry count to be 2, got %d", deadJob.RetryCount)
	}

	// Test requeuing the dead lettered job
	err = queue.RequeueDeadLetteredJob(context.Background(), "failing-job")
	if err != nil {
		t.Fatalf("Failed to requeue dead lettered job: %v", err)
	}

	// Check that the job is no longer in the dead letter queue
	deadJobs, err = queue.GetDeadLetteredJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("Failed to get dead lettered jobs after requeue: %v", err)
	}

	if len(deadJobs) != 0 {
		t.Errorf("Expected 0 dead lettered jobs after requeue, got %d", len(deadJobs))
	}

	// Test purging a dead lettered job
	// First, let the job fail again
	time.Sleep(500 * time.Millisecond)

	// Get the dead lettered job again
	deadJobs, err = queue.GetDeadLetteredJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("Failed to get dead lettered jobs for purge test: %v", err)
	}

	if len(deadJobs) != 1 {
		t.Fatalf("Expected 1 dead lettered job for purge test, got %d", len(deadJobs))
	}

	// Purge the dead lettered job
	err = queue.PurgeDeadLetteredJob(context.Background(), "failing-job")
	if err != nil {
		t.Fatalf("Failed to purge dead lettered job: %v", err)
	}

	// Check that the job is completely removed
	deadJobs, err = queue.GetDeadLetteredJobs(context.Background(), 10)
	if err != nil {
		t.Fatalf("Failed to get dead lettered jobs after purge: %v", err)
	}

	if len(deadJobs) != 0 {
		t.Errorf("Expected 0 dead lettered jobs after purge, got %d", len(deadJobs))
	}

	// Check that the job is also removed from the main jobs map
	_, err = queue.Get(context.Background(), "failing-job")
	if err != ErrJobNotFound {
		t.Errorf("Expected ErrJobNotFound after purge, got %v", err)
	}
}
