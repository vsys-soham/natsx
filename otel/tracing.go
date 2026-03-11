// Package otel provides OpenTelemetry implementations of the middleware.Tracer
// and middleware.MetricsRecorder interfaces.
//
// This package has a hard dependency on go.opentelemetry.io/otel. If you don't
// use OpenTelemetry, use middleware.NopTracer and middleware.NopMetrics instead.
//
// Usage:
//
//	import "github.com/vsys-soham/natsx/otel"
//
//	tracer := otel.NewTracer(otel.TracerConfig{ServiceName: "my-svc"})
//	recorder := otel.NewMetricsRecorder(otel.MetricsConfig{ServiceName: "my-svc"})
//
//	c.Subscribe("orders.>", handler,
//	    middleware.Tracing(tracer, "process-order"),
//	    middleware.Metrics(recorder),
//	)
package otel

import (
	"context"

	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/middleware"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// TracerConfig configures the OpenTelemetry tracer.
type TracerConfig struct {
	// ServiceName is used as the tracer name. Defaults to "natsx".
	ServiceName string

	// TracerProvider is the OTel tracer provider. If nil, uses the global provider.
	TracerProvider trace.TracerProvider
}

// Tracer implements middleware.Tracer using OpenTelemetry.
type Tracer struct {
	tracer trace.Tracer
}

// NewTracer creates a new OTel-backed Tracer.
func NewTracer(cfg TracerConfig) *Tracer {
	name := cfg.ServiceName
	if name == "" {
		name = "natsx"
	}

	var tp trace.TracerProvider
	if cfg.TracerProvider != nil {
		tp = cfg.TracerProvider
	} else {
		tp = otel.GetTracerProvider()
	}

	return &Tracer{
		tracer: tp.Tracer(name),
	}
}

// headerCarrier adapts nats.Header to propagation.TextMapCarrier.
type headerCarrier natsx.Msg

func (c *headerCarrier) Get(key string) string {
	return (*natsx.Msg)(c).Header.Get(key)
}

func (c *headerCarrier) Set(key, value string) {
	(*natsx.Msg)(c).Header.Set(key, value)
}

func (c *headerCarrier) Keys() []string {
	msg := (*natsx.Msg)(c)
	keys := make([]string, 0, len(msg.Header))
	for k := range msg.Header {
		keys = append(keys, k)
	}
	return keys
}

// StartSpan implements middleware.Tracer. It extracts parent context from NATS
// headers, creates a child span, and returns a finish function.
func (t *Tracer) StartSpan(ctx context.Context, operationName string, msg *natsx.Msg) (context.Context, middleware.SpanContext, func()) {
	// Extract parent span context from NATS headers.
	prop := otel.GetTextMapPropagator()
	ctx = prop.Extract(ctx, (*headerCarrier)(msg))

	// Start a new span.
	ctx, span := t.tracer.Start(ctx, operationName,
		trace.WithSpanKind(trace.SpanKindConsumer),
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination", msg.Subject),
			attribute.String("messaging.operation", "receive"),
		),
	)

	sc := span.SpanContext()
	mwSC := middleware.SpanContext{
		TraceID: sc.TraceID().String(),
		SpanID:  sc.SpanID().String(),
	}

	finish := func() {
		span.End()
	}

	return ctx, mwSC, finish
}

// InjectHeaders injects the current span context into NATS message headers
// for propagation to downstream services. Call this before publishing.
func InjectHeaders(ctx context.Context, msg *natsx.Msg) {
	prop := otel.GetTextMapPropagator()
	prop.Inject(ctx, (*headerCarrier)(msg))
}

// compile-time interface check
var _ middleware.Tracer = (*Tracer)(nil)
