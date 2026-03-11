package worker

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/jetstream"
)

// JetStreamWorkerConfig configures a JetStream push-subscribe consumer worker.
type JetStreamWorkerConfig struct {
	// Stream is the JetStream stream name (required).
	Stream string

	// Subject is the subject filter for the subscription (required).
	Subject string

	// Durable is the durable consumer name (required for persistent consumers).
	Durable string

	// Concurrency is the number of goroutines processing messages concurrently.
	// Defaults to 1.
	Concurrency int

	// MaxDeliveries is the number of delivery attempts before routing to DLQ.
	// Set to 0 to disable DLQ routing (unlimited retries via Nak).
	MaxDeliveries int

	// DLQSubject is the subject to route failed messages to.
	// Defaults to "dlq.<Subject>" if MaxDeliveries > 0.
	DLQSubject string

	// AckWait is how long JetStream waits before redelivering an unacked message.
	// Defaults to 30s.
	AckWait time.Duration

	// Middlewares are applied to each message before the handler.
	Middlewares []natsx.Middleware
}

// JetStreamHandler is a JetStream message handler that returns an error.
// Return nil to ack, return an error to nak (with exponential backoff delay),
// return ErrTerminate to term (stop redelivery permanently).
type JetStreamHandler func(ctx context.Context, msg *natsx.Msg) error

// ErrTerminate signals that a message should be terminated (never redelivered).
var ErrTerminate = fmt.Errorf("worker: message terminated")

// JetStreamWorker is a long-running worker that consumes messages from a
// JetStream push-subscription with explicit ack semantics.
type JetStreamWorker struct {
	client  *natsx.Client
	js      *jetstream.Client
	cfg     JetStreamWorkerConfig
	log     natsx.Logger
	handler JetStreamHandler
}

// NewJetStreamWorker creates a new JetStreamWorker.
func NewJetStreamWorker(client *natsx.Client, cfg JetStreamWorkerConfig, handler JetStreamHandler) *JetStreamWorker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	if cfg.AckWait <= 0 {
		cfg.AckWait = 30 * time.Second
	}
	return &JetStreamWorker{
		client:  client,
		js:      jetstream.New(client),
		cfg:     cfg,
		log:     client.Log(),
		handler: handler,
	}
}

// Run starts the JetStream worker and blocks until ctx is cancelled.
// Messages are acked on success, nak'd with exponential backoff on error,
// or term'd when ErrTerminate is returned.
func (w *JetStreamWorker) Run(ctx context.Context) error {
	subOpts := []nats.SubOpt{
		nats.Durable(w.cfg.Durable),
		nats.AckWait(w.cfg.AckWait),
		nats.AckExplicit(),
	}
	if w.cfg.MaxDeliveries > 0 {
		subOpts = append(subOpts, nats.MaxDeliver(w.cfg.MaxDeliveries))
	}

	msgCh := make(chan *natsx.Msg, w.cfg.Concurrency*4)

	var rawSub *nats.Subscription
	var err error

	if w.cfg.MaxDeliveries > 0 {
		dlqCfg := jetstream.DLQConfig{
			MaxDeliveries: w.cfg.MaxDeliveries,
			DLQSubject:    w.cfg.DLQSubject,
			OnDLQ: func(msg *natsx.Msg, count uint64) {
				w.log.Warn("message routed to DLQ",
					"subject", msg.Subject,
					"deliveries", count,
				)
			},
		}
		rawSub, err = w.js.SubscribeWithDLQ(w.cfg.Subject, func(msg *natsx.Msg) {
			select {
			case msgCh <- msg:
			case <-ctx.Done():
			}
		}, dlqCfg, subOpts...)
	} else {
		rawSub, err = w.js.Subscribe(w.cfg.Subject, func(msg *natsx.Msg) {
			select {
			case msgCh <- msg:
			case <-ctx.Done():
			}
		}, subOpts...)
	}

	if err != nil {
		return fmt.Errorf("JetStreamWorker subscribe: %w", err)
	}

	w.log.Info("jetstream worker started",
		"stream", w.cfg.Stream,
		"subject", w.cfg.Subject,
		"durable", w.cfg.Durable,
		"concurrency", w.cfg.Concurrency,
	)

	var wg sync.WaitGroup
	for i := 0; i < w.cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range msgCh {
				w.processMsg(ctx, msg)
			}
		}()
	}

	<-ctx.Done()
	rawSub.Unsubscribe()
	close(msgCh)
	wg.Wait()

	w.log.Info("jetstream worker stopped",
		"subject", w.cfg.Subject,
		"durable", w.cfg.Durable,
	)
	return nil
}

// processMsg runs the handler, then acks/naks/terms based on result.
func (w *JetStreamWorker) processMsg(ctx context.Context, msg *natsx.Msg) {
	defer func() {
		if r := recover(); r != nil {
			w.log.Error("jetstream worker panic",
				"subject", msg.Subject,
				"panic", r,
			)
			// Nak with backoff delay on panic
			msg.NakWithDelay(5 * time.Second)
		}
	}()

	// Get delivery count for backoff
	md, _ := msg.Metadata()

	err := w.handler(ctx, msg)

	switch {
	case err == nil:
		msg.AckSync()

	case err == ErrTerminate:
		w.log.Warn("handler requested termination",
			"subject", msg.Subject,
		)
		msg.Term()

	default:
		// Exponential backoff: delay = 2^(deliveries-1) seconds, capped at 60s
		delay := time.Second
		if md != nil && md.NumDelivered > 0 {
			delay = time.Duration(1<<(md.NumDelivered-1)) * time.Second
			if delay > 60*time.Second {
				delay = 60 * time.Second
			}
		}
		w.log.Warn("handler error, requeuing with delay",
			"subject", msg.Subject,
			"error", err,
			"delay", delay,
		)
		msg.NakWithDelay(delay)
	}
}
