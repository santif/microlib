package messaging

import (
	"context"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/santif/microlib/observability"
)

// RelayConfig holds configuration for the OutboxRelay
type RelayConfig struct {
	// BatchSize is the maximum number of messages to process in a single batch
	BatchSize int `yaml:"batchSize" validate:"min=1"`

	// PollInterval is the time to wait between processing batches
	PollInterval time.Duration `yaml:"pollInterval" validate:"required"`

	// MaxRetries is the maximum number of retries for a message before giving up
	MaxRetries int `yaml:"maxRetries" validate:"min=0"`

	// InitialBackoff is the initial backoff duration for the first retry
	InitialBackoff time.Duration `yaml:"initialBackoff" validate:"required"`

	// MaxBackoff is the maximum backoff duration for any retry
	MaxBackoff time.Duration `yaml:"maxBackoff" validate:"required"`

	// BackoffFactor is the multiplier for each subsequent retry backoff
	BackoffFactor float64 `yaml:"backoffFactor" validate:"min=1"`
}

// DefaultRelayConfig returns a default configuration for the OutboxRelay
func DefaultRelayConfig() RelayConfig {
	return RelayConfig{
		BatchSize:      100,
		PollInterval:   time.Second * 5,
		MaxRetries:     10,
		InitialBackoff: time.Second * 1,
		MaxBackoff:     time.Hour * 1,
		BackoffFactor:  2.0,
	}
}

// OutboxRelay processes messages from the outbox and publishes them to the broker
type OutboxRelay struct {
	store        OutboxStore
	broker       Broker
	config       RelayConfig
	logger       observability.Logger
	metrics      observability.Metrics
	processingMu sync.Mutex
	stopCh       chan struct{}
	stopped      bool
	wg           sync.WaitGroup
}

// NewOutboxRelay creates a new OutboxRelay
func NewOutboxRelay(
	store OutboxStore,
	broker Broker,
	config RelayConfig,
	logger observability.Logger,
	metrics observability.Metrics,
) *OutboxRelay {
	if logger == nil {
		logger = observability.NewLogger()
	}

	if metrics == nil {
		metrics = observability.NewMetrics()
	}

	return &OutboxRelay{
		store:   store,
		broker:  broker,
		config:  config,
		logger:  logger,
		metrics: metrics,
		stopCh:  make(chan struct{}),
	}
}

// Start begins the relay process in a background goroutine
func (r *OutboxRelay) Start(ctx context.Context) error {
	r.processingMu.Lock()
	defer r.processingMu.Unlock()

	if r.stopped {
		return fmt.Errorf("relay has been stopped and cannot be restarted")
	}

	r.wg.Add(1)
	go r.processLoop(ctx)

	r.logger.Info("Outbox relay started",
		observability.NewField("batchSize", r.config.BatchSize),
		observability.NewField("pollInterval", r.config.PollInterval.String()))

	return nil
}

// Stop gracefully stops the relay process
func (r *OutboxRelay) Stop(ctx context.Context) error {
	r.processingMu.Lock()
	defer r.processingMu.Unlock()

	if r.stopped {
		return nil
	}

	r.stopped = true
	close(r.stopCh)

	// Wait for processing to complete with timeout
	done := make(chan struct{})
	go func() {
		r.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		r.logger.Info("Outbox relay stopped gracefully")
		return nil
	case <-ctx.Done():
		r.logger.Error("Outbox relay stop timed out", ctx.Err())
		return ctx.Err()
	}
}

// processLoop continuously processes outbox messages until stopped
func (r *OutboxRelay) processLoop(ctx context.Context) {
	defer r.wg.Done()

	ticker := time.NewTicker(r.config.PollInterval)
	defer ticker.Stop()

	// Process immediately on startup
	r.processBatch(ctx)

	for {
		select {
		case <-ticker.C:
			r.processBatch(ctx)
		case <-r.stopCh:
			return
		case <-ctx.Done():
			return
		}
	}
}

