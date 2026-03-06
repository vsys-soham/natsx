// Package retry provides configurable retry policies with exponential backoff
// and jitter for use with NATS operations.
package retry

import (
	"context"
	"errors"
	"math"
	"math/rand/v2"
	"time"
)

// Policy controls retry behavior.
type Policy struct {
	// MaxAttempts is the maximum number of attempts (including the first).
	// 0 or 1 means no retries. Negative means unlimited.
	MaxAttempts int

	// InitialBackoff is the delay before the first retry.
	InitialBackoff time.Duration

	// MaxBackoff caps the backoff duration.
	MaxBackoff time.Duration

	// Multiplier is applied to the backoff after each attempt.
	Multiplier float64

	// Jitter adds randomness to the backoff. 0.0 = no jitter, 1.0 = full jitter.
	// The actual delay is: backoff * (1 - jitter) + rand(0, backoff * jitter).
	Jitter float64

	// Classifier determines whether an error is retryable.
	// If nil, all non-nil errors are retried.
	Classifier func(error) bool
}

// DefaultPolicy returns a production-safe retry policy.
func DefaultPolicy() Policy {
	return Policy{
		MaxAttempts:    3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     5 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.2,
	}
}

// NoRetry returns a policy that never retries.
func NoRetry() Policy {
	return Policy{MaxAttempts: 1}
}

// backoff calculates the backoff duration for a given attempt (0-indexed).
func (p Policy) backoff(attempt int) time.Duration {
	b := float64(p.InitialBackoff) * math.Pow(p.Multiplier, float64(attempt))
	if b > float64(p.MaxBackoff) {
		b = float64(p.MaxBackoff)
	}

	if p.Jitter > 0 {
		jit := b * p.Jitter
		b = b*(1-p.Jitter) + rand.Float64()*jit
	}

	return time.Duration(b)
}

// isRetryable checks whether an error should be retried according to the policy.
func (p Policy) isRetryable(err error) bool {
	if err == nil {
		return false
	}
	if p.Classifier != nil {
		return p.Classifier(err)
	}
	return true
}

// hasAttemptsLeft returns true if more attempts are allowed.
func (p Policy) hasAttemptsLeft(attempt int) bool {
	if p.MaxAttempts < 0 {
		return true // unlimited
	}
	return attempt < p.MaxAttempts
}

// Result holds the outcome of a retry operation.
type Result struct {
	// Attempts is the total number of attempts made.
	Attempts int

	// Err is the last error, or nil on success.
	Err error
}

// Do executes fn with the given retry policy. It retries on retryable errors
// until the policy is exhausted or the context is cancelled.
//
// The attempt number (0-indexed) is passed to fn for observability.
func Do(ctx context.Context, p Policy, fn func(ctx context.Context, attempt int) error) Result {
	var lastErr error

	for attempt := 0; p.hasAttemptsLeft(attempt); attempt++ {
		if err := ctx.Err(); err != nil {
			return Result{Attempts: attempt, Err: err}
		}

		lastErr = fn(ctx, attempt)
		if lastErr == nil {
			return Result{Attempts: attempt + 1}
		}

		if !p.isRetryable(lastErr) {
			return Result{Attempts: attempt + 1, Err: lastErr}
		}

		// Don't sleep after the last attempt.
		if !p.hasAttemptsLeft(attempt + 1) {
			break
		}

		wait := p.backoff(attempt)
		select {
		case <-ctx.Done():
			return Result{Attempts: attempt + 1, Err: ctx.Err()}
		case <-time.After(wait):
		}
	}

	return Result{Attempts: p.MaxAttempts, Err: lastErr}
}

// Permanent wraps an error to signal that it should not be retried,
// regardless of the policy's classifier.
type Permanent struct {
	Err error
}

func (e *Permanent) Error() string { return e.Err.Error() }
func (e *Permanent) Unwrap() error { return e.Err }

// IsPermanent reports whether err (or any error in its chain) is a Permanent error.
func IsPermanent(err error) bool {
	var p *Permanent
	return errors.As(err, &p)
}

// ClassifyPermanent is a Classifier that does not retry Permanent errors.
// All other errors are retried.
func ClassifyPermanent(err error) bool {
	return !IsPermanent(err)
}
