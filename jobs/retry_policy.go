package jobs

import (
	"math/rand"
	"time"
)

// RetryPolicyType defines the type of retry policy
type RetryPolicyType string

const (
	// RetryPolicyTypeExponential represents an exponential backoff retry policy
	RetryPolicyTypeExponential RetryPolicyType = "exponential"

	// RetryPolicyTypeFixed represents a fixed interval retry policy
	RetryPolicyTypeFixed RetryPolicyType = "fixed"

	// RetryPolicyTypeNone represents no retry policy
	RetryPolicyTypeNone RetryPolicyType = "none"
)

// RetryPolicy defines how jobs should be retried on failure
type RetryPolicy struct {
	// Type defines the type of retry policy
	Type RetryPolicyType `yaml:"type" json:"type" validate:"required,oneof=exponential fixed none"`

	// MaxRetries is the maximum number of retry attempts
	MaxRetries int `yaml:"maxRetries" json:"maxRetries" validate:"min=0"`

	// InitialInterval is the initial delay before the first retry
	InitialInterval time.Duration `yaml:"initialInterval" json:"initialInterval"`

	// MaxInterval is the maximum delay between retries
	MaxInterval time.Duration `yaml:"maxInterval" json:"maxInterval"`

	// Multiplier is the factor by which the delay increases with each retry
	Multiplier float64 `yaml:"multiplier" json:"multiplier" validate:"min=1"`

	// RandomizationFactor adds jitter to retry intervals
	RandomizationFactor float64 `yaml:"randomizationFactor" json:"randomizationFactor" validate:"min=0,max=1"`
}

// DefaultRetryPolicy returns a sensible default retry policy
func DefaultRetryPolicy() RetryPolicy {
	return RetryPolicy{
		Type:                RetryPolicyTypeExponential,
		MaxRetries:          3,
		InitialInterval:     1 * time.Second,
		MaxInterval:         1 * time.Minute,
		Multiplier:          2.0,
		RandomizationFactor: 0.2,
	}
}

// NoRetryPolicy returns a policy that doesn't retry
func NoRetryPolicy() RetryPolicy {
	return RetryPolicy{
		Type:       RetryPolicyTypeNone,
		MaxRetries: 0,
	}
}

// FixedRetryPolicy returns a policy with fixed intervals
func FixedRetryPolicy(interval time.Duration, maxRetries int) RetryPolicy {
	return RetryPolicy{
		Type:                RetryPolicyTypeFixed,
		MaxRetries:          maxRetries,
		InitialInterval:     interval,
		MaxInterval:         interval,
		Multiplier:          1.0,
		RandomizationFactor: 0,
	}
}

// CalculateBackoff calculates the next retry interval based on the retry policy
func CalculateBackoff(policy RetryPolicy, retryCount int) time.Duration {
	if retryCount <= 0 || policy.Type == RetryPolicyTypeNone || policy.MaxRetries < retryCount {
		return 0
	}

	var interval time.Duration

	switch policy.Type {
	case RetryPolicyTypeFixed:
		interval = policy.InitialInterval
	case RetryPolicyTypeExponential:
		// Calculate base interval with exponential backoff
		intervalFloat := float64(policy.InitialInterval)
		for i := 1; i < retryCount; i++ {
			intervalFloat *= policy.Multiplier
		}

		// Apply max interval cap
		if intervalFloat > float64(policy.MaxInterval) {
			intervalFloat = float64(policy.MaxInterval)
		}

		// Apply jitter
		if policy.RandomizationFactor > 0 {
			delta := policy.RandomizationFactor * intervalFloat
			minInterval := intervalFloat - delta
			maxInterval := intervalFloat + delta

			// Generate random value between min and max
			intervalFloat = minInterval + (rand.Float64() * (maxInterval - minInterval))
		}

		interval = time.Duration(intervalFloat)
	default:
		// Default to no retry
		return 0
	}

	return interval
}

// ShouldRetry determines if a job should be retried based on the retry policy and current retry count
func ShouldRetry(policy RetryPolicy, retryCount int) bool {
	return policy.Type != RetryPolicyTypeNone && retryCount <= policy.MaxRetries
}

// NextRetryTime calculates the next retry time based on the retry policy and current retry count
func NextRetryTime(policy RetryPolicy, retryCount int) *time.Time {
	backoff := CalculateBackoff(policy, retryCount)
	if backoff == 0 {
		return nil
	}

	nextRetry := time.Now().Add(backoff)
	return &nextRetry
}
