package natsx

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
)

// Publish sends raw bytes to the given subject.
func (c *Client) Publish(subject string, data []byte) error {
	if err := ValidateSubject(subject); err != nil {
		return WrapError("Publish", err)
	}
	if err := c.nc.Publish(subject, data); err != nil {
		return WrapError("Publish", err)
	}
	return nil
}

// PublishCtx sends raw bytes to the given subject, respecting context
// cancellation and deadline. If the context expires before the publish
// completes, an error is returned.
func (c *Client) PublishCtx(ctx context.Context, subject string, data []byte) error {
	if err := ctx.Err(); err != nil {
		return WrapError("PublishCtx", err)
	}
	if err := ValidateSubject(subject); err != nil {
		return WrapError("PublishCtx", err)
	}
	if err := c.nc.Publish(subject, data); err != nil {
		return WrapError("PublishCtx", err)
	}
	return nil
}

// PublishMsg sends a nats.Msg, allowing custom headers.
func (c *Client) PublishMsg(msg *nats.Msg) error {
	if msg.Subject != "" {
		if err := ValidateSubject(msg.Subject); err != nil {
			return WrapError("PublishMsg", err)
		}
	}
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

// PublishJSONCtx marshals v to JSON and publishes it to the given subject,
// respecting context cancellation and deadline.
func (c *Client) PublishJSONCtx(ctx context.Context, subject string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return WrapError("PublishJSONCtx", ErrEncoding)
	}
	return c.PublishCtx(ctx, subject, data)
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
