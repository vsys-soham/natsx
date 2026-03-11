// Package natsxtest provides test helpers for writing integration tests with natsx.
//
// It starts an in-process NATS server (with optional JetStream) so tests have
// zero external dependencies, and provides assertion helpers for common
// publish/subscribe patterns.
//
// Usage:
//
//	func TestMyHandler(t *testing.T) {
//	    env := natsxtest.NewEnv(t)
//	    env.Publish("orders.new", []byte("payload"))
//	    msg := env.WaitForMessage(t, "orders.new", time.Second)
//	    natsxtest.AssertPayload(t, msg, []byte("payload"))
//	}
package natsxtest

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
)

// Env is a test environment with an in-process NATS server and a connected client.
type Env struct {
	// Server is the in-process NATS server.
	Server *server.Server

	// Client is a pre-connected natsx client.
	Client *natsx.Client

	// JS is the raw JetStream context (nil unless started with NewJetStreamEnv).
	JS nats.JetStreamContext
}

// NewEnv starts an in-process NATS server and connects a natsx client.
// The server and client are automatically shut down when the test ends.
func NewEnv(t *testing.T) *Env {
	t.Helper()
	return newEnv(t, false)
}

// NewJetStreamEnv starts an in-process NATS server with JetStream enabled.
func NewJetStreamEnv(t *testing.T) *Env {
	t.Helper()
	return newEnv(t, true)
}

func newEnv(t *testing.T, js bool) *Env {
	t.Helper()

	opts := natsserver.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = js
	if js {
		opts.StoreDir = t.TempDir()
	}

	s := natsserver.RunServer(&opts)
	t.Cleanup(s.Shutdown)

	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("natsxtest.NewEnv: connect: %v", err)
	}
	t.Cleanup(c.Close)

	env := &Env{Server: s, Client: c}

	if js {
		jsc, err := c.Conn().JetStream()
		if err != nil {
			t.Fatalf("natsxtest.NewJetStreamEnv: JetStream: %v", err)
		}
		env.JS = jsc
	}

	return env
}

// Publish publishes raw bytes to the given subject, failing the test on error.
func (e *Env) Publish(t *testing.T, subject string, data []byte) {
	t.Helper()
	if err := e.Client.Publish(subject, data); err != nil {
		t.Fatalf("natsxtest.Publish(%q): %v", subject, err)
	}
}

// PublishJSON publishes v as JSON to subject, failing the test on error.
func (e *Env) PublishJSON(t *testing.T, subject string, v any) {
	t.Helper()
	if err := e.Client.PublishJSON(subject, v); err != nil {
		t.Fatalf("natsxtest.PublishJSON(%q): %v", subject, err)
	}
}

// Subscribe subscribes to subject and returns the first message received within timeout.
// Fails the test if no message arrives within timeout.
func (e *Env) Subscribe(t *testing.T, subject string) <-chan *natsx.Msg {
	t.Helper()
	ch := make(chan *natsx.Msg, 16)
	sub, err := e.Client.Subscribe(subject, func(msg *natsx.Msg) {
		ch <- msg
	})
	if err != nil {
		t.Fatalf("natsxtest.Subscribe(%q): %v", subject, err)
	}
	t.Cleanup(func() { sub.Unsubscribe() })
	return ch
}

// WaitForMessage waits up to timeout for a message on the given subject.
// Subscribes internally and fails the test if no message arrives in time.
func (e *Env) WaitForMessage(t *testing.T, subject string, timeout time.Duration) *natsx.Msg {
	t.Helper()
	ch := e.Subscribe(t, subject)
	select {
	case msg := <-ch:
		return msg
	case <-time.After(timeout):
		t.Fatalf("natsxtest.WaitForMessage(%q): timed out after %v", subject, timeout)
		return nil
	}
}

// WaitForN waits for exactly n messages on subject within timeout.
func (e *Env) WaitForN(t *testing.T, subject string, n int, timeout time.Duration) []*natsx.Msg {
	t.Helper()
	ch := e.Subscribe(t, subject)
	msgs := make([]*natsx.Msg, 0, n)
	deadline := time.After(timeout)
	for len(msgs) < n {
		select {
		case msg := <-ch:
			msgs = append(msgs, msg)
		case <-deadline:
			t.Fatalf("natsxtest.WaitForN(%q, %d): got %d after %v", subject, n, len(msgs), timeout)
		}
	}
	return msgs
}

// Request performs a request/reply and returns the response, failing on error or timeout.
func (e *Env) Request(t *testing.T, subject string, data []byte, timeout time.Duration) *natsx.Msg {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	msg, err := e.Client.Request(ctx, subject, data)
	if err != nil {
		t.Fatalf("natsxtest.Request(%q): %v", subject, err)
	}
	return msg
}

// ---------- Assertion Helpers ----------

// AssertPayload fails the test if msg.Data does not equal expected.
func AssertPayload(t *testing.T, msg *natsx.Msg, expected []byte) {
	t.Helper()
	if string(msg.Data) != string(expected) {
		t.Errorf("payload mismatch:\n  got:  %q\n  want: %q", msg.Data, expected)
	}
}

// AssertSubject fails the test if msg.Subject does not equal expected.
func AssertSubject(t *testing.T, msg *natsx.Msg, expected string) {
	t.Helper()
	if msg.Subject != expected {
		t.Errorf("subject mismatch: got %q, want %q", msg.Subject, expected)
	}
}

// AssertHeader fails the test if the header key does not have the expected value.
func AssertHeader(t *testing.T, msg *natsx.Msg, key, expected string) {
	t.Helper()
	got := msg.Header.Get(key)
	if got != expected {
		t.Errorf("header %q mismatch: got %q, want %q", key, got, expected)
	}
}

// AssertJSONPayload unmarshals msg.Data into v and fails on decode error.
// The caller can then inspect v for further assertions.
func AssertJSONPayload[T any](t *testing.T, msg *natsx.Msg) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(msg.Data, &v); err != nil {
		t.Fatalf("AssertJSONPayload: decode: %v", err)
	}
	return v
}

// AssertNoMessage fails if a message arrives on subject within timeout
// (useful for verifying a message was NOT published).
func AssertNoMessage(t *testing.T, ch <-chan *natsx.Msg, timeout time.Duration) {
	t.Helper()
	select {
	case msg := <-ch:
		t.Errorf("AssertNoMessage: unexpected message on subject %q: %q", msg.Subject, msg.Data)
	case <-time.After(timeout):
		// pass — no message arrived
	}
}
