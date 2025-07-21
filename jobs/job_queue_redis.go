package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisJobQueue implements JobQueue using Redis
type RedisJobQueue struct {
	client    *redis.Client
	config    JobQueueConfig
	namespace string
	workers   int
	wg        sync.WaitGroup
	closed    bool
	closeCh   chan struct{}
	mutex     sync.RWMutex
}

// RedisJobEntry represents a job stored in Redis
type RedisJobEntry struct {
	JobID      string      `json:"job_id"`
	JobName    string      `json:"job_name"`
	JobData    []byte      `json:"job_data"`
	Status     JobStatus   `json:"status"`
	EnqueuedAt time.Time   `json:"enqueued_at"`
	StartedAt  *time.Time  `json:"started_at,omitempty"`
	FinishedAt *time.Time  `json:"finished_at,omitempty"`
	Error      string      `json:"error,omitempty"`
	RetryCount int         `json:"retry_count"`
	NextRetry  *time.Time  `json:"next_retry,omitempty"`
	Metadata   JobMetadata `json:"metadata"`
}

// NewRedisJobQueue creates a new Redis-based job queue
func NewRedisJobQueue(config JobQueueConfig) (*RedisJobQueue, error) {
	if config.Type != "redis" {
		return nil, errors.New("job queue: config type must be 'redis'")
	}

	if config.Redis.Address == "" {
		return nil, errors.New("job queue: Redis address is required")
	}

	// Set default values if not provided
	if config.WorkerCount <= 0 {
		config.WorkerCount = 5
	}

	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}

	if config.Redis.Namespace == "" {
		config.Redis.Namespace = "microlib:jobs"
	}

	// Create Redis client
	client := redis.NewClient(&redis.Options{
		Addr:     config.Redis.Address,
		Password: config.Redis.Password,
		DB:       config.Redis.Database,
	})

	// Create job queue
	queue := &RedisJobQueue{
		client:    client,
		config:    config,
		namespace: config.Redis.Namespace,
		workers:   config.WorkerCount,
		closeCh:   make(chan struct{}),
	}

	return queue, nil
}

// Enqueue adds a job to the queue
func (q *RedisJobQueue) Enqueue(ctx context.Context, job Job) error {
	return q.EnqueueWithDelay(ctx, job, 0)
}

// EnqueueWithDelay adds a job to the queue with a specified delay
func (q *RedisJobQueue) EnqueueWithDelay(ctx context.Context, job Job, delay time.Duration) error {
	if job == nil || job.ID() == "" {
		return ErrInvalidJobID
	}

	q.mutex.RLock()
	if q.closed {
		q.mutex.RUnlock()
		return ErrQueueClosed
	}
	q.mutex.RUnlock()

	// Check if job already exists
	exists, err := q.client.Exists(ctx, q.jobKey(job.ID())).Result()
	if err != nil {
		return fmt.Errorf("failed to check if job exists: %w", err)
	}
	if exists > 0 {
		return ErrJobAlreadyExists
	}

	// Serialize job data
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	// Create job entry
	entry := RedisJobEntry{
		JobID:      job.ID(),
		JobName:    job.Name(),
		JobData:    jobData,
		Status:     JobStatusPending,
		EnqueuedAt: time.Now(),
		Metadata:   job.Metadata(),
	}

	// Serialize entry
	entryData, err := json.Marshal(entry)
	if err != nil {
		return fmt.Errorf("failed to serialize job entry: %w", err)
	}

	// Store job in Redis
	pipe := q.client.Pipeline()
	pipe.Set(ctx, q.jobKey(job.ID()), entryData, 0)

	// Add to pending queue with score based on delay
	score := float64(time.Now().Add(delay).UnixNano())
	pipe.ZAdd(ctx, q.pendingQueueKey(), redis.Z{
		Score:  score,
		Member: job.ID(),
	})

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// Process starts processing jobs from the queue
func (q *RedisJobQueue) Process(ctx context.Context, handler JobHandler) error {
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
func (q *RedisJobQueue) worker(ctx context.Context, handler JobHandler) {
	defer q.wg.Done()

	ticker := time.NewTicker(q.config.PollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-q.closeCh:
			return
		case <-ticker.C:
			// Try to get a job from the queue
			jobID, err := q.dequeueJob(ctx)
			if err != nil || jobID == "" {
				continue
			}

			// Process the job
			q.processJob(ctx, jobID, handler)
		}
	}
}

// dequeueJob gets the next job from the queue
func (q *RedisJobQueue) dequeueJob(ctx context.Context) (string, error) {
	now := time.Now().UnixNano()

	// Get jobs that are ready to be processed
	result, err := q.client.ZRangeByScore(ctx, q.pendingQueueKey(), &redis.ZRangeBy{
		Min:    "0",
		Max:    fmt.Sprintf("%d", now),
		Offset: 0,
		Count:  1,
	}).Result()

	if err != nil {
		return "", err
	}

	if len(result) == 0 {
		return "", nil
	}

	jobID := result[0]

	// Move from pending to processing queue
	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, q.pendingQueueKey(), jobID)
	pipe.ZAdd(ctx, q.processingQueueKey(), redis.Z{
		Score:  float64(now),
		Member: jobID,
	})

	// Update job status
	q.updateJobStatus(pipe, ctx, jobID, JobStatusRunning)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return "", err
	}

	return jobID, nil
}

