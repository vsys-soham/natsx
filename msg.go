package natsx

import (
	"context"
	"encoding/json"

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

// MsgHandler is the handler function signature used for subscriptions.
type MsgHandler func(msg *Msg)
