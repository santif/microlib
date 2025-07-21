package jobs

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/santif/microlib/observability"
)

// Common errors for job queue operations
var (
	ErrJobNotFound      = errors.New("job not found")
	ErrQueueClosed      = errors.New("job queue is closed")
	ErrInvalidJobID     = errors.New("invalid job ID")
	ErrJobAlreadyExists = errors.New("job already exists")
	ErrInvalidRetry     = errors.New("invalid retry policy")
)

// JobQueue defines the interface for a distributed job queue
type JobQueue interface {
	// Enqueue adds a job to the queue
	Enqueue(ctx context.Context, job Job) error

	// EnqueueWithDelay adds a job to the queue with a specified delay
	EnqueueWithDelay(ctx context.Context, job Job, delay time.Duration) error

	// Process starts processing jobs from the queue using the provided handler
	Process(ctx context.Context, handler JobHandler) error

	// Cancel cancels a job by ID
	Cancel(ctx context.Context, jobID string) error

	// Get retrieves job information by ID
	Get(ctx context.Context, jobID string) (*JobInfo, error)

	// List retrieves jobs with optional filters
	List(ctx context.Context, filter JobFilter) ([]JobInfo, error)

	// GetDeadLetteredJobs retrieves jobs that have been moved to the dead letter queue
	GetDeadLetteredJobs(ctx context.Context, limit int) ([]JobInfo, error)

	// RequeueDeadLetteredJob moves a job from the dead letter queue back to the main queue
	RequeueDeadLetteredJob(ctx context.Context, jobID string) error

	// PurgeDeadLetteredJob permanently removes a job from the dead letter queue
	PurgeDeadLetteredJob(ctx context.Context, jobID string) error

	// Close stops the job queue and releases resources
	Close() error
}

// JobHandler is a function that processes a job
type JobHandler func(ctx context.Context, job Job) error

// JobFilter defines criteria for filtering jobs
type JobFilter struct {
	Status    []JobStatus
	StartTime *time.Time
	EndTime   *time.Time
	Limit     int
	Offset    int
}

// RetryPolicy and DefaultRetryPolicy are defined in retry_policy.go

// JobQueueConfig contains configuration for job queues
type JobQueueConfig struct {
	// Name is a unique identifier for this job queue
	Name string `yaml:"name" json:"name"`

	// Type specifies the job queue implementation ("memory", "redis", "postgres")
	Type string `yaml:"type" json:"type" validate:"required,oneof=memory redis postgres"`

	// WorkerCount is the number of concurrent workers
	WorkerCount int `yaml:"workerCount" json:"workerCount" validate:"min=1"`

	// PollInterval is how often to check for new jobs
	PollInterval time.Duration `yaml:"pollInterval" json:"pollInterval"`

	// DefaultTimeout is the default job execution timeout
	DefaultTimeout time.Duration `yaml:"defaultTimeout" json:"defaultTimeout"`

	// RetryPolicy defines the default retry behavior
	RetryPolicy RetryPolicy `yaml:"retryPolicy" json:"retryPolicy"`

	// DeadLetterEnabled determines if failed jobs should be moved to dead letter queue
	DeadLetterEnabled bool `yaml:"deadLetterEnabled" json:"deadLetterEnabled"`

	// Redis contains Redis-specific configuration
	Redis struct {
		// Address is the Redis server address
		Address string `yaml:"address" json:"address"`

		// Password is the Redis server password
		Password string `yaml:"password" json:"password"`

		// Database is the Redis database number
		Database int `yaml:"database" json:"database"`

		// Namespace is the key prefix for Redis keys
		Namespace string `yaml:"namespace" json:"namespace"`
	} `yaml:"redis" json:"redis"`

	// Postgres contains PostgreSQL-specific configuration
	Postgres struct {
		// TableName is the name of the jobs table
		TableName string `yaml:"tableName" json:"tableName"`
	} `yaml:"postgres" json:"postgres"`
}

// DefaultJobQueueConfig returns a default configuration for job queues
func DefaultJobQueueConfig() JobQueueConfig {
	config := JobQueueConfig{
		Name:              "default",
		Type:              "memory",
		WorkerCount:       5,
		PollInterval:      5 * time.Second,
		DefaultTimeout:    5 * time.Minute,
		RetryPolicy:       DefaultRetryPolicy(),
		DeadLetterEnabled: true,
	}

	// Set default Redis configuration
	config.Redis.Namespace = "microlib:jobs"
	config.Redis.Database = 0

	// Set default PostgreSQL configuration
	config.Postgres.TableName = "job_queue"

	return config
}

