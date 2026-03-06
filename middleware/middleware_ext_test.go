package middleware_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	natslib "github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/middleware"
)

// ---------- Timeout ----------

func TestTimeoutCompletes(t *testing.T) {
	log := natsx.NopLogger{}
	done := make(chan struct{})

	handler := func(m *natsx.Msg) {
		close(done)
	}

	mw := natsx.Chain(handler, middleware.Timeout(time.Second, log))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.timeout"}}
	mw(msg)

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("handler should have completed")
	}
}

func TestTimeoutExceeded(t *testing.T) {
	log := natsx.NopLogger{}

	handler := func(m *natsx.Msg) {
		// Handler checks context for cooperative cancellation.
		<-m.Context().Done()
	}

	mw := natsx.Chain(handler, middleware.Timeout(50*time.Millisecond, log))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.timeout.slow"}}

	start := time.Now()
	mw(msg)
	elapsed := time.Since(start)

	// Should have finished around the timeout duration.
	if elapsed < 40*time.Millisecond {
		t.Fatalf("expected ~50ms, got %v", elapsed)
	}
}

func TestTimeoutContextPropagated(t *testing.T) {
	log := natsx.NopLogger{}
	var hasDeadline bool

	handler := func(m *natsx.Msg) {
		_, hasDeadline = m.Context().Deadline()
	}

	mw := natsx.Chain(handler, middleware.Timeout(time.Second, log))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.timeout.ctx"}}
	mw(msg)

	if !hasDeadline {
		t.Fatal("expected context to have a deadline")
	}
}

// ---------- CorrelationID ----------

func TestCorrelationIDFromHeader(t *testing.T) {
	var capturedCID string

	handler := func(m *natsx.Msg) {
		capturedCID = middleware.GetCorrelationID(m.Context())
	}

	mw := natsx.Chain(handler, middleware.CorrelationID("", nil))

	msg := &natsx.Msg{Msg: &natslib.Msg{
		Subject: "test.cid",
		Header:  natslib.Header{"X-Correlation-ID": []string{"abc-123"}},
	}}
	mw(msg)

	if capturedCID != "abc-123" {
		t.Fatalf("expected cid=abc-123, got %q", capturedCID)
	}
}

func TestCorrelationIDGenerated(t *testing.T) {
	var capturedCID string

	handler := func(m *natsx.Msg) {
		capturedCID = middleware.GetCorrelationID(m.Context())
	}

	gen := func() string { return "gen-456" }
	mw := natsx.Chain(handler, middleware.CorrelationID("", gen))

	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.cid.gen"}}
	mw(msg)

	if capturedCID != "gen-456" {
		t.Fatalf("expected cid=gen-456, got %q", capturedCID)
	}
}

func TestCorrelationIDCustomHeader(t *testing.T) {
	var capturedCID string

	handler := func(m *natsx.Msg) {
		capturedCID = middleware.GetCorrelationID(m.Context())
	}

	mw := natsx.Chain(handler, middleware.CorrelationID("X-Request-ID", nil))

	msg := &natsx.Msg{Msg: &natslib.Msg{
		Subject: "test.cid.custom",
		Header:  natslib.Header{"X-Request-ID": []string{"req-789"}},
	}}
	mw(msg)

	if capturedCID != "req-789" {
		t.Fatalf("expected cid=req-789, got %q", capturedCID)
	}
}

func TestGetCorrelationIDEmpty(t *testing.T) {
	cid := middleware.GetCorrelationID(context.Background())
	if cid != "" {
		t.Fatalf("expected empty cid, got %q", cid)
	}
}

// ---------- ConcurrencyLimit ----------