// processBatch processes a single batch of outbox messages
func (r *OutboxRelay) processBatch(ctx context.Context) {
	startTime := time.Now()

	// Create a child context with timeout
	processCtx, cancel := context.WithTimeout(ctx, r.config.PollInterval)
	defer cancel()

	messages, err := r.store.GetPendingMessages(processCtx, r.config.BatchSize)
	if err != nil {
		r.logger.Error("Failed to get pending messages", err,
			observability.NewField("component", "outbox_relay"))

		errCounter := r.metrics.Counter("outbox_relay_errors_total", "Total errors in outbox relay processing")
		errCounter.Inc()
		return
	}

	if len(messages) == 0 {
		return
	}

	r.logger.Info("Processing outbox messages",
		observability.NewField("count", len(messages)),
		observability.NewField("component", "outbox_relay"))

	processedCount := 0
	failedCount := 0

	// Process each message
	for _, msg := range messages {
		// Skip messages that have exceeded retry limit
		if msg.RetryCount >= r.config.MaxRetries && r.config.MaxRetries > 0 {
			r.logger.Error("Message exceeded retry limit", nil,
				observability.NewField("messageId", msg.ID),
				observability.NewField("topic", msg.Topic),
				observability.NewField("retries", msg.RetryCount),
				observability.NewField("component", "outbox_relay"))

			// Mark as processed to prevent further processing attempts
			// In a production system, you might want to move this to a dead letter queue instead
			if err := r.store.MarkProcessed(processCtx, msg.ID); err != nil {
				r.logger.Error("Failed to mark message as processed after max retries", err,
					observability.NewField("messageId", msg.ID))
			}

			droppedCounter := r.metrics.Counter("outbox_relay_messages_dropped_total", "Total messages dropped due to max retries")
			droppedCounter.Inc()
			continue
		}

		// Check if we should process this message now or skip due to backoff
		if msg.RetryCount > 0 {
			backoffDuration := r.calculateBackoff(msg.RetryCount)
			// If the message has failed before, check if it's time to retry
			retryAfter := time.Now().Add(-backoffDuration)
			if msg.CreatedAt.After(retryAfter) {
				// Not time to retry yet, skip this message
				continue
			}
		}

		// Publish the message
		err := r.broker.Publish(processCtx, msg.Topic, msg.ToMessage())
		if err != nil {
			failedCount++
			r.logger.Error("Failed to publish message", err,
				observability.NewField("messageId", msg.ID),
				observability.NewField("topic", msg.Topic),
				observability.NewField("retryCount", msg.RetryCount),
				observability.NewField("component", "outbox_relay"))

			if markErr := r.store.MarkFailed(processCtx, msg.ID, err); markErr != nil {
				r.logger.Error("Failed to mark message as failed", markErr,
					observability.NewField("messageId", msg.ID))
			}

			failureCounter := r.metrics.Counter("outbox_relay_publish_failures_total", "Total message publish failures")
			failureCounter.Inc()
			continue
		}

		// Mark as processed
		if err := r.store.MarkProcessed(processCtx, msg.ID); err != nil {
			r.logger.Error("Failed to mark message as processed", err,
				observability.NewField("messageId", msg.ID))

			errCounter := r.metrics.Counter("outbox_relay_errors_total", "Total errors in outbox relay processing")
			errCounter.Inc()
			continue
		}

		processedCount++
	}

	// Record metrics
	processedCounter := r.metrics.Counter("outbox_relay_messages_processed_total", "Total messages processed successfully")
	processedCounter.Add(float64(processedCount))

	failedCounter := r.metrics.Counter("outbox_relay_messages_failed_total", "Total messages that failed processing")
	failedCounter.Add(float64(failedCount))

	durationHistogram := r.metrics.Histogram("outbox_relay_batch_duration_seconds",
		"Duration of batch processing in seconds",
		[]float64{0.001, 0.01, 0.1, 0.5, 1, 5, 10})
	durationHistogram.Observe(time.Since(startTime).Seconds())

	batchSizeGauge := r.metrics.Gauge("outbox_relay_batch_size", "Size of the processed batch")
	batchSizeGauge.Set(float64(len(messages)))

	r.logger.Info("Completed processing outbox batch",
		observability.NewField("processed", processedCount),
		observability.NewField("failed", failedCount),
		observability.NewField("duration", time.Since(startTime).String()),
		observability.NewField("component", "outbox_relay"))
}

// calculateBackoff calculates the backoff duration for a retry attempt
func (r *OutboxRelay) calculateBackoff(retryCount int) time.Duration {
	// Calculate exponential backoff: initialBackoff * backoffFactor^(retryCount-1)
	backoffSeconds := float64(r.config.InitialBackoff) * math.Pow(r.config.BackoffFactor, float64(retryCount-1))
	backoff := time.Duration(backoffSeconds)

	// Cap at max backoff
	if backoff > r.config.MaxBackoff {
		backoff = r.config.MaxBackoff
	}

	return backoff
}
