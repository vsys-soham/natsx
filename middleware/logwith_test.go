package middleware_test

import (
	"context"
	"testing"

	natslib "github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/middleware"
)

// captureLogger captures log calls for inspection.
type captureLogger struct {
	calls []capturedCall
}

type capturedCall struct {
	Level string
	Msg   string
	KV    []any
}

func (l *captureLogger) Debug(msg string, kv ...any) {
	l.calls = append(l.calls, capturedCall{Level: "debug", Msg: msg, KV: kv})
}
func (l *captureLogger) Info(msg string, kv ...any) {
	l.calls = append(l.calls, capturedCall{Level: "info", Msg: msg, KV: kv})
}
func (l *captureLogger) Warn(msg string, kv ...any) {
	l.calls = append(l.calls, capturedCall{Level: "warn", Msg: msg, KV: kv})
}
func (l *captureLogger) Error(msg string, kv ...any) {
	l.calls = append(l.calls, capturedCall{Level: "error", Msg: msg, KV: kv})
}

func hasKV(kv []any, key, value string) bool {
	for i := 0; i+1 < len(kv); i += 2 {
		if k, ok := kv[i].(string); ok && k == key {
			if v, ok := kv[i+1].(string); ok && v == value {
				return true
			}
		}
	}
	return false
}

func TestLogWithSubject(t *testing.T) {
	log := &captureLogger{}
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "orders.new"}}

	enriched := middleware.LogWith(log, msg)
	enriched.Info("processed")

	if len(log.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(log.calls))
	}
	if !hasKV(log.calls[0].KV, "subject", "orders.new") {
		t.Errorf("expected subject=orders.new in KV: %v", log.calls[0].KV)
	}
}

func TestLogWithCorrelationID(t *testing.T) {
	log := &captureLogger{}

	// Set up a correlation ID in the context on the message
	ctx := context.WithValue(context.Background(), correlationIDKeyForTest{}, "corr-abc")
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.sub"}}
	msg = msg.WithContext(ctx)

	// Use middleware chain to inject correlation ID properly
	var capturedMsg *natsx.Msg
	mw := natsx.Chain(func(m *natsx.Msg) {
		capturedMsg = m
	}, middleware.CorrelationID("X-Correlation-ID", nil))

	natsMsg := &natsx.Msg{Msg: &natslib.Msg{
		Subject: "orders.ship",
		Header:  natslib.Header{"X-Correlation-ID": []string{"cid-xyz"}},
	}}
	mw(natsMsg)

	enriched := middleware.LogWith(log, capturedMsg)
	enriched.Warn("slow handler")

	if len(log.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(log.calls))
	}
	if !hasKV(log.calls[0].KV, "correlation_id", "cid-xyz") {
		t.Errorf("expected correlation_id=cid-xyz in KV: %v", log.calls[0].KV)
	}
}

// correlationIDKeyForTest is kept here just for documentation.
type correlationIDKeyForTest struct{}

func TestLogWithNoEnrichmentWhenEmpty(t *testing.T) {
	log := &captureLogger{}
	// Message with no headers, no context values
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "plain.subject"}}

	enriched := middleware.LogWith(log, msg)
	enriched.Debug("raw msg")

	if len(log.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(log.calls))
	}
	// subject should still be present
	if !hasKV(log.calls[0].KV, "subject", "plain.subject") {
		t.Errorf("expected subject in KV: %v", log.calls[0].KV)
	}
	// correlation_id and trace_id should not appear (they'd cause empty-string keys)
	for i := 0; i+1 < len(log.calls[0].KV); i += 2 {
		if k, ok := log.calls[0].KV[i].(string); ok {
			if k == "correlation_id" || k == "trace_id" || k == "span_id" {
				if v, ok := log.calls[0].KV[i+1].(string); ok && v == "" {
					t.Errorf("should not emit empty %s", k)
				}
			}
		}
	}
}

func TestLogWithAllLogLevels(t *testing.T) {
	log := &captureLogger{}
	msg := &natsx.Msg{Msg: &natslib.Msg{Subject: "test.levels"}}
	enriched := middleware.LogWith(log, msg)

	enriched.Debug("d")
	enriched.Info("i")
	enriched.Warn("w")
	enriched.Error("e")

	if len(log.calls) != 4 {
		t.Fatalf("expected 4 calls, got %d", len(log.calls))
	}
	levels := []string{"debug", "info", "warn", "error"}
	for i, l := range levels {
		if log.calls[i].Level != l {
			t.Errorf("call %d: expected level %q, got %q", i, l, log.calls[i].Level)
		}
	}
}
