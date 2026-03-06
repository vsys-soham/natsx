package natsx

import (
	"context"
	"encoding/json"
	"time"

	"github.com/nats-io/nats.go"
)

// Msg wraps a *nats.Msg with convenience helpers and an optional context.
type Msg struct {
	*nats.Msg
	ctx context.Context
}

// Context returns the context associated with this message.
// If no context was set, it returns context.Background().
func (m *Msg) Context() context.Context {
	if m.ctx != nil {
		return m.ctx
	}
	return context.Background()
}

// WithContext returns a shallow copy of the Msg with the given context.
// This is used by middleware to propagate deadlines, cancellation, and values.
func (m *Msg) WithContext(ctx context.Context) *Msg {
	m2 := *m
	m2.ctx = ctx
	return &m2
}

// Decode unmarshals the message data from JSON into v.
func (m *Msg) Decode(v any) error {
	if err := json.Unmarshal(m.Data, v); err != nil {
		return WrapError("Msg.Decode", ErrDecoding)
	}
	return nil
}

// RespondJSON marshals v to JSON and sends it as a reply.
func (m *Msg) RespondJSON(v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return WrapError("Msg.RespondJSON", ErrEncoding)
	}
	return m.Respond(data)
}

// ---------- JetStream ack helpers ----------

// AckSync acknowledges the message and waits for confirmation from the server.
func (m *Msg) AckSync() error {
	if err := m.Msg.AckSync(); err != nil {
		return WrapError("Msg.AckSync", err)
	}
	return nil
}

// NakWithDelay negatively acknowledges the message, requesting redelivery
// after the specified delay. Useful for implementing backoff on retries.
func (m *Msg) NakWithDelay(delay time.Duration) error {
	if err := m.Msg.NakWithDelay(delay); err != nil {
		return WrapError("Msg.NakWithDelay", err)
	}
	return nil
}

// Term terminates the message, telling the server to stop redelivering it.
// Use this for poison messages that should never be retried.
func (m *Msg) Term() error {
	if err := m.Msg.Term(); err != nil {
		return WrapError("Msg.Term", err)
	}
	return nil
}

// InProgress resets the redelivery timer, signaling that the message is
// still being processed. Call this periodically for long-running handlers.
func (m *Msg) InProgress() error {
	if err := m.Msg.InProgress(); err != nil {
		return WrapError("Msg.InProgress", err)
	}
	return nil
}

// Metadata returns JetStream message metadata (stream, consumer, sequence, etc.).
func (m *Msg) Metadata() (*nats.MsgMetadata, error) {
	md, err := m.Msg.Metadata()
	if err != nil {
		return nil, WrapError("Msg.Metadata", err)
	}
	return md, nil
}

// MsgHandler is the handler function signature used for subscriptions.
type MsgHandler func(msg *Msg)
