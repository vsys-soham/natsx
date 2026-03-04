package natsx

import (
	"github.com/nats-io/nats.go"
)

// Subscribe subscribes to a subject with an optional middleware chain.
func (c *Client) Subscribe(subject string, handler MsgHandler, mw ...Middleware) (*nats.Subscription, error) {
	h := Chain(handler, mw...)
	sub, err := c.nc.Subscribe(subject, func(m *nats.Msg) {
		h(&Msg{Msg: m})
	})
	if err != nil {
		return nil, WrapError("Subscribe", err)
	}
	c.log.Debug("subscribed", "subject", subject)
	return sub, nil
}

// QueueSubscribe subscribes to a subject with a queue group and optional middleware.
// Messages are load-balanced across members of the queue group.
func (c *Client) QueueSubscribe(subject, queue string, handler MsgHandler, mw ...Middleware) (*nats.Subscription, error) {
	h := Chain(handler, mw...)
	sub, err := c.nc.QueueSubscribe(subject, queue, func(m *nats.Msg) {
		h(&Msg{Msg: m})
	})
	if err != nil {
		return nil, WrapError("QueueSubscribe", err)
	}
	c.log.Debug("queue subscribed", "subject", subject, "queue", queue)
	return sub, nil
}

// SubscribeChan subscribes and delivers messages on the returned channel.
// The caller is responsible for draining the channel.
func (c *Client) SubscribeChan(subject string, ch chan *nats.Msg) (*nats.Subscription, error) {
	sub, err := c.nc.ChanSubscribe(subject, ch)
	if err != nil {
		return nil, WrapError("SubscribeChan", err)
	}
	c.log.Debug("chan subscribed", "subject", subject)
	return sub, nil
}
