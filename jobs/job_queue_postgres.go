package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PostgresJobQueue implements JobQueue using PostgreSQL
type PostgresJobQueue struct {
	pool      *pgxpool.Pool
	config    JobQueueConfig
	tableName string
	workers   int
	wg        sync.WaitGroup
	closed    bool
	closeCh   chan struct{}
	mutex     sync.RWMutex
}

// NewPostgresJobQueue creates a new PostgreSQL-based job queue
func NewPostgresJobQueue(ctx context.Context, config JobQueueConfig, connString string) (*PostgresJobQueue, error) {
	if config.Type != "postgres" {
		return nil, errors.New("job queue: config type must be 'postgres'")
	}

	// Set default values if not provided
	if config.WorkerCount <= 0 {
		config.WorkerCount = 5
	}

	if config.PollInterval <= 0 {
		config.PollInterval = 5 * time.Second
	}

	if config.Postgres.TableName == "" {
		config.Postgres.TableName = "job_queue"
	}

	// Create connection pool
	poolConfig, err := pgxpool.ParseConfig(connString)
	if err != nil {
		return nil, fmt.Errorf("failed to parse postgres config: %w", err)
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create postgres connection pool: %w", err)
	}

	// Create job queue
	queue := &PostgresJobQueue{
		pool:      pool,
		config:    config,
		tableName: config.Postgres.TableName,
		workers:   config.WorkerCount,
		closeCh:   make(chan struct{}),
	}

	// Ensure table exists
	err = queue.ensureTableExists(ctx)
	if err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to create job queue table: %w", err)
	}

	return queue, nil
}

// ensureTableExists creates the job queue table if it doesn't exist
func (q *PostgresJobQueue) ensureTableExists(ctx context.Context) error {
	createTableSQL := fmt.Sprintf(`
		CREATE TABLE IF NOT EXISTS %s (
			job_id TEXT PRIMARY KEY,
			job_name TEXT NOT NULL,
			job_data BYTEA NOT NULL,
			status TEXT NOT NULL,
			enqueued_at TIMESTAMP WITH TIME ZONE NOT NULL,
			started_at TIMESTAMP WITH TIME ZONE,
			finished_at TIMESTAMP WITH TIME ZONE,
			error TEXT,
			retry_count INTEGER NOT NULL DEFAULT 0,
			next_retry TIMESTAMP WITH TIME ZONE,
			metadata JSONB NOT NULL,
			created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
			updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW()
		);
		
		CREATE INDEX IF NOT EXISTS %s_status_idx ON %s (status);
		CREATE INDEX IF NOT EXISTS %s_next_retry_idx ON %s (next_retry) WHERE status = 'pending';
	`, q.tableName, q.tableName, q.tableName, q.tableName, q.tableName)

	_, err := q.pool.Exec(ctx, createTableSQL)
	return err
}

// Enqueue adds a job to the queue
func (q *PostgresJobQueue) Enqueue(ctx context.Context, job Job) error {
	return q.EnqueueWithDelay(ctx, job, 0)
}

