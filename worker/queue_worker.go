// Package worker provides long-running consumer runner abstractions for NATS
// and JetStream. Workers handle graceful shutdown, panic recovery, logging,
// and structured lifecycle management so application code stays focused on
// business logic.
package worker

import (
	"context"
	"sync"

	"github.com/vsys-soham/natsx"
)

// QueueWorkerConfig configures a queue-subscribe consumer worker.
type QueueWorkerConfig struct {
	// Subject is the NATS subject to subscribe to (required).
	Subject string

	// Queue is the queue group name (required).
	Queue string

	// Concurrency is the number of goroutines processing messages concurrently.
	// Defaults to 1.
	Concurrency int

	// Middlewares are applied to each message before the handler.
	Middlewares []natsx.Middleware
}

// QueueWorker is a long-running worker that consumes messages from a NATS
// queue group. It handles graceful shutdown and panic recovery.
type QueueWorker struct {
	client  *natsx.Client
	cfg     QueueWorkerConfig
	log     natsx.Logger
	handler natsx.MsgHandler
}

// NewQueueWorker creates a new QueueWorker.
func NewQueueWorker(client *natsx.Client, cfg QueueWorkerConfig, handler natsx.MsgHandler) *QueueWorker {
	if cfg.Concurrency <= 0 {
		cfg.Concurrency = 1
	}
	return &QueueWorker{
		client:  client,
		cfg:     cfg,
		log:     client.Log(),
		handler: handler,
	}
}

// Run starts the worker and blocks until ctx is cancelled or an error occurs.
// It subscribes to the configured subject/queue group and fans out to
// Concurrency goroutines. Returns nil if ctx was cancelled, or an error
// if the subscription failed.
func (w *QueueWorker) Run(ctx context.Context) error {
	// Build handler chain with panic recovery + user middlewares.
	mws := append(
		[]natsx.Middleware{recoveryMiddleware(w.log)},
		w.cfg.Middlewares...,
	)
	chained := natsx.Chain(w.handler, mws...)

	// Use a buffered channel to fan out to Concurrency goroutines.
	msgCh := make(chan *natsx.Msg, w.cfg.Concurrency*4)

	sub, err := w.client.QueueSubscribe(w.cfg.Subject, w.cfg.Queue, func(msg *natsx.Msg) {
		select {
		case msgCh <- msg:
		case <-ctx.Done():
		}
	})
	if err != nil {
		return err
	}

	w.log.Info("queue worker started",
		"subject", w.cfg.Subject,
		"queue", w.cfg.Queue,
		"concurrency", w.cfg.Concurrency,
	)

	var wg sync.WaitGroup
	for i := 0; i < w.cfg.Concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for msg := range msgCh {
				chained(msg)
			}
		}()
	}

	<-ctx.Done()
	sub.Unsubscribe()
	close(msgCh)
	wg.Wait()

	w.log.Info("queue worker stopped",
		"subject", w.cfg.Subject,
		"queue", w.cfg.Queue,
	)
	return nil
}

// recoveryMiddleware wraps the handler to recover from panics.
func recoveryMiddleware(log natsx.Logger) natsx.Middleware {
	return func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			defer func() {
				if r := recover(); r != nil {
					log.Error("worker handler panic recovered",
						"subject", msg.Subject,
						"panic", r,
					)
				}
			}()
			next(msg)
		}
	}
}