// MemoryJobQueue implements JobQueue using in-memory storage
// This is primarily for testing and development
type MemoryJobQueue struct {
	jobs         map[string]*queueJobEntry
	queue        []*queueJobEntry
	processing   map[string]*queueJobEntry
	deadLettered map[string]*queueJobEntry // Dead letter queue for failed jobs
	mutex        sync.RWMutex
	workers      int
	wg           sync.WaitGroup
	closed       bool
	closeCh      chan struct{}
	metrics      *JobQueueMetrics
	config       JobQueueConfig
}

type queueJobEntry struct {
	Job        Job
	Status     JobStatus
	EnqueuedAt time.Time
	StartedAt  *time.Time
	FinishedAt *time.Time
	Error      error
	RetryCount int
	NextRetry  *time.Time
}

// NewMemoryJobQueue creates a new in-memory job queue
func NewMemoryJobQueue(config JobQueueConfig, metrics observability.Metrics) *MemoryJobQueue {
	if config.WorkerCount <= 0 {
		config.WorkerCount = 1
	}

	queueName := "memory"
	if config.Name != "" {
		queueName = config.Name
	}

	return &MemoryJobQueue{
		jobs:         make(map[string]*queueJobEntry),
		queue:        make([]*queueJobEntry, 0),
		processing:   make(map[string]*queueJobEntry),
		deadLettered: make(map[string]*queueJobEntry),
		workers:      config.WorkerCount,
		closeCh:      make(chan struct{}),
		metrics:      newJobQueueMetrics(metrics, queueName),
		config:       config,
	}
}

// Enqueue adds a job to the queue
func (q *MemoryJobQueue) Enqueue(ctx context.Context, job Job) error {
	return q.EnqueueWithDelay(ctx, job, 0)
}

// EnqueueWithDelay adds a job to the queue with a specified delay
func (q *MemoryJobQueue) EnqueueWithDelay(ctx context.Context, job Job, delay time.Duration) error {
	if job == nil || job.ID() == "" {
		return ErrInvalidJobID
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	if q.closed {
		return ErrQueueClosed
	}

	// Check if job already exists
	if _, exists := q.jobs[job.ID()]; exists {
		return ErrJobAlreadyExists
	}

	// Create job entry
	now := time.Now()
	entry := &queueJobEntry{
		Job:        job,
		Status:     JobStatusPending,
		EnqueuedAt: now,
	}

	// If delay is specified, set the NextRetry time
	if delay > 0 {
		nextRun := now.Add(delay)
		entry.NextRetry = &nextRun
	}

	// Add job to queue and map
	q.jobs[job.ID()] = entry
	q.queue = append(q.queue, entry)

	// Record metric
	q.metrics.enqueuedJobs.Inc()

	return nil
}

// Process starts processing jobs from the queue
func (q *MemoryJobQueue) Process(ctx context.Context, handler JobHandler) error {
	if handler == nil {
		return errors.New("handler cannot be nil")
	}

	q.mutex.Lock()
	if q.closed {
		q.mutex.Unlock()
		return ErrQueueClosed
	}
	q.mutex.Unlock()

	// Start worker pool
	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx, handler)
	}

	return nil
}

