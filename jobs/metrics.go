package jobs

import (
	"github.com/santif/microlib/observability"
)

// JobQueueMetrics contains metrics for job queue operations
type JobQueueMetrics struct {
	// Counters
	enqueuedJobs     observability.Counter
	startedJobs      observability.Counter
	completedJobs    observability.Counter
	failedJobs       observability.Counter
	retriedJobs      observability.Counter
	deadLetteredJobs observability.Counter
	cancelledJobs    observability.Counter

	// Histograms
	jobDuration   observability.Histogram
	jobQueueTime  observability.Histogram
	jobRetryCount observability.Histogram
}

// newJobQueueMetrics creates and registers job queue metrics
func newJobQueueMetrics(metrics observability.Metrics, queueName string) *JobQueueMetrics {
	labels := map[string]string{"queue": queueName}

	return &JobQueueMetrics{
		enqueuedJobs: metrics.Counter(
			"jobs_enqueued_total",
			"Total number of jobs enqueued",
			"queue",
		).WithLabels(labels),
		startedJobs: metrics.Counter(
			"jobs_started_total",
			"Total number of jobs started",
			"queue",
		).WithLabels(labels),
		completedJobs: metrics.Counter(
			"jobs_completed_total",
			"Total number of jobs completed successfully",
			"queue",
		).WithLabels(labels),
		failedJobs: metrics.Counter(
			"jobs_failed_total",
			"Total number of jobs that failed",
			"queue",
		).WithLabels(labels),
		retriedJobs: metrics.Counter(
			"jobs_retried_total",
			"Total number of jobs that were retried",
			"queue",
		).WithLabels(labels),
		deadLetteredJobs: metrics.Counter(
			"jobs_deadlettered_total",
			"Total number of jobs that were moved to the dead letter queue",
			"queue",
		).WithLabels(labels),
		cancelledJobs: metrics.Counter(
			"jobs_cancelled_total",
			"Total number of jobs that were cancelled",
			"queue",
		).WithLabels(labels),
		jobDuration: metrics.Histogram(
			"job_duration_seconds",
			"Duration of job execution in seconds",
			[]float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60, 300, 600},
			"queue",
		).WithLabels(labels),
		jobQueueTime: metrics.Histogram(
			"job_queue_time_seconds",
			"Time spent in queue before execution in seconds",
			[]float64{0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30, 60, 300, 600},
			"queue",
		).WithLabels(labels),
		jobRetryCount: metrics.Histogram(
			"job_retry_count",
			"Number of retries per job",
			[]float64{0, 1, 2, 3, 5, 10},
			"queue",
		).WithLabels(labels),
	}
}
