package otel

import (
	"context"
	"time"

	"github.com/vsys-soham/natsx/middleware"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
)

// MetricsConfig configures the OpenTelemetry metrics recorder.
type MetricsConfig struct {
	// ServiceName is used as the meter name. Defaults to "natsx".
	ServiceName string

	// MeterProvider is the OTel meter provider. If nil, uses the global provider.
	MeterProvider metric.MeterProvider
}

// MetricsRecorder implements middleware.MetricsRecorder using OpenTelemetry metrics.
type MetricsRecorder struct {
	received  metric.Int64Counter
	processed metric.Int64Counter
	failed    metric.Int64Counter
	duration  metric.Float64Histogram
}

// NewMetricsRecorder creates a new OTel-backed MetricsRecorder.
func NewMetricsRecorder(cfg MetricsConfig) *MetricsRecorder {
	name := cfg.ServiceName
	if name == "" {
		name = "natsx"
	}

	var mp metric.MeterProvider
	if cfg.MeterProvider != nil {
		mp = cfg.MeterProvider
	} else {
		mp = otel.GetMeterProvider()
	}

	meter := mp.Meter(name)

	received, _ := meter.Int64Counter("nats.messages.received",
		metric.WithDescription("Total number of NATS messages received"),
		metric.WithUnit("{message}"),
	)
	processed, _ := meter.Int64Counter("nats.messages.processed",
		metric.WithDescription("Total number of NATS messages successfully processed"),
		metric.WithUnit("{message}"),
	)
	failed, _ := meter.Int64Counter("nats.messages.failed",
		metric.WithDescription("Total number of NATS messages that failed processing"),
		metric.WithUnit("{message}"),
	)
	duration, _ := meter.Float64Histogram("nats.messages.duration",
		metric.WithDescription("Duration of NATS message processing"),
		metric.WithUnit("s"),
	)

	return &MetricsRecorder{
		received:  received,
		processed: processed,
		failed:    failed,
		duration:  duration,
	}
}

func (r *MetricsRecorder) IncMessagesReceived(subject string) {
	r.received.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("nats.subject", subject)),
	)
}

func (r *MetricsRecorder) IncMessagesProcessed(subject string) {
	r.processed.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("nats.subject", subject)),
	)
}

func (r *MetricsRecorder) IncMessagesFailed(subject string) {
	r.failed.Add(context.Background(), 1,
		metric.WithAttributes(attribute.String("nats.subject", subject)),
	)
}

func (r *MetricsRecorder) ObserveDuration(subject string, d time.Duration) {
	r.duration.Record(context.Background(), d.Seconds(),
		metric.WithAttributes(attribute.String("nats.subject", subject)),
	)
}

// compile-time interface check
var _ middleware.MetricsRecorder = (*MetricsRecorder)(nil)
