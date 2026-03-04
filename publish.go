package natsx

import (
	"encoding/json"

	"github.com/nats-io/nats.go"
)

// Publish sends raw bytes to the given subject.
func (c *Client) Publish(subject string, data []byte) error {
	if err := c.nc.Publish(subject, data); err != nil {
		return WrapError("Publish", err)
	}
	return nil
}

// PublishMsg sends a nats.Msg, allowing custom headers.
func (c *Client) PublishMsg(msg *nats.Msg) error {
	if err := c.nc.PublishMsg(msg); err != nil {
		return WrapError("PublishMsg", err)
	}
	return nil
}

// PublishJSON marshals v to JSON and publishes it to the given subject.
func (c *Client) PublishJSON(subject string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return WrapError("PublishJSON", ErrEncoding)
	}
	return c.Publish(subject, data)
}

// PublishWithHeaders sends raw bytes with custom headers to the given subject.
func (c *Client) PublishWithHeaders(subject string, headers nats.Header, data []byte) error {
	msg := &nats.Msg{
		Subject: subject,
		Header:  headers,
		Data:    data,
	}
	return c.PublishMsg(msg)
}
