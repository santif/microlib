package jobs

import (
	"testing"
	"time"
)

func TestRetryPolicy(t *testing.T) {
	t.Run("DefaultRetryPolicy", func(t *testing.T) {
		policy := DefaultRetryPolicy()

		if policy.Type != RetryPolicyTypeExponential {
			t.Errorf("Expected default policy type to be exponential, got %s", policy.Type)
		}

		if policy.MaxRetries != 3 {
			t.Errorf("Expected default max retries to be 3, got %d", policy.MaxRetries)
		}
	})

	t.Run("NoRetryPolicy", func(t *testing.T) {
		policy := NoRetryPolicy()

		if policy.Type != RetryPolicyTypeNone {
			t.Errorf("Expected no retry policy type to be none, got %s", policy.Type)
		}

		if policy.MaxRetries != 0 {
			t.Errorf("Expected no retry max retries to be 0, got %d", policy.MaxRetries)
		}
	})

	t.Run("FixedRetryPolicy", func(t *testing.T) {
		interval := 5 * time.Second
		maxRetries := 2
		policy := FixedRetryPolicy(interval, maxRetries)

		if policy.Type != RetryPolicyTypeFixed {
			t.Errorf("Expected fixed policy type to be fixed, got %s", policy.Type)
		}

		if policy.MaxRetries != maxRetries {
			t.Errorf("Expected fixed max retries to be %d, got %d", maxRetries, policy.MaxRetries)
		}

		if policy.InitialInterval != interval {
			t.Errorf("Expected fixed interval to be %v, got %v", interval, policy.InitialInterval)
		}
	})

	t.Run("CalculateBackoff_Exponential", func(t *testing.T) {
		policy := RetryPolicy{
			Type:                RetryPolicyTypeExponential,
			MaxRetries:          3,
			InitialInterval:     1 * time.Second,
			MaxInterval:         10 * time.Second,
			Multiplier:          2.0,
			RandomizationFactor: 0, // Disable randomization for predictable tests
		}

		// First retry should be initial interval
		backoff1 := CalculateBackoff(policy, 1)
		if backoff1 != 1*time.Second {
			t.Errorf("Expected first backoff to be 1s, got %v", backoff1)
		}

		// Second retry should be initial * multiplier
		backoff2 := CalculateBackoff(policy, 2)
		if backoff2 != 2*time.Second {
			t.Errorf("Expected second backoff to be 2s, got %v", backoff2)
		}

		// Third retry should be initial * multiplier^2
		backoff3 := CalculateBackoff(policy, 3)
		if backoff3 != 4*time.Second {
			t.Errorf("Expected third backoff to be 4s, got %v", backoff3)
		}

		// Fourth retry should be beyond max retries, so 0
		backoff4 := CalculateBackoff(policy, 4)
		if backoff4 != 0 {
			t.Errorf("Expected fourth backoff to be 0s, got %v", backoff4)
		}
	})

	t.Run("CalculateBackoff_MaxInterval", func(t *testing.T) {
		policy := RetryPolicy{
			Type:                RetryPolicyTypeExponential,
			MaxRetries:          5,
			InitialInterval:     1 * time.Second,
			MaxInterval:         5 * time.Second,
			Multiplier:          3.0,
			RandomizationFactor: 0, // Disable randomization for predictable tests
		}

		// Third retry would be 9s, but max is 5s
		backoff3 := CalculateBackoff(policy, 3)
		if backoff3 != 5*time.Second {
			t.Errorf("Expected third backoff to be capped at 5s, got %v", backoff3)
		}
	})

	t.Run("ShouldRetry", func(t *testing.T) {
		policy := DefaultRetryPolicy() // MaxRetries = 3

		if !ShouldRetry(policy, 1) {
			t.Error("Expected ShouldRetry to return true for retry count 1")
		}

		if !ShouldRetry(policy, 3) {
			t.Error("Expected ShouldRetry to return true for retry count 3")
		}

		if ShouldRetry(policy, 4) {
			t.Error("Expected ShouldRetry to return false for retry count 4")
		}

		noRetryPolicy := NoRetryPolicy()
		if ShouldRetry(noRetryPolicy, 1) {
			t.Error("Expected ShouldRetry to return false for NoRetryPolicy")
		}
	})

	t.Run("NextRetryTime", func(t *testing.T) {
		policy := RetryPolicy{
			Type:                RetryPolicyTypeFixed,
			MaxRetries:          3,
			InitialInterval:     5 * time.Second,
			RandomizationFactor: 0, // Disable randomization for predictable tests
		}

		now := time.Now()
		nextRetry := NextRetryTime(policy, 1)

		if nextRetry == nil {
			t.Fatal("Expected NextRetryTime to return a non-nil time")
		}

		// Should be approximately now + 5s
		expectedMin := now.Add(4900 * time.Millisecond)
		expectedMax := now.Add(5100 * time.Millisecond)

		if nextRetry.Before(expectedMin) || nextRetry.After(expectedMax) {
			t.Errorf("Expected next retry time to be around %v, got %v", now.Add(5*time.Second), nextRetry)
		}

		// Beyond max retries should return nil
		if NextRetryTime(policy, 4) != nil {
			t.Error("Expected NextRetryTime to return nil for retry count beyond max")
		}
	})
}
