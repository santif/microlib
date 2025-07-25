package jobs

import (
	"context"
	"time"
)

// Scheduler is an interface for scheduling and managing cron jobs
type Scheduler interface {
	// Schedule registers a job to be executed according to a cron specification
	Schedule(spec string, job Job) error

	// Start begins the scheduler, executing jobs according to their schedules
	Start(ctx context.Context) error

	// Stop gracefully stops the scheduler, allowing in-progress jobs to complete
	Stop(ctx context.Context) error

	// Jobs returns all registered jobs
	Jobs() []JobInfo
}

// Job represents a scheduled task that can be executed
type Job interface {
	// ID returns the unique identifier for this job
	ID() string

	// Name returns a human-readable name for this job
	Name() string

	// Execute runs the job with the provided context
	Execute(ctx context.Context) error

	// Metadata returns job metadata
	Metadata() JobMetadata
}

// JobMetadata contains configuration and metadata for a job
type JobMetadata struct {
	// Description provides details about the job's purpose
	Description string

	// Owner identifies the team or individual responsible for the job
	Owner string

	// Timeout specifies the maximum duration the job should run
	Timeout time.Duration

	// Singleton indicates if the job should only run on one instance at a time
	Singleton bool

	// RetryCount specifies how many times to retry on failure (deprecated, use RetryPolicy)
	RetryCount int

	// RetryPolicy defines how the job should be retried on failure
	RetryPolicy RetryPolicy

	// Tags are arbitrary key-value pairs for job categorization
	Tags map[string]string
}

// JobInfo provides information about a scheduled job
type JobInfo struct {
	// ID is the unique identifier of the job
	ID string

	// Name is the human-readable name of the job
	Name string

	// Spec is the cron specification for when the job runs (for scheduled jobs)
	Spec string

	// NextRun is the next scheduled execution time (for scheduled jobs)
	NextRun time.Time

	// LastRun is the previous execution time, if any (for scheduled jobs)
	LastRun *time.Time

	// LastError contains the error from the last execution, if any
	LastError error

	// Status is the current status of the job (for queue jobs)
	Status JobStatus

	// EnqueuedAt is when the job was added to the queue (for queue jobs)
	EnqueuedAt time.Time

	// StartedAt is when the job execution began (for queue jobs)
	StartedAt *time.Time

	// FinishedAt is when the job execution completed (for queue jobs)
	FinishedAt *time.Time

	// Error contains any error that occurred during execution (for queue jobs)
	Error error

	// RetryCount is the number of retry attempts (for queue jobs)
	RetryCount int

	// NextRetry is when the job will be retried next (for queue jobs)
	NextRetry *time.Time

	// Metadata contains the job's metadata
	Metadata JobMetadata
}

// JobStatus represents the current state of a job execution
type JobStatus string

const (
	// JobStatusPending indicates the job is scheduled but not yet running
	JobStatusPending JobStatus = "pending"

	// JobStatusRunning indicates the job is currently executing
	JobStatusRunning JobStatus = "running"

	// JobStatusCompleted indicates the job completed successfully
	JobStatusCompleted JobStatus = "completed"

	// JobStatusFailed indicates the job failed to complete
	JobStatusFailed JobStatus = "failed"

	// JobStatusDeadLettered indicates the job failed and exceeded retry attempts
	JobStatusDeadLettered JobStatus = "dead_lettered"

	// JobStatusCancelled indicates the job was cancelled
	JobStatusCancelled JobStatus = "cancelled"
)

// JobExecution represents a single execution of a job
type JobExecution struct {
	// JobID is the ID of the job that was executed
	JobID string

	// ExecutionID is a unique identifier for this specific execution
	ExecutionID string

	// StartTime is when the job execution began
	StartTime time.Time

	// EndTime is when the job execution completed, if it has
	EndTime *time.Time

	// Status is the current status of the job execution
	Status JobStatus

	// Error contains any error that occurred during execution
	Error error

	// InstanceID identifies which service instance executed the job
	InstanceID string
}
