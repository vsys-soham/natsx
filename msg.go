package natsx

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

// Msg wraps a *nats.Msg with convenience helpers.
type Msg struct {
	*nats.Msg
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
