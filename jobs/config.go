package jobs

import (
	"time"
)

// SchedulerConfig contains configuration for the job scheduler
type SchedulerConfig struct {
	// Enabled determines if the scheduler is active
	Enabled bool `yaml:"enabled" json:"enabled" validate:"required"`

	// LeaderElection contains configuration for leader election
	LeaderElection LeaderElectionConfig `yaml:"leaderElection" json:"leaderElection"`

	// MaxConcurrent is the maximum number of jobs that can run concurrently
	MaxConcurrent int `yaml:"maxConcurrent" json:"maxConcurrent" validate:"min=1"`

	// DefaultTimeout is the default timeout for jobs that don't specify one
	DefaultTimeout time.Duration `yaml:"defaultTimeout" json:"defaultTimeout"`

	// HistoryLimit is the number of job executions to keep in history
	HistoryLimit int `yaml:"historyLimit" json:"historyLimit" validate:"min=0"`
}

// DefaultSchedulerConfig returns a default configuration for the scheduler
func DefaultSchedulerConfig() SchedulerConfig {
	return SchedulerConfig{
		Enabled: true,
		LeaderElection: LeaderElectionConfig{
			LeaseDuration: 15 * time.Second,
			RenewDeadline: 10 * time.Second,
			RetryPeriod:   2 * time.Second,
			LeaseName:     "microlib-scheduler",
		},
		MaxConcurrent:  5,
		DefaultTimeout: 5 * time.Minute,
		HistoryLimit:   100,
	}
}
