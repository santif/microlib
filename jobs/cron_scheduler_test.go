package jobs

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestJob is a simple job implementation for testing
type TestJob struct {
	id       string
	name     string
	execFunc func(ctx context.Context) error
	metadata JobMetadata
	executed bool
	mutex    sync.Mutex
}

func NewTestJob(id, name string, execFunc func(ctx context.Context) error) *TestJob {
	return &TestJob{
		id:       id,
		name:     name,
		execFunc: execFunc,
		metadata: JobMetadata{
			Description: "Test job",
			Timeout:     time.Second,
		},
	}
}

func (j *TestJob) ID() string {
	return j.id
}

func (j *TestJob) Name() string {
	return j.name
}

func (j *TestJob) Execute(ctx context.Context) error {
	j.mutex.Lock()
	j.executed = true
	j.mutex.Unlock()

	if j.execFunc != nil {
		return j.execFunc(ctx)
	}
	return nil
}

func (j *TestJob) Metadata() JobMetadata {
	return j.metadata
}

func (j *TestJob) WasExecuted() bool {
	j.mutex.Lock()
	defer j.mutex.Unlock()
	return j.executed
}

func TestCronScheduler_Schedule(t *testing.T) {
	scheduler := NewCronScheduler()

	// Test scheduling a job
	job := NewTestJob("test-job", "Test Job", nil)
	err := scheduler.Schedule("* * * * * *", job)
	if err != nil {
		t.Fatalf("Failed to schedule job: %v", err)
	}

	// Test scheduling the same job again (should fail)
	err = scheduler.Schedule("* * * * * *", job)
	if err == nil {
		t.Fatal("Expected error when scheduling the same job twice, got nil")
	}

	// Test scheduling with invalid cron spec
	job2 := NewTestJob("test-job-2", "Test Job 2", nil)
	err = scheduler.Schedule("invalid spec", job2)
	if err == nil {
		t.Fatal("Expected error when using invalid cron spec, got nil")
	}
}

func TestCronScheduler_Jobs(t *testing.T) {
	scheduler := NewCronScheduler()

	// Schedule a job
	job := NewTestJob("test-job", "Test Job", nil)
	err := scheduler.Schedule("* * * * * *", job)
	if err != nil {
		t.Fatalf("Failed to schedule job: %v", err)
	}

	// Check jobs list
	jobs := scheduler.Jobs()
	if len(jobs) != 1 {
		t.Fatalf("Expected 1 job, got %d", len(jobs))
	}

	if jobs[0].ID != "test-job" {
		t.Errorf("Expected job ID 'test-job', got '%s'", jobs[0].ID)
	}

	if jobs[0].Name != "Test Job" {
		t.Errorf("Expected job name 'Test Job', got '%s'", jobs[0].Name)
	}

	if jobs[0].Spec != "* * * * * *" {
		t.Errorf("Expected job spec '* * * * * *', got '%s'", jobs[0].Spec)
	}
}

func TestCronScheduler_LeaderElection(t *testing.T) {
	// Create a mock leader elector that can be controlled for testing
	mockElector := &mockLeaderElector{
		isLeader: false,
	}

	// Create a job
	job := NewTestJob("singleton-job", "Singleton Job", nil)
	job.metadata.Singleton = true

	// Create scheduler with mock leader elector
	scheduler := NewCronScheduler(WithLeaderElector(mockElector))

	// Manually execute job (not leader, should not execute singleton job)
	ctx := context.Background()
	scheduler.executeJob(ctx, job)

	// Check if job was executed (should not be)
	if job.WasExecuted() {
		t.Fatal("Job executed despite not being the leader")
	}

	// Now become the leader
	mockElector.SetLeader(true)
	scheduler.isLeader = true // Manually update leader status

	// Manually execute job again (now as leader, should execute)
	scheduler.executeJob(ctx, job)

	// Check if job was executed
	if !job.WasExecuted() {
		t.Fatal("Job did not execute after becoming leader")
	}
}

// mockLeaderElector is a controllable leader elector for testing
type mockLeaderElector struct {
	isLeader bool
	mutex    sync.Mutex
}

func (m *mockLeaderElector) IsLeader(ctx context.Context) (bool, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	return m.isLeader, nil
}

func (m *mockLeaderElector) RunForLeader(ctx context.Context) error {
	return nil
}

func (m *mockLeaderElector) ResignLeadership(ctx context.Context) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.isLeader = false
	return nil
}

func (m *mockLeaderElector) SetLeader(isLeader bool) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	m.isLeader = isLeader
}