func TestConcurrencyLimit(t *testing.T) {
	const maxConcurrent = 2
	var current atomic.Int32
	var peak atomic.Int32

	handler := func(m *natsx.Msg) {
		v := current.Add(1)
		// Update peak concurrency.
		for {
			p := peak.Load()
			if v <= p || peak.CompareAndSwap(p, v) {
				break
			}
		}
		time.Sleep(50 * time.Millisecond)
		current.Add(-1)
	}

	mw := natsx.Chain(handler, middleware.ConcurrencyLimit(maxConcurrent))

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			msg := &natsx.Msg{Msg: &natslib.Msg{Subject: fmt.Sprintf("test.conc.%d", idx)}}
			mw(msg)
		}(i)
	}
	wg.Wait()

	if p := peak.Load(); p > int32(maxConcurrent) {
		t.Fatalf("peak concurrency %d exceeded limit %d", p, maxConcurrent)
	}
}

// ---------- Validation ----------

func TestValidationPass(t *testing.T) {
	var called bool

	validator := func(msg *natsx.Msg) error {
		return nil // always valid
	}

	handler := func(m *natsx.Msg) {
		called = true
	}

	mw := natsx.Chain(handler, middleware.Validation(validator, nil))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.valid"}}
	mw(msg)

	if !called {
		t.Fatal("handler should have been called")
	}
}

func TestValidationReject(t *testing.T) {
	var handlerCalled bool
	var rejectCalled bool
	var rejectErr error

	validator := func(msg *natsx.Msg) error {
		return errors.New("invalid payload")
	}

	handler := func(m *natsx.Msg) {
		handlerCalled = true
	}

	onReject := func(msg *natsx.Msg, err error) {
		rejectCalled = true
		rejectErr = err
	}

	mw := natsx.Chain(handler, middleware.Validation(validator, onReject))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.invalid"}}
	mw(msg)

	if handlerCalled {
		t.Fatal("handler should NOT have been called")
	}
	if !rejectCalled {
		t.Fatal("onReject should have been called")
	}
	if rejectErr == nil || rejectErr.Error() != "invalid payload" {
		t.Fatalf("unexpected reject error: %v", rejectErr)
	}
}

func TestValidationRejectNoCallback(t *testing.T) {
	var handlerCalled bool

	validator := func(msg *natsx.Msg) error {
		return errors.New("bad")
	}

	handler := func(m *natsx.Msg) {
		handlerCalled = true
	}

	mw := natsx.Chain(handler, middleware.Validation(validator, nil))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.reject.nocb"}}
	mw(msg)

	if handlerCalled {
		t.Fatal("handler should NOT have been called")
	}
}

// ---------- Metrics ----------

type testMetrics struct {
	mu        sync.Mutex
	received  map[string]int
	processed map[string]int
	failed    map[string]int
	durations map[string][]time.Duration
}

func newTestMetrics() *testMetrics {
	return &testMetrics{
		received:  make(map[string]int),
		processed: make(map[string]int),
		failed:    make(map[string]int),
		durations: make(map[string][]time.Duration),
	}
}

func (m *testMetrics) IncMessagesReceived(subject string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.received[subject]++
}
func (m *testMetrics) IncMessagesProcessed(subject string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.processed[subject]++
}
func (m *testMetrics) IncMessagesFailed(subject string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.failed[subject]++
}
func (m *testMetrics) ObserveDuration(subject string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.durations[subject] = append(m.durations[subject], d)
}

func TestMetricsSuccess(t *testing.T) {
	rec := newTestMetrics()

	handler := func(m *natsx.Msg) {}
	mw := natsx.Chain(handler, middleware.Metrics(rec))

	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.metrics"}}
	mw(msg)

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if rec.received["test.metrics"] != 1 {
		t.Fatalf("received=%d, want 1", rec.received["test.metrics"])
	}
	if rec.processed["test.metrics"] != 1 {
		t.Fatalf("processed=%d, want 1", rec.processed["test.metrics"])
	}
	if rec.failed["test.metrics"] != 0 {
		t.Fatalf("failed=%d, want 0", rec.failed["test.metrics"])
	}
	if len(rec.durations["test.metrics"]) != 1 {
		t.Fatalf("duration records=%d, want 1", len(rec.durations["test.metrics"]))
	}
}