// EnqueueWithDelay adds a job to the queue with a specified delay
func (q *PostgresJobQueue) EnqueueWithDelay(ctx context.Context, job Job, delay time.Duration) error {
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
	var exists bool
	err := q.pool.QueryRow(ctx, fmt.Sprintf("SELECT EXISTS(SELECT 1 FROM %s WHERE job_id = $1)", q.tableName), job.ID()).Scan(&exists)
	if err != nil {
		return fmt.Errorf("failed to check if job exists: %w", err)
	}
	if exists {
		return ErrJobAlreadyExists
	}

	// Serialize job data
	jobData, err := json.Marshal(job)
	if err != nil {
		return fmt.Errorf("failed to serialize job: %w", err)
	}

	// Serialize metadata
	metadataJSON, err := json.Marshal(job.Metadata())
	if err != nil {
		return fmt.Errorf("failed to serialize job metadata: %w", err)
	}

	// Calculate next retry time if delay is specified
	var nextRetry *time.Time
	if delay > 0 {
		t := time.Now().Add(delay)
		nextRetry = &t
	}

	// Insert job into database
	_, err = q.pool.Exec(ctx,
		fmt.Sprintf(`
			INSERT INTO %s (
				job_id, job_name, job_data, status, enqueued_at, next_retry, metadata
			) VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, q.tableName),
		job.ID(), job.Name(), jobData, JobStatusPending, time.Now(), nextRetry, metadataJSON,
	)

	if err != nil {
		return fmt.Errorf("failed to enqueue job: %w", err)
	}

	return nil
}

// Process starts processing jobs from the queue
func (q *PostgresJobQueue) Process(ctx context.Context, handler JobHandler) error {
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
func (q *PostgresJobQueue) worker(ctx context.Context, handler JobHandler) {
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
func (q *PostgresJobQueue) dequeueJob(ctx context.Context) (string, error) {
	// Use advisory lock to prevent other workers from processing the same job
	// This is a simplified implementation - a real one would use FOR UPDATE SKIP LOCKED
	var jobID string
	err := q.pool.QueryRow(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = $1, started_at = $2, updated_at = $3
		WHERE job_id = (
			SELECT job_id
			FROM %s
			WHERE status = $4
			AND (next_retry IS NULL OR next_retry <= $5)
			ORDER BY enqueued_at
			LIMIT 1
			FOR UPDATE SKIP LOCKED
		)
		RETURNING job_id
	`, q.tableName, q.tableName),
		JobStatusRunning, time.Now(), time.Now(), JobStatusPending, time.Now(),
	).Scan(&jobID)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return "", nil
		}
		return "", err
	}

	return jobID, nil
}

// processJob handles the execution of a job
func (q *PostgresJobQueue) processJob(ctx context.Context, jobID string, handler JobHandler) {
	// Get job data
	var jobData []byte
	var jobName string
	var metadataJSON []byte
	err := q.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT job_data, job_name, metadata
		FROM %s
		WHERE job_id = $1
	`, q.tableName), jobID).Scan(&jobData, &jobName, &metadataJSON)

	if err != nil {
		// Log error and return
		fmt.Printf("Failed to get job data: %v\n", err)
		return
	}

	// Parse metadata
	var metadata JobMetadata
	if err := json.Unmarshal(metadataJSON, &metadata); err != nil {
		q.markJobFailed(ctx, jobID, fmt.Errorf("failed to parse job metadata: %w", err))
		return
	}

	// Create job
	job := &serializedJob{
		id:       jobID,
		name:     jobName,
		data:     jobData,
		metadata: metadata,
	}

	// Create job context with timeout
	jobCtx := ctx
	if timeout := metadata.Timeout; timeout > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Execute job
	err = handler(jobCtx, job)

	// Update job status based on result
	if err != nil {
		q.handleJobFailure(ctx, jobID, err)
	} else {
		q.markJobCompleted(ctx, jobID)
	}
}

// handleJobFailure processes a failed job and handles retries
func (q *PostgresJobQueue) handleJobFailure(ctx context.Context, jobID string, err error) {
	// Get current retry count and metadata
	var retryCount int
	var metadataJSON []byte
	queryErr := q.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT retry_count, metadata
		FROM %s
		WHERE job_id = $1
	`, q.tableName), jobID).Scan(&retryCount, &metadataJSON)

	if queryErr != nil {
		// If we can't get the retry count, just mark as failed
		q.markJobFailed(ctx, jobID, err)
		return
	}

	// Parse metadata
	var metadata JobMetadata
	if json.Unmarshal(metadataJSON, &metadata) != nil {
		// If we can't parse metadata, just mark as failed
		q.markJobFailed(ctx, jobID, err)
		return
	}

	// Increment retry count
	retryCount++

	// Check if we should retry
	retryPolicy := metadata.RetryPolicy
	if retryPolicy.MaxRetries == 0 {
		retryPolicy = q.config.RetryPolicy
	}

	if retryCount <= retryPolicy.MaxRetries {
		// Calculate next retry time
		backoff := calculateBackoff(retryPolicy, retryCount)
		nextRetry := time.Now().Add(backoff)

		// Update job for retry
		_, updateErr := q.pool.Exec(ctx, fmt.Sprintf(`
			UPDATE %s
			SET status = $1, error = $2, retry_count = $3, next_retry = $4, updated_at = $5
			WHERE job_id = $6
		`, q.tableName),
			JobStatusPending, err.Error(), retryCount, nextRetry, time.Now(), jobID,
		)

		if updateErr != nil {
			// If update fails, mark as failed
			q.markJobFailed(ctx, jobID, err)
		}
	} else {
		// Mark as permanently failed
		q.markJobFailed(ctx, jobID, err)
	}
}

