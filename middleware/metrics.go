package middleware

import (
	"time"

	"github.com/vsys-soham/natsx"
)

// MetricsRecorder is an interface for recording message processing metrics.
// Implement this interface to integrate with your metrics backend (Prometheus,
// OTel, StatsD, etc.) without coupling the middleware to a specific library.
type MetricsRecorder interface {
	// IncMessagesReceived increments the counter for received messages.
	IncMessagesReceived(subject string)

	// IncMessagesProcessed increments the counter for successfully processed messages.
	IncMessagesProcessed(subject string)

	// IncMessagesFailed increments the counter for failed messages (handler panicked).
	IncMessagesFailed(subject string)

	// ObserveDuration records the duration of message processing.
	ObserveDuration(subject string, duration time.Duration)
}

// Metrics returns a middleware that records message processing metrics using
// the provided MetricsRecorder. It captures:
//   - messages received (before handler)
//   - messages processed / failed (after handler, detecting panics)
//   - processing duration histogram
//
// Panics are re-raised after recording the failure metric. Pair with Recovery
// middleware (placed before Metrics in the chain) to catch panics, or let them
// propagate if you want to crash on unhandled panics.
func Metrics(recorder MetricsRecorder) natsx.Middleware {
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			subject := msg.Subject
			recorder.IncMessagesReceived(subject)

			start := time.Now()
			panicked := true

			defer func() {
				duration := time.Since(start)
				recorder.ObserveDuration(subject, duration)
				if panicked {
					recorder.IncMessagesFailed(subject)
				} else {
					recorder.IncMessagesProcessed(subject)
				}
			}()

			next(msg)
			panicked = false
		}
	}
}

// NopMetrics is a no-op MetricsRecorder. Useful for tests and as a default.
type NopMetrics struct{}

func (NopMetrics) IncMessagesReceived(string)            {}
func (NopMetrics) IncMessagesProcessed(string)           {}
func (NopMetrics) IncMessagesFailed(string)              {}
func (NopMetrics) ObserveDuration(string, time.Duration) {}