func TestMetricsPanicDetection(t *testing.T) {
	rec := newTestMetrics()

	handler := func(m *natsx.Msg) {
		panic("boom")
	}

	// Metrics alone — the panic should propagate through, but metrics are recorded.
	mw := natsx.Chain(handler, middleware.Metrics(rec))

	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.metrics.panic"}}

	func() {
		defer func() { recover() }()
		mw(msg)
	}()

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if rec.received["test.metrics.panic"] != 1 {
		t.Fatalf("received=%d, want 1", rec.received["test.metrics.panic"])
	}
	if rec.failed["test.metrics.panic"] != 1 {
		t.Fatalf("failed=%d, want 1 (panic detected)", rec.failed["test.metrics.panic"])
	}
	if rec.processed["test.metrics.panic"] != 0 {
		t.Fatalf("processed=%d, want 0 (panicked)", rec.processed["test.metrics.panic"])
	}
}

func TestMetricsDurationRecorded(t *testing.T) {
	rec := newTestMetrics()

	handler := func(m *natsx.Msg) {
		time.Sleep(20 * time.Millisecond)
	}

	mw := natsx.Chain(handler, middleware.Metrics(rec))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.metrics.dur"}}
	mw(msg)

	rec.mu.Lock()
	defer rec.mu.Unlock()

	if len(rec.durations["test.metrics.dur"]) != 1 {
		t.Fatal("expected 1 duration record")
	}
	if rec.durations["test.metrics.dur"][0] < 15*time.Millisecond {
		t.Fatalf("expected duration >= 15ms, got %v", rec.durations["test.metrics.dur"][0])
	}
}

// ---------- Tracing ----------

// simpleTracer implements middleware.Tracer for testing.
type simpleTracer struct {
	started  *atomic.Int32
	finished *atomic.Int32
}

func (tr *simpleTracer) StartSpan(ctx context.Context, _ string, _ *natsx.Msg) (context.Context, middleware.SpanContext, func()) {
	tr.started.Add(1)
	sc := middleware.SpanContext{TraceID: "trace-001", SpanID: "span-001"}
	return ctx, sc, func() { tr.finished.Add(1) }
}

func TestTracingMiddleware(t *testing.T) {
	var started, finished atomic.Int32
	tracer := &simpleTracer{started: &started, finished: &finished}

	var capturedSC middleware.SpanContext

	handler := func(m *natsx.Msg) {
		capturedSC = middleware.GetSpanContext(m.Context())
	}

	mw := natsx.Chain(handler, middleware.Tracing(tracer, "test-op"))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.trace"}}
	mw(msg)

	if started.Load() != 1 {
		t.Fatalf("expected 1 span started, got %d", started.Load())
	}
	if finished.Load() != 1 {
		t.Fatalf("expected 1 span finished, got %d", finished.Load())
	}
	if capturedSC.TraceID != "trace-001" {
		t.Fatalf("expected TraceID=trace-001, got %q", capturedSC.TraceID)
	}
	if capturedSC.SpanID != "span-001" {
		t.Fatalf("expected SpanID=span-001, got %q", capturedSC.SpanID)
	}
}

func TestGetSpanContextEmpty(t *testing.T) {
	sc := middleware.GetSpanContext(context.Background())
	if sc.TraceID != "" || sc.SpanID != "" {
		t.Fatalf("expected empty SpanContext, got %+v", sc)
	}
}

func TestTracingFinishCalledOnPanic(t *testing.T) {
	var started, finished atomic.Int32
	tracer := &simpleTracer{started: &started, finished: &finished}

	handler := func(m *natsx.Msg) {
		panic("handler panic")
	}

	mw := natsx.Chain(handler, middleware.Tracing(tracer, "panic-op"))
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.trace.panic"}}

	func() {
		defer func() { recover() }()
		mw(msg)
	}()

	if finished.Load() != 1 {
		t.Fatal("expected finish to be called even on panic")
	}
}