// processJob handles the execution of a job
func (q *RedisJobQueue) processJob(ctx context.Context, jobID string, handler JobHandler) {
	// Get job data
	jobEntry, err := q.getJobEntry(ctx, jobID)
	if err != nil {
		// Log error and return
		fmt.Printf("Failed to get job entry: %v\n", err)
		return
	}

	// Deserialize job
	job, err := q.deserializeJob(jobEntry)
	if err != nil {
		// Log error and mark job as failed
		q.markJobFailed(ctx, jobID, fmt.Errorf("failed to deserialize job: %w", err))
		return
	}

	// Create job context with timeout
	jobCtx := ctx
	if timeout := jobEntry.Metadata.Timeout; timeout > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Execute job
	err = handler(jobCtx, job)

	// Update job status based on result
	if err != nil {
		q.handleJobFailure(ctx, jobID, jobEntry, err)
	} else {
		q.markJobCompleted(ctx, jobID)
	}
}

// handleJobFailure processes a failed job and handles retries
func (q *RedisJobQueue) handleJobFailure(ctx context.Context, jobID string, jobEntry *RedisJobEntry, err error) {
	// Increment retry count
	jobEntry.RetryCount++

	// Check if we should retry
	retryPolicy := jobEntry.Metadata.RetryPolicy
	if retryPolicy.MaxRetries == 0 {
		retryPolicy = q.config.RetryPolicy
	}

	if jobEntry.RetryCount <= retryPolicy.MaxRetries {
		// Calculate next retry time
		backoff := CalculateBackoff(retryPolicy, jobEntry.RetryCount)
		nextRetry := time.Now().Add(backoff)
		jobEntry.NextRetry = &nextRetry

		// Update job entry
		pipe := q.client.Pipeline()

		// Remove from processing queue
		pipe.ZRem(ctx, q.processingQueueKey(), jobID)

		// Add back to pending queue with new score
		pipe.ZAdd(ctx, q.pendingQueueKey(), redis.Z{
			Score:  float64(nextRetry.UnixNano()),
			Member: jobID,
		})

		// Update job status and retry info
		jobEntry.Status = JobStatusPending
		jobEntry.Error = err.Error()
		entryData, _ := json.Marshal(jobEntry)
		pipe.Set(ctx, q.jobKey(jobID), entryData, 0)

		pipe.Exec(ctx)
	} else {
		// Mark as permanently failed
		q.markJobFailed(ctx, jobID, err)
	}
}

// markJobCompleted marks a job as completed
func (q *RedisJobQueue) markJobCompleted(ctx context.Context, jobID string) {
	pipe := q.client.Pipeline()

	// Remove from processing queue
	pipe.ZRem(ctx, q.processingQueueKey(), jobID)

	// Update job status
	q.updateJobStatus(pipe, ctx, jobID, JobStatusCompleted)

	pipe.Exec(ctx)
}

