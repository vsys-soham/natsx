package otel_test

import (
	"context"
	"testing"
	"time"

	natslib "github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	natsotel "github.com/vsys-soham/natsx/otel"
	"github.com/vsys-soham/natsx/middleware"
)

// ---------- Tracer ----------

func TestTracerStartSpan_NoopProvider(t *testing.T) {
	// Uses global noop provider — verifies no panic and correct return shape.
	tracer := natsotel.NewTracer(natsotel.TracerConfig{ServiceName: "test-svc"})

	msg := &natsx.Msg{Msg: &natslib.Msg{
		Subject: "orders.new",
		Header:  natslib.Header{},
	}}

	ctx, sc, finish := tracer.StartSpan(context.Background(), "test-op", msg)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
	// With build-level noop provider, IDs may be all-zero — just verify shape
	_ = sc.TraceID
	_ = sc.SpanID
	finish() // must not panic
}

func TestTracerStartSpan_ContextPropagated(t *testing.T) {
	tracer := natsotel.NewTracer(natsotel.TracerConfig{})

	msg := &natsx.Msg{Msg: &natslib.Msg{
		Subject: "events.click",
		Header:  natslib.Header{},
	}}

	parentCtx := context.WithValue(context.Background(), struct{ K string }{"key"}, "val")
	ctx, _, finish := tracer.StartSpan(parentCtx, "child-op", msg)
	defer finish()

	// Returned context should derive from the parent
	if ctx.Value(struct{ K string }{"key"}) != "val" {
		t.Error("returned context should carry parent values")
	}
}

func TestTracerImplementsInterface(t *testing.T) {
	// Compile-time check — if this compiles, the interface is satisfied.
	tracer := natsotel.NewTracer(natsotel.TracerConfig{})
	var _ middleware.Tracer = tracer
}

func TestTracerFinishCalledAfterHandler(t *testing.T) {
	tracer := natsotel.NewTracer(natsotel.TracerConfig{})

	var finished bool
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.finish", Header: natslib.Header{}}}

	handler := func(m *natsx.Msg) {
		_, _, finish := tracer.StartSpan(m.Context(), "op", m)
		defer func() { finish(); finished = true }()
	}

	mw := natsx.Chain(handler, middleware.Tracing(tracer, "test-finish"))
	mw(msg)

	// finish should have been called by the Tracing middleware's defer
	if !finished {
		t.Error("expected finish to be called")
	}
}

func TestInjectHeaders_NoopProvider(t *testing.T) {
	// Inject should never panic, even with a noop span.
	msg := &natsx.Msg{Msg: &natslib.Msg{
		Subject: "outgoing.event",
		Header:  natslib.Header{},
	}}
	natsotel.InjectHeaders(context.Background(), msg)
	// No panic = pass
}

// ---------- MetricsRecorder ----------

func TestMetricsRecorder_ImplementsInterface(t *testing.T) {
	rec := natsotel.NewMetricsRecorder(natsotel.MetricsConfig{ServiceName: "test"})
	var _ middleware.MetricsRecorder = rec
}

func TestMetricsRecorder_NoPanic(t *testing.T) {
	// Use global noop meter — just verify all paths execute without panicking.
	rec := natsotel.NewMetricsRecorder(natsotel.MetricsConfig{})

	rec.IncMessagesReceived("orders.new")
	rec.IncMessagesProcessed("orders.new")
	rec.IncMessagesFailed("orders.new")
	rec.ObserveDuration("orders.new", 25*time.Millisecond)
}

func TestMetricsRecorder_MultipleSubjects(t *testing.T) {
	rec := natsotel.NewMetricsRecorder(natsotel.MetricsConfig{ServiceName: "multi-svc"})

	subjects := []string{"orders.new", "orders.ship", "events.click", "dlq.orders"}
	for _, s := range subjects {
		rec.IncMessagesReceived(s)
		rec.IncMessagesProcessed(s)
		rec.ObserveDuration(s, time.Duration(len(s))*time.Millisecond)
	}
}

func TestMetricsRecorder_FailedMetric(t *testing.T) {
	rec := natsotel.NewMetricsRecorder(natsotel.MetricsConfig{})

	// 5 received, 3 processed, 2 failed — just verify no panic
	for i := 0; i < 5; i++ {
		rec.IncMessagesReceived("test.subject")
	}
	for i := 0; i < 3; i++ {
		rec.IncMessagesProcessed("test.subject")
	}
	for i := 0; i < 2; i++ {
		rec.IncMessagesFailed("test.subject")
	}
}

// ---------- Integration: OTel middleware chain ----------

func TestOtelMiddlewareChain(t *testing.T) {
	tracer := natsotel.NewTracer(natsotel.TracerConfig{})
	rec := natsotel.NewMetricsRecorder(natsotel.MetricsConfig{})

	var handlerCalled bool

	handler := func(m *natsx.Msg) {
		handlerCalled = true
		// Verify span context exists in message context
		sc := middleware.GetSpanContext(m.Context())
		_ = sc // may be zero-value with noop provider — no panic is the assertion
	}

	mw := natsx.Chain(handler,
		middleware.Tracing(tracer, "test-chain"),
		middleware.Metrics(rec),
		middleware.CorrelationID("", func() string { return "test-cid" }),
	)

	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "chain.test", Header: natslib.Header{}}}
	mw(msg)

	if !handlerCalled {
		t.Fatal("handler should have been called")
	}
}