// markJobCompleted marks a job as completed
func (q *PostgresJobQueue) markJobCompleted(ctx context.Context, jobID string) {
	_, err := q.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = $1, finished_at = $2, updated_at = $3
		WHERE job_id = $4
	`, q.tableName),
		JobStatusCompleted, time.Now(), time.Now(), jobID,
	)

	if err != nil {
		// Log error
		fmt.Printf("Failed to mark job as completed: %v\n", err)
	}
}

// markJobFailed marks a job as failed
func (q *PostgresJobQueue) markJobFailed(ctx context.Context, jobID string, err error) {
	_, updateErr := q.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = $1, error = $2, finished_at = $3, updated_at = $4
		WHERE job_id = $5
	`, q.tableName),
		JobStatusFailed, err.Error(), time.Now(), time.Now(), jobID,
	)

	if updateErr != nil {
		// Log error
		fmt.Printf("Failed to mark job as failed: %v\n", updateErr)
	}
}

// Cancel cancels a job by ID
func (q *PostgresJobQueue) Cancel(ctx context.Context, jobID string) error {
	if jobID == "" {
		return ErrInvalidJobID
	}

	// Check if job exists and is pending
	var status JobStatus
	err := q.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT status
		FROM %s
		WHERE job_id = $1
	`, q.tableName), jobID).Scan(&status)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrJobNotFound
		}
		return fmt.Errorf("failed to check job status: %w", err)
	}

	// Can only cancel pending jobs
	if status != JobStatusPending {
		return errors.New("cannot cancel job that is not pending")
	}

	// Update job status
	_, err = q.pool.Exec(ctx, fmt.Sprintf(`
		UPDATE %s
		SET status = $1, finished_at = $2, updated_at = $3
		WHERE job_id = $4
	`, q.tableName),
		JobStatusCancelled, time.Now(), time.Now(), jobID,
	)

	if err != nil {
		return fmt.Errorf("failed to cancel job: %w", err)
	}

	return nil
}

// Get retrieves job information by ID
func (q *PostgresJobQueue) Get(ctx context.Context, jobID string) (*JobInfo, error) {
	if jobID == "" {
		return nil, ErrInvalidJobID
	}

	// Query job data
	var (
		name       string
		status     JobStatus
		enqueuedAt time.Time
		startedAt  *time.Time
		finishedAt *time.Time
		errorMsg   *string
		retryCount int
		nextRetry  *time.Time
		metadata   []byte
	)

	err := q.pool.QueryRow(ctx, fmt.Sprintf(`
		SELECT job_name, status, enqueued_at, started_at, finished_at, error, retry_count, next_retry, metadata
		FROM %s
		WHERE job_id = $1
	`, q.tableName), jobID).Scan(
		&name, &status, &enqueuedAt, &startedAt, &finishedAt, &errorMsg, &retryCount, &nextRetry, &metadata,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrJobNotFound
		}
		return nil, fmt.Errorf("failed to get job: %w", err)
	}

	// Parse metadata
	var jobMetadata JobMetadata
	if err := json.Unmarshal(metadata, &jobMetadata); err != nil {
		return nil, fmt.Errorf("failed to parse job metadata: %w", err)
	}

	// Create job info
	jobInfo := &JobInfo{
		ID:         jobID,
		Name:       name,
		Status:     status,
		EnqueuedAt: enqueuedAt,
		StartedAt:  startedAt,
		FinishedAt: finishedAt,
		RetryCount: retryCount,
		NextRetry:  nextRetry,
		Metadata:   jobMetadata,
	}

	// Set error if present
	if errorMsg != nil {
		jobInfo.Error = errors.New(*errorMsg)
	}

	return jobInfo, nil
}

// List retrieves jobs with optional filters
func (q *PostgresJobQueue) List(ctx context.Context, filter JobFilter) ([]JobInfo, error) {
	// Build query
	query := fmt.Sprintf(`
		SELECT job_id, job_name, status, enqueued_at, started_at, finished_at, error, retry_count, next_retry, metadata
		FROM %s
		WHERE 1=1
	`, q.tableName)

	args := make([]interface{}, 0)
	argIndex := 1

	// Apply status filter
	if len(filter.Status) > 0 {
		query += fmt.Sprintf(" AND status = ANY($%d)", argIndex)
		statusArray := make([]string, len(filter.Status))
		for i, s := range filter.Status {
			statusArray[i] = string(s)
		}
		args = append(args, statusArray)
		argIndex++
	}

	// Apply time filters
	if filter.StartTime != nil {
		query += fmt.Sprintf(" AND enqueued_at >= $%d", argIndex)
		args = append(args, filter.StartTime)
		argIndex++
	}

	if filter.EndTime != nil {
		query += fmt.Sprintf(" AND enqueued_at <= $%d", argIndex)
		args = append(args, filter.EndTime)
		argIndex++
	}

	// Apply ordering
	query += " ORDER BY enqueued_at DESC"

	// Apply limit and offset
	if filter.Limit > 0 {
		query += fmt.Sprintf(" LIMIT $%d", argIndex)
		args = append(args, filter.Limit)
		argIndex++
	}

	if filter.Offset > 0 {
		query += fmt.Sprintf(" OFFSET $%d", argIndex)
		args = append(args, filter.Offset)
	}

	// Execute query
	rows, err := q.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	// Process results
	result := make([]JobInfo, 0)
	for rows.Next() {
		var (
			jobID      string
			name       string
			status     JobStatus
			enqueuedAt time.Time
			startedAt  *time.Time
			finishedAt *time.Time
			errorMsg   *string
			retryCount int
			nextRetry  *time.Time
			metadata   []byte
		)

		err := rows.Scan(
			&jobID, &name, &status, &enqueuedAt, &startedAt, &finishedAt,
			&errorMsg, &retryCount, &nextRetry, &metadata,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job row: %w", err)
		}

		// Parse metadata
		var jobMetadata JobMetadata
		if err := json.Unmarshal(metadata, &jobMetadata); err != nil {
			// Skip jobs with invalid metadata
			continue
		}

		// Create job info
		jobInfo := JobInfo{
			ID:         jobID,
			Name:       name,
			Status:     status,
			EnqueuedAt: enqueuedAt,
			StartedAt:  startedAt,
			FinishedAt: finishedAt,
			RetryCount: retryCount,
			NextRetry:  nextRetry,
			Metadata:   jobMetadata,
		}

		// Set error if present
		if errorMsg != nil {
			jobInfo.Error = errors.New(*errorMsg)
		}

		result = append(result, jobInfo)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating job rows: %w", err)
	}

	return result, nil
}

// Close stops the job queue and releases resources
func (q *PostgresJobQueue) Close() error {
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

	// Close database connection
	q.pool.Close()
	return nil
}