// markJobFailed marks a job as failed
func (q *RedisJobQueue) markJobFailed(ctx context.Context, jobID string, err error) {
	pipe := q.client.Pipeline()

	// Remove from processing queue
	pipe.ZRem(ctx, q.processingQueueKey(), jobID)

	// Get job entry
	jobEntryData, getErr := q.client.Get(ctx, q.jobKey(jobID)).Bytes()
	if getErr != nil {
		// If we can't get the job, just update the status
		q.updateJobStatus(pipe, ctx, jobID, JobStatusFailed)
		pipe.Exec(ctx)
		return
	}

	// Update job entry with error
	var jobEntry RedisJobEntry
	if json.Unmarshal(jobEntryData, &jobEntry) == nil {
		jobEntry.Status = JobStatusFailed
		jobEntry.Error = err.Error()
		now := time.Now()
		jobEntry.FinishedAt = &now

		// Save updated entry
		entryData, _ := json.Marshal(jobEntry)
		pipe.Set(ctx, q.jobKey(jobID), entryData, 0)
	} else {
		// If we can't unmarshal, just update the status
		q.updateJobStatus(pipe, ctx, jobID, JobStatusFailed)
	}

	pipe.Exec(ctx)
}

// updateJobStatus updates the status of a job
func (q *RedisJobQueue) updateJobStatus(pipe redis.Pipeliner, ctx context.Context, jobID string, status JobStatus) {
	// Get job entry
	jobEntryData, err := q.client.Get(ctx, q.jobKey(jobID)).Bytes()
	if err != nil {
		return
	}

	// Update job entry
	var jobEntry RedisJobEntry
	if err := json.Unmarshal(jobEntryData, &jobEntry); err != nil {
		return
	}

	jobEntry.Status = status
	now := time.Now()

	if status == JobStatusRunning && jobEntry.StartedAt == nil {
		jobEntry.StartedAt = &now
	}

	if status == JobStatusCompleted || status == JobStatusFailed || status == JobStatusCancelled {
		jobEntry.FinishedAt = &now
	}

	// Save updated entry
	entryData, _ := json.Marshal(jobEntry)
	pipe.Set(ctx, q.jobKey(jobID), entryData, 0)
}

// Cancel cancels a job by ID
func (q *RedisJobQueue) Cancel(ctx context.Context, jobID string) error {
	if jobID == "" {
		return ErrInvalidJobID
	}

	// Check if job exists
	exists, err := q.client.Exists(ctx, q.jobKey(jobID)).Result()
	if err != nil {
		return fmt.Errorf("failed to check if job exists: %w", err)
	}
	if exists == 0 {
		return ErrJobNotFound
	}

	// Get job entry
	jobEntry, err := q.getJobEntry(ctx, jobID)
	if err != nil {
		return err
	}

	// Can only cancel pending jobs
	if jobEntry.Status != JobStatusPending {
		return errors.New("cannot cancel job that is not pending")
	}

	// Update job status
	pipe := q.client.Pipeline()
	pipe.ZRem(ctx, q.pendingQueueKey(), jobID)
	q.updateJobStatus(pipe, ctx, jobID, JobStatusCancelled)
	_, err = pipe.Exec(ctx)

	return err
}

// Get retrieves job information by ID
func (q *RedisJobQueue) Get(ctx context.Context, jobID string) (*JobInfo, error) {
	if jobID == "" {
		return nil, ErrInvalidJobID
	}

	// Get job entry
	jobEntry, err := q.getJobEntry(ctx, jobID)
	if err != nil {
		return nil, err
	}

	// Convert to JobInfo
	return &JobInfo{
		ID:         jobEntry.JobID,
		Name:       jobEntry.JobName,
		Status:     jobEntry.Status,
		EnqueuedAt: jobEntry.EnqueuedAt,
		StartedAt:  jobEntry.StartedAt,
		FinishedAt: jobEntry.FinishedAt,
		Error:      errors.New(jobEntry.Error),
		RetryCount: jobEntry.RetryCount,
		NextRetry:  jobEntry.NextRetry,
		Metadata:   jobEntry.Metadata,
	}, nil
}

