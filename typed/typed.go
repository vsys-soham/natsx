// Package typed provides generic, type-safe NATS publish/subscribe/request
// helpers that eliminate the need for manual JSON marshal/unmarshal boilerplate.
//
// All functions are thin wrappers over natsx.Client — they add compile-time type
// safety without introducing new connection or subscription management.
package typed

import (
	"context"
	"encoding/json"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
)

// Publish marshals v of type T to JSON and publishes it to the given subject.
func Publish[T any](c *natsx.Client, ctx context.Context, subject string, v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return natsx.WrapError("typed.Publish", natsx.ErrEncoding)
	}
	return c.PublishCtx(ctx, subject, data)
}

// PublishWithHeaders marshals v of type T to JSON and publishes with headers.
func PublishWithHeaders[T any](c *natsx.Client, subject string, headers nats.Header, v T) error {
	data, err := json.Marshal(v)
	if err != nil {
		return natsx.WrapError("typed.PublishWithHeaders", natsx.ErrEncoding)
	}
	return c.PublishWithHeaders(subject, headers, data)
}

// Handler is a typed message handler that receives a decoded value of type T.
// Return an error to signal processing failure (useful for middleware/logging).
type Handler[T any] func(ctx context.Context, subject string, v T) error

// Subscribe subscribes to a subject and decodes each message as type T before
// calling the handler. Decoding errors are logged and the message is skipped.
func Subscribe[T any](c *natsx.Client, subject string, handler Handler[T], mw ...natsx.Middleware) (*nats.Subscription, error) {
	rawHandler := func(msg *natsx.Msg) {
		var v T
		if err := json.Unmarshal(msg.Data, &v); err != nil {
			c.Log().Error("typed.Subscribe decode error",
				"subject", msg.Subject,
				"error", err,
			)
			return
		}
		if err := handler(msg.Context(), msg.Subject, v); err != nil {
			c.Log().Error("typed.Subscribe handler error",
				"subject", msg.Subject,
				"error", err,
			)
		}
	}
	return c.Subscribe(subject, rawHandler, mw...)
}

// QueueSubscribe subscribes to a subject with a queue group and decodes each
// message as type T.
func QueueSubscribe[T any](c *natsx.Client, subject, queue string, handler Handler[T], mw ...natsx.Middleware) (*nats.Subscription, error) {
	rawHandler := func(msg *natsx.Msg) {
		var v T
		if err := json.Unmarshal(msg.Data, &v); err != nil {
			c.Log().Error("typed.QueueSubscribe decode error",
				"subject", msg.Subject,
				"error", err,
			)
			return
		}
		if err := handler(msg.Context(), msg.Subject, v); err != nil {
			c.Log().Error("typed.QueueSubscribe handler error",
				"subject", msg.Subject,
				"error", err,
			)
		}
	}
	return c.QueueSubscribe(subject, queue, rawHandler, mw...)
}

// Request sends a typed request and decodes the reply into type Resp.
func Request[Req, Resp any](c *natsx.Client, ctx context.Context, subject string, req Req) (Resp, error) {
	var zero Resp

	data, err := json.Marshal(req)
	if err != nil {
		return zero, natsx.WrapError("typed.Request", natsx.ErrEncoding)
	}

	reply, err := c.Request(ctx, subject, data)
	if err != nil {
		return zero, err
	}

	var resp Resp
	if err := json.Unmarshal(reply.Data, &resp); err != nil {
		return zero, natsx.WrapError("typed.Request", natsx.ErrDecoding)
	}
	return resp, nil
}
