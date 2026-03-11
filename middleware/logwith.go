package middleware

import (
	"github.com/vsys-soham/natsx"
)

// LogWith returns a logger wrapper that enriches every log call with
// correlation ID and span context from the message's context.
// Use this inside handlers to get structured logs with trace metadata.
//
// Example:
//
//	c.Subscribe("orders.>", func(msg *natsx.Msg) {
//	    log := middleware.LogWith(c.Log(), msg)
//	    log.Info("processing order", "order_id", "123")
//	    // Output includes: correlation_id=abc-123 trace_id=... span_id=...
//	})
func LogWith(log natsx.Logger, msg *natsx.Msg) natsx.Logger {
	ctx := msg.Context()
	cid := GetCorrelationID(ctx)
	sc := GetSpanContext(ctx)

	return &enrichedLogger{
		inner:         log,
		correlationID: cid,
		traceID:       sc.TraceID,
		spanID:        sc.SpanID,
		subject:       msg.Subject,
	}
}

// enrichedLogger wraps a Logger, prepending observability fields to every call.
type enrichedLogger struct {
	inner         natsx.Logger
	correlationID string
	traceID       string
	spanID        string
	subject       string
}

func (l *enrichedLogger) fields() []any {
	var kv []any
	if l.subject != "" {
		kv = append(kv, "subject", l.subject)
	}
	if l.correlationID != "" {
		kv = append(kv, "correlation_id", l.correlationID)
	}
	if l.traceID != "" {
		kv = append(kv, "trace_id", l.traceID)
	}
	if l.spanID != "" {
		kv = append(kv, "span_id", l.spanID)
	}
	return kv
}

func (l *enrichedLogger) Debug(msg string, kv ...any) {
	l.inner.Debug(msg, append(l.fields(), kv...)...)
}
func (l *enrichedLogger) Info(msg string, kv ...any) {
	l.inner.Info(msg, append(l.fields(), kv...)...)
}
func (l *enrichedLogger) Warn(msg string, kv ...any) {
	l.inner.Warn(msg, append(l.fields(), kv...)...)
}
func (l *enrichedLogger) Error(msg string, kv ...any) {
	l.inner.Error(msg, append(l.fields(), kv...)...)
}
