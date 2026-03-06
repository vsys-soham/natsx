package jetstream

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
)

// PublishIdempotent publishes a message with a deduplication ID via the
// Nats-Msg-Id header. JetStream will deduplicate messages with the same
// msgID within the stream's dedup window.
//
// This is the recommended approach for at-least-once → exactly-once publish.
func (c *Client) PublishIdempotent(subject, msgID string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}

	msg := &nats.Msg{
		Subject: subject,
		Data:    data,
		Header:  nats.Header{},
	}
	msg.Header.Set(nats.MsgIdHdr, msgID)

	ack, err := js.PublishMsg(msg, opts...)
	if err != nil {
		return nil, natsx.WrapError("PublishIdempotent", err)
	}

	if ack.Duplicate {
		c.log.Debug("jetstream duplicate detected",
			"subject", subject,
			"msg_id", msgID,
			"stream", ack.Stream,
		)
	}

	return ack, nil
}

// PublishJSONIdempotent marshals v to JSON and publishes with a dedup ID.
func (c *Client) PublishJSONIdempotent(subject, msgID string, v any, opts ...nats.PubOpt) (*nats.PubAck, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, natsx.WrapError("PublishJSONIdempotent", natsx.ErrEncoding)
	}
	return c.PublishIdempotent(subject, msgID, data, opts...)
}

// PublishAsync publishes a message asynchronously, returning a PubAckFuture.
// Use the future to check for errors and acknowledgments later.
func (c *Client) PublishAsync(subject string, data []byte, opts ...nats.PubOpt) (nats.PubAckFuture, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}

	future, err := js.PublishAsync(subject, data, opts...)
	if err != nil {
		return nil, natsx.WrapError("PublishAsync", err)
	}
	return future, nil
}
