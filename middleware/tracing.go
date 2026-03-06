package middleware

import (
	"context"

	"github.com/vsys-soham/natsx"
)

// SpanContext holds trace/span identifiers extracted or created by the tracing
// middleware. This abstraction avoids a hard dependency on OpenTelemetry in
// the middleware package — the actual OTel integration happens in the otel/ package.
type SpanContext struct {
	TraceID string
	SpanID  string
}

// spanContextKey is the context key for SpanContext.
type spanContextKey struct{}

// Tracer is an interface for creating and extracting trace spans from NATS
// message headers. Implement this interface to integrate with your tracing
// backend (OpenTelemetry, Jaeger, Zipkin, etc.).
type Tracer interface {
	// StartSpan creates a new span for the given message. It should:
	//   1. Extract any parent span from msg headers
	//   2. Create a child span (or root span if no parent)
	//   3. Return the span context and a finish function
	//
	// The finish function is called after the handler completes.
	StartSpan(ctx context.Context, operationName string, msg *natsx.Msg) (context.Context, SpanContext, func())
}

// Tracing returns a middleware that creates a trace span for each message.
// The span context is stored in the message context and can be retrieved
// with GetSpanContext().
func Tracing(tracer Tracer, operationName string) natsx.Middleware {
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			ctx, sc, finish := tracer.StartSpan(msg.Context(), operationName, msg)
			defer finish()

			ctx = context.WithValue(ctx, spanContextKey{}, sc)
			msg = msg.WithContext(ctx)
			next(msg)
		}
	}
}

// GetSpanContext extracts the SpanContext from a context.
// Returns a zero-value SpanContext if none is set.
func GetSpanContext(ctx context.Context) SpanContext {
	if v, ok := ctx.Value(spanContextKey{}).(SpanContext); ok {
		return v
	}
	return SpanContext{}
}

// NopTracer is a no-op Tracer implementation. Useful for tests.
type NopTracer struct{}

func (NopTracer) StartSpan(ctx context.Context, _ string, _ *natsx.Msg) (context.Context, SpanContext, func()) {
	return ctx, SpanContext{}, func() {}
}