// List retrieves jobs with optional filters
func (q *RedisJobQueue) List(ctx context.Context, filter JobFilter) ([]JobInfo, error) {
	// Get all job IDs
	pattern := q.jobKey("*")
	var cursor uint64
	var jobIDs []string

	for {
		var keys []string
		var err error
		keys, cursor, err = q.client.Scan(ctx, cursor, pattern, 100).Result()
		if err != nil {
			return nil, fmt.Errorf("failed to scan jobs: %w", err)
		}

		for _, key := range keys {
			// Extract job ID from key
			jobID := key[len(q.namespace)+6:] // Remove "microlib:jobs:job:" prefix
			jobIDs = append(jobIDs, jobID)
		}

		if cursor == 0 {
			break
		}
	}

	// Get job entries
	result := make([]JobInfo, 0, len(jobIDs))
	for _, jobID := range jobIDs {
		jobEntry, err := q.getJobEntry(ctx, jobID)
		if err != nil {
			continue
		}

		// Apply filters
		if len(filter.Status) > 0 {
			statusMatch := false
			for _, status := range filter.Status {
				if jobEntry.Status == status {
					statusMatch = true
					break
				}
			}
			if !statusMatch {
				continue
			}
		}

		if filter.StartTime != nil && jobEntry.EnqueuedAt.Before(*filter.StartTime) {
			continue
		}

		if filter.EndTime != nil && jobEntry.EnqueuedAt.After(*filter.EndTime) {
			continue
		}

		// Add to result
		result = append(result, JobInfo{
			ID:         jobEntry.JobID,
			Name:       jobEntry.JobName,
			Status:     jobEntry.Status,
			EnqueuedAt: jobEntry.EnqueuedAt,
			StartedAt:  jobEntry.StartedAt,
			FinishedAt: jobEntry.FinishedAt,
			Error:      errors.New(jobEntry.Error),
			RetryCount: jobEntry.RetryCount,
			NextRetry:  jobEntry.NextRetry,
			Metadata:   jobEntry.Metadata,
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
func (q *RedisJobQueue) Close() error {
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

	// Close Redis connection
	return q.client.Close()
}

// getJobEntry retrieves a job entry from Redis
func (q *RedisJobQueue) getJobEntry(ctx context.Context, jobID string) (*RedisJobEntry, error) {
	// Get job data
	jobEntryData, err := q.client.Get(ctx, q.jobKey(jobID)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get job entry: %w", err)
	}

	// Deserialize job entry
	var jobEntry RedisJobEntry
	if err := json.Unmarshal(jobEntryData, &jobEntry); err != nil {
		return nil, fmt.Errorf("failed to deserialize job entry: %w", err)
	}

	return &jobEntry, nil
}

// deserializeJob deserializes a job from a job entry
func (q *RedisJobQueue) deserializeJob(jobEntry *RedisJobEntry) (Job, error) {
	// This is a simplified implementation
	// In a real implementation, you would use reflection or a registry to create the correct job type
	return &serializedJob{
		id:       jobEntry.JobID,
		name:     jobEntry.JobName,
		data:     jobEntry.JobData,
		metadata: jobEntry.Metadata,
	}, nil
}

// jobKey returns the Redis key for a job
func (q *RedisJobQueue) jobKey(jobID string) string {
	return fmt.Sprintf("%s:job:%s", q.namespace, jobID)
}

// pendingQueueKey returns the Redis key for the pending queue
func (q *RedisJobQueue) pendingQueueKey() string {
	return fmt.Sprintf("%s:pending", q.namespace)
}

// processingQueueKey returns the Redis key for the processing queue
func (q *RedisJobQueue) processingQueueKey() string {
	return fmt.Sprintf("%s:processing", q.namespace)
}

// serializedJob is a simple implementation of Job for deserialized jobs
type serializedJob struct {
	id       string
	name     string
	data     []byte
	metadata JobMetadata
}

func (j *serializedJob) ID() string {
	return j.id
}

func (j *serializedJob) Name() string {
	return j.name
}

func (j *serializedJob) Execute(ctx context.Context) error {
	// This would be implemented by the actual job
	return errors.New("serialized job cannot be executed directly")
}

func (j *serializedJob) Metadata() JobMetadata {
	return j.metadata
}
