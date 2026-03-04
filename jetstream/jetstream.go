// Package jetstream provides a high-level wrapper for NATS JetStream operations.
// It builds on top of the core natsx package, providing publish, subscribe, and
// pull-subscribe helpers with JSON support.
package jetstream

import (
	"encoding/json"
	"sync"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
)

// Client wraps a nats.JetStreamContext with high-level JetStream helpers.
type Client struct {
	nc  *nats.Conn
	log natsx.Logger

	mu sync.Mutex
	js nats.JetStreamContext
}

// New creates a new JetStream Client from a natsx.Client.
// It uses the underlying *nats.Conn and logger from the parent client.
func New(c *natsx.Client) *Client {
	return &Client{
		nc:  c.Conn(),
		log: c.Log(),
	}
}

// JetStream returns a lazily-initialized JetStreamContext.
// It is safe for concurrent use.
func (c *Client) JetStream(opts ...nats.JSOpt) (nats.JetStreamContext, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.js != nil {
		return c.js, nil
	}

	js, err := c.nc.JetStream(opts...)
	if err != nil {
		return nil, natsx.WrapError("JetStream", err)
	}
	c.js = js
	c.log.Info("jetstream context initialized")
	return c.js, nil
}

// Publish publishes a message to a JetStream subject and returns the ack.
func (c *Client) Publish(subject string, data []byte, opts ...nats.PubOpt) (*nats.PubAck, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}
	ack, err := js.Publish(subject, data, opts...)
	if err != nil {
		return nil, natsx.WrapError("JSPublish", err)
	}
	return ack, nil
}

// PublishJSON marshals v to JSON and publishes to a JetStream subject.
func (c *Client) PublishJSON(subject string, v any, opts ...nats.PubOpt) (*nats.PubAck, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return nil, natsx.WrapError("JSPublishJSON", natsx.ErrEncoding)
	}
	return c.Publish(subject, data, opts...)
}

// Subscribe creates a JetStream subscription on the given subject.
func (c *Client) Subscribe(subject string, handler natsx.MsgHandler, opts ...nats.SubOpt) (*nats.Subscription, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}
	sub, err := js.Subscribe(subject, func(m *nats.Msg) {
		handler(&natsx.Msg{Msg: m})
	}, opts...)
	if err != nil {
		return nil, natsx.WrapError("JSSubscribe", err)
	}
	c.log.Debug("jetstream subscribed", "subject", subject)
	return sub, nil
}

// PullSubscribe creates a JetStream pull-based subscription.
func (c *Client) PullSubscribe(subject, durable string, opts ...nats.SubOpt) (*nats.Subscription, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}
	sub, err := js.PullSubscribe(subject, durable, opts...)
	if err != nil {
		return nil, natsx.WrapError("JSPullSubscribe", err)
	}
	c.log.Debug("jetstream pull subscribed", "subject", subject, "durable", durable)
	return sub, nil
}