// worker processes jobs from the queue
func (q *MemoryJobQueue) worker(ctx context.Context, handler JobHandler) {
	defer q.wg.Done()

	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.closeCh:
			return
		case <-ticker.C:
			// Try to get a job from the queue
			entry := q.dequeueJob()
			if entry == nil {
				continue
			}

			// Calculate queue time and record metric
			now := time.Now()
			queueTime := now.Sub(entry.EnqueuedAt).Seconds()
			q.metrics.jobQueueTime.Observe(queueTime)

			// Mark job as running
			entry.Status = JobStatusRunning
			entry.StartedAt = &now
			q.metrics.startedJobs.Inc()

			// Process the job with timeout
			jobCtx := ctx
			var cancel context.CancelFunc

			// Use job-specific timeout if available, otherwise use default
			timeout := entry.Job.Metadata().Timeout
			if timeout <= 0 {
				timeout = q.config.DefaultTimeout
			}

			if timeout > 0 {
				jobCtx, cancel = context.WithTimeout(ctx, timeout)
				defer cancel()
			}

			// Execute the job and measure duration
			startTime := time.Now()
			err := handler(jobCtx, entry.Job)
			duration := time.Since(startTime).Seconds()
			q.metrics.jobDuration.Observe(duration)

			// Update job status
			q.mutex.Lock()
			finishTime := time.Now()
			entry.FinishedAt = &finishTime

			if err != nil {
				entry.Error = err
				entry.Status = JobStatusFailed
				q.metrics.failedJobs.Inc()

				// Check if error was due to context timeout
				if jobCtx.Err() == context.DeadlineExceeded {
					// Handle timeout specially if needed
					// For now, we'll treat it like any other error
				}

				// Handle retries if needed
				retryPolicy := entry.Job.Metadata().RetryPolicy
				if ShouldRetry(retryPolicy, entry.RetryCount+1) {
					entry.RetryCount++
					q.metrics.retriedJobs.Inc()

					backoff := CalculateBackoff(retryPolicy, entry.RetryCount)
					nextRetry := time.Now().Add(backoff)
					entry.NextRetry = &nextRetry
					entry.Status = JobStatusPending

					// Re-queue the job
					q.queue = append(q.queue, entry)
				} else if q.config.DeadLetterEnabled {
					// Move to dead letter queue if max retries exceeded
					entry.Status = JobStatusDeadLettered
					q.deadLettered[entry.Job.ID()] = entry
					q.metrics.deadLetteredJobs.Inc()
					q.metrics.jobRetryCount.Observe(float64(entry.RetryCount))
				}
			} else {
				entry.Status = JobStatusCompleted
				q.metrics.completedJobs.Inc()
				q.metrics.jobRetryCount.Observe(float64(entry.RetryCount))
			}

			delete(q.processing, entry.Job.ID())
			q.mutex.Unlock()
		}
	}
}

// dequeueJob gets the next job from the queue
func (q *MemoryJobQueue) dequeueJob() *queueJobEntry {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if len(q.queue) == 0 {
		return nil
	}

	// Get the first job
	entry := q.queue[0]
	q.queue = q.queue[1:]

	// Check if job should be processed now
	if entry.NextRetry != nil && time.Now().Before(*entry.NextRetry) {
		// Re-queue the job for later
		q.queue = append(q.queue, entry)
		return nil
	}

	// Mark as processing
	q.processing[entry.Job.ID()] = entry

	return entry
}

// Cancel cancels a job by ID
func (q *MemoryJobQueue) Cancel(ctx context.Context, jobID string) error {
	if jobID == "" {
		return ErrInvalidJobID
	}

	q.mutex.Lock()
	defer q.mutex.Unlock()

	entry, exists := q.jobs[jobID]
	if !exists {
		return ErrJobNotFound
	}

	// Can only cancel pending jobs
	if entry.Status != JobStatusPending {
		return errors.New("cannot cancel job that is not pending")
	}

	// Remove from queue
	for i, queuedEntry := range q.queue {
		if queuedEntry.Job.ID() == jobID {
			q.queue = append(q.queue[:i], q.queue[i+1:]...)
			break
		}
	}

	// Mark as cancelled
	now := time.Now()
	entry.Status = JobStatusCancelled
	entry.FinishedAt = &now

	// Record metric
	q.metrics.cancelledJobs.Inc()

	return nil
}

// Get retrieves job information by ID
func (q *MemoryJobQueue) Get(ctx context.Context, jobID string) (*JobInfo, error) {
	if jobID == "" {
		return nil, ErrInvalidJobID
	}

	q.mutex.RLock()
	defer q.mutex.RUnlock()

	entry, exists := q.jobs[jobID]
	if !exists {
		return nil, ErrJobNotFound
	}

	return &JobInfo{
		ID:         entry.Job.ID(),
		Name:       entry.Job.Name(),
		Status:     entry.Status,
		EnqueuedAt: entry.EnqueuedAt,
		StartedAt:  entry.StartedAt,
		FinishedAt: entry.FinishedAt,
		Error:      entry.Error,
		RetryCount: entry.RetryCount,
		NextRetry:  entry.NextRetry,
		Metadata:   entry.Job.Metadata(),
	}, nil
}

