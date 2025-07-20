package jobs

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// CronScheduler implements the Scheduler interface using cron
type CronScheduler struct {
	cron          *cron.Cron
	jobs          map[string]jobEntry
	mutex         sync.RWMutex
	leaderElector LeaderElector
	instanceID    string
	isLeader      bool
	executions    map[string]*JobExecution
	execMutex     sync.RWMutex
}

type jobEntry struct {
	job      Job
	spec     string
	cronID   cron.EntryID
	nextRun  time.Time
	lastRun  *time.Time
	lastErr  error
	metadata JobMetadata
}

// NewCronScheduler creates a new scheduler with the provided options
func NewCronScheduler(opts ...SchedulerOption) *CronScheduler {
	cronOpts := cron.WithSeconds()

	s := &CronScheduler{
		cron:       cron.New(cronOpts),
		jobs:       make(map[string]jobEntry),
		executions: make(map[string]*JobExecution),
		instanceID: generateInstanceID(),
	}

	// Apply options
	for _, opt := range opts {
		opt(s)
	}

	// If no leader elector is provided, use a local one that always returns true
	if s.leaderElector == nil {
		s.leaderElector = &localLeaderElector{isLeader: true}
	}

	return s
}

// SchedulerOption configures a CronScheduler
type SchedulerOption func(*CronScheduler)

// WithLeaderElector configures the scheduler with a leader election mechanism
func WithLeaderElector(elector LeaderElector) SchedulerOption {
	return func(s *CronScheduler) {
		s.leaderElector = elector
	}
}

// WithInstanceID sets the instance ID for this scheduler
func WithInstanceID(id string) SchedulerOption {
	return func(s *CronScheduler) {
		s.instanceID = id
	}
}

// Schedule registers a job to be executed according to a cron specification
func (s *CronScheduler) Schedule(spec string, job Job) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	// Check if job already exists
	if _, exists := s.jobs[job.ID()]; exists {
		return fmt.Errorf("job with ID %s already scheduled", job.ID())
	}

	// Add the job to cron
	entryID, err := s.cron.AddFunc(spec, func() {
		s.executeJob(context.Background(), job)
	})

	if err != nil {
		return fmt.Errorf("invalid cron spec %q: %w", spec, err)
	}

	// Store job information
	entry := s.cron.Entry(entryID)
	s.jobs[job.ID()] = jobEntry{
		job:      job,
		spec:     spec,
		cronID:   entryID,
		nextRun:  entry.Next,
		metadata: job.Metadata(),
	}

	return nil
}

// Start begins the scheduler, executing jobs according to their schedules
func (s *CronScheduler) Start(ctx context.Context) error {
	// Start leadership check goroutine
	go s.leadershipMonitor(ctx)

	// Start the cron scheduler
	s.cron.Start()

	return nil
}

// Stop gracefully stops the scheduler, allowing in-progress jobs to complete
func (s *CronScheduler) Stop(ctx context.Context) error {
	stopCtx := s.cron.Stop()

	// Wait for jobs to complete or context to be done
	select {
	case <-stopCtx.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Jobs returns all registered jobs
func (s *CronScheduler) Jobs() []JobInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	jobs := make([]JobInfo, 0, len(s.jobs))
	for _, entry := range s.jobs {
		jobs = append(jobs, JobInfo{
			ID:        entry.job.ID(),
			Name:      entry.job.Name(),
			Spec:      entry.spec,
			NextRun:   entry.nextRun,
			LastRun:   entry.lastRun,
			LastError: entry.lastErr,
			Metadata:  entry.metadata,
		})
	}

	return jobs
}

// executeJob handles the execution of a job with leader election
func (s *CronScheduler) executeJob(ctx context.Context, job Job) {
	// Check if we should run singleton jobs
	if job.Metadata().Singleton && !s.isLeader {
		// Skip execution if this is a singleton job and we're not the leader
		return
	}

	// Create execution record
	execID := generateExecutionID()
	execution := &JobExecution{
		JobID:       job.ID(),
		ExecutionID: execID,
		StartTime:   time.Now(),
		Status:      JobStatusRunning,
		InstanceID:  s.instanceID,
	}

	// Store execution
	s.execMutex.Lock()
	s.executions[execID] = execution
	s.execMutex.Unlock()

	// Create context with timeout if specified
	jobCtx := ctx
	if timeout := job.Metadata().Timeout; timeout > 0 {
		var cancel context.CancelFunc
		jobCtx, cancel = context.WithTimeout(ctx, timeout)
		defer cancel()
	}

	// Execute the job
	err := job.Execute(jobCtx)

	// Update execution record
	endTime := time.Now()
	execution.EndTime = &endTime
	if err != nil {
		execution.Status = JobStatusFailed
		execution.Error = err
	} else {
		execution.Status = JobStatusCompleted
	}

	// Update job entry
	s.mutex.Lock()
	if entry, ok := s.jobs[job.ID()]; ok {
		entry.lastRun = &endTime
		entry.lastErr = err
		entry.nextRun = s.cron.Entry(entry.cronID).Next
		s.jobs[job.ID()] = entry
	}
	s.mutex.Unlock()
}

// leadershipMonitor periodically checks if this instance is the leader
func (s *CronScheduler) leadershipMonitor(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isLeader, err := s.leaderElector.IsLeader(ctx)
			if err == nil {
				s.isLeader = isLeader
			}
		}
	}
}

// generateInstanceID creates a unique identifier for this scheduler instance
func generateInstanceID() string {
	return fmt.Sprintf("scheduler-%d", time.Now().UnixNano())
}

// generateExecutionID creates a unique identifier for a job execution
func generateExecutionID() string {
	return fmt.Sprintf("exec-%d", time.Now().UnixNano())
}

// localLeaderElector is a simple implementation that always returns the same leadership status
type localLeaderElector struct {
	isLeader bool
}

func (l *localLeaderElector) IsLeader(ctx context.Context) (bool, error) {
	return l.isLeader, nil
}

func (l *localLeaderElector) RunForLeader(ctx context.Context) error {
	l.isLeader = true
	return nil
}

func (l *localLeaderElector) ResignLeadership(ctx context.Context) error {
	l.isLeader = false
	return nil
}
