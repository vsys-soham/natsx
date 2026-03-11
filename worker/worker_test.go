package worker_test

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/worker"
)

func startServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1
	s := natsserver.RunServer(&opts)
	t.Cleanup(s.Shutdown)
	return s
}

func connectClient(t *testing.T, s *server.Server) *natsx.Client {
	t.Helper()
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

// ---------- QueueWorker ----------

func TestQueueWorker_ProcessesMessages(t *testing.T) {
	s := startServer(t)
	c := connectClient(t, s)

	var count atomic.Int32

	w := worker.NewQueueWorker(c, worker.QueueWorkerConfig{
		Subject:     "test.queue.*",
		Queue:       "my-workers",
		Concurrency: 2,
	}, func(msg *natsx.Msg) {
		count.Add(1)
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	go w.Run(ctx)                     //nolint:errcheck
	time.Sleep(50 * time.Millisecond) // let subscription set up

	// Publish 5 messages
	for i := 0; i < 5; i++ {
		c.Publish("test.queue.item", []byte("hello"))
	}
	c.Flush()

	// Wait for all to be processed
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if count.Load() == 5 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	cancel() // stop worker

	if count.Load() != 5 {
		t.Errorf("expected 5 messages processed, got %d", count.Load())
	}
}

func TestQueueWorker_PanicRecovery(t *testing.T) {
	s := startServer(t)
	c := connectClient(t, s)

	var handlerCalls atomic.Int32

	w := worker.NewQueueWorker(c, worker.QueueWorkerConfig{
		Subject: "panic.test.*",
		Queue:   "panic-workers",
	}, func(msg *natsx.Msg) {
		handlerCalls.Add(1)
		panic("intentional panic")
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- w.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	c.Publish("panic.test.item", []byte("boom"))
	c.Flush()

	time.Sleep(200 * time.Millisecond)
	cancel()

	select {
	case err := <-errCh:
		// Worker should exit cleanly (nil) not crash
		if err != nil {
			t.Errorf("expected nil error, got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker did not stop")
	}

	if handlerCalls.Load() == 0 {
		t.Error("handler should have been called at least once")
	}
}

func TestQueueWorker_GracefulShutdown(t *testing.T) {
	s := startServer(t)
	c := connectClient(t, s)

	w := worker.NewQueueWorker(c, worker.QueueWorkerConfig{
		Subject: "shutdown.test.*",
		Queue:   "shutdown-workers",
	}, func(msg *natsx.Msg) {
		time.Sleep(100 * time.Millisecond) // simulate slow handler
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)

	errCh := make(chan error, 1)
	go func() { errCh <- w.Run(ctx) }()

	time.Sleep(50 * time.Millisecond)
	c.Publish("shutdown.test.item", []byte("msg"))
	c.Flush()
	time.Sleep(20 * time.Millisecond)

	cancel() // trigger shutdown while handler is running

	select {
	case err := <-errCh:
		if err != nil {
			t.Errorf("expected nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("worker did not complete graceful shutdown")
	}
}

func TestQueueWorker_DefaultConcurrency(t *testing.T) {
	s := startServer(t)
	c := connectClient(t, s)

	// Zero concurrency should default to 1 — just verify no panic
	w := worker.NewQueueWorker(c, worker.QueueWorkerConfig{
		Subject:     "default.conc.*",
		Queue:       "dc-workers",
		Concurrency: 0, // should default to 1
	}, func(msg *natsx.Msg) {})

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	if err := w.Run(ctx); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