// List retrieves jobs with optional filters
func (q *MemoryJobQueue) List(ctx context.Context, filter JobFilter) ([]JobInfo, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	result := make([]JobInfo, 0)

	// Apply filters
	for _, entry := range q.jobs {
		// Filter by status if specified
		if len(filter.Status) > 0 {
			statusMatch := false
			for _, status := range filter.Status {
				if entry.Status == status {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				continue
			}
		}

		// Filter by start time if specified
		if filter.StartTime != nil && (entry.EnqueuedAt.Before(*filter.StartTime)) {
			continue
		}

		// Filter by end time if specified
		if filter.EndTime != nil && (entry.EnqueuedAt.After(*filter.EndTime)) {
			continue
		}

		// Add to result
		result = append(result, JobInfo{
			ID:         entry.Job.ID(),
			Name:       entry.Job.Name(),
			Status:     entry.Status,
			EnqueuedAt: entry.EnqueuedAt,
			StartedAt:  entry.StartedAt,
			FinishedAt: entry.FinishedAt,
			Error:      entry.Error,
			RetryCount: entry.RetryCount,
			NextRetry:  entry.NextRetry,
			Metadata:   entry.Job.Metadata(),
		})
	}

	// Apply limit and offset
	if filter.Offset > 0 && filter.Offset < len(result) {
		result = result[filter.Offset:]
	}

	if filter.Limit > 0 && filter.Limit < len(result) {
		result = result[:filter.Limit]
	}

	return result, nil
}

// Close stops the job queue and releases resources
func (q *MemoryJobQueue) Close() error {
	q.mutex.Lock()
	if q.closed {
		q.mutex.Unlock()
		return nil
	}
	q.closed = true
	close(q.closeCh)
	q.mutex.Unlock()

	// Wait for all workers to finish
	q.wg.Wait()
	return nil
}

// calculateBackoff is now defined in retry_policy.go as CalculateBackoff

// JobStatusCancelled is defined in scheduler.go

// GetDeadLetteredJobs retrieves jobs that have been moved to the dead letter queue
func (q *MemoryJobQueue) GetDeadLetteredJobs(ctx context.Context, limit int) ([]JobInfo, error) {
	q.mutex.RLock()
	defer q.mutex.RUnlock()

	result := make([]JobInfo, 0, len(q.deadLettered))
	count := 0

	for _, entry := range q.deadLettered {
		if limit > 0 && count >= limit {
			break
		}

		result = append(result, JobInfo{
			ID:         entry.Job.ID(),
			Name:       entry.Job.Name(),
			Status:     entry.Status,
			EnqueuedAt: entry.EnqueuedAt,
			StartedAt:  entry.StartedAt,
			FinishedAt: entry.FinishedAt,
			Error:      entry.Error,
			RetryCount: entry.RetryCount,
			NextRetry:  entry.NextRetry,
			Metadata:   entry.Job.Metadata(),
		})
		count++
	}

	return result, nil
}

// RequeueDeadLetteredJob moves a job from the dead letter queue back to the main queue
func (q *MemoryJobQueue) RequeueDeadLetteredJob(ctx context.Context, jobID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	entry, exists := q.deadLettered[jobID]
	if !exists {
		return ErrJobNotFound
	}

	// Reset job status
	entry.Status = JobStatusPending
	entry.RetryCount = 0
	entry.Error = nil
	entry.NextRetry = nil
	entry.StartedAt = nil
	entry.FinishedAt = nil
	entry.EnqueuedAt = time.Now()

	// Move from dead letter queue back to main queue
	delete(q.deadLettered, jobID)
	q.queue = append(q.queue, entry)

	return nil
}

// PurgeDeadLetteredJob permanently removes a job from the dead letter queue
func (q *MemoryJobQueue) PurgeDeadLetteredJob(ctx context.Context, jobID string) error {
	q.mutex.Lock()
	defer q.mutex.Unlock()

	if _, exists := q.deadLettered[jobID]; !exists {
		return ErrJobNotFound
	}

	delete(q.deadLettered, jobID)
	delete(q.jobs, jobID)

	return nil
}
