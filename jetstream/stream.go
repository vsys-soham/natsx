package jetstream

import (
	"fmt"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
)

// EnsureStream creates a stream if it doesn't exist, or updates its config if
// it already exists. This is idempotent — safe to call on every startup.
//
// Returns the stream info after creation/update.
func (c *Client) EnsureStream(cfg *nats.StreamConfig) (*nats.StreamInfo, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}

	// Try to get existing stream info.
	info, err := js.StreamInfo(cfg.Name)
	if err == nil {
		// Stream exists — update its config.
		info, err = js.UpdateStream(cfg)
		if err != nil {
			return nil, natsx.WrapError("EnsureStream.Update", err)
		}
		c.log.Info("jetstream stream updated", "stream", cfg.Name)
		return info, nil
	}

	// Stream doesn't exist — create it.
	info, err = js.AddStream(cfg)
	if err != nil {
		return nil, natsx.WrapError("EnsureStream.Create", err)
	}
	c.log.Info("jetstream stream created",
		"stream", cfg.Name,
		"subjects", fmt.Sprintf("%v", cfg.Subjects),
		"storage", cfg.Storage.String(),
	)
	return info, nil
}

// DeleteStream deletes a stream by name.
func (c *Client) DeleteStream(name string) error {
	js, err := c.JetStream()
	if err != nil {
		return err
	}
	if err := js.DeleteStream(name); err != nil {
		return natsx.WrapError("DeleteStream", err)
	}
	c.log.Info("jetstream stream deleted", "stream", name)
	return nil
}

// StreamInfo returns information about a stream.
func (c *Client) StreamInfo(name string) (*nats.StreamInfo, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}
	info, err := js.StreamInfo(name)
	if err != nil {
		return nil, natsx.WrapError("StreamInfo", err)
	}
	return info, nil
}

// EnsureConsumer creates or updates a durable consumer on a stream.
// This is idempotent — safe to call on every startup.
func (c *Client) EnsureConsumer(stream string, cfg *nats.ConsumerConfig) (*nats.ConsumerInfo, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}

	// Try to get existing consumer.
	info, err := js.ConsumerInfo(stream, cfg.Durable)
	if err == nil {
		// Consumer exists — update it.
		info, err = js.UpdateConsumer(stream, cfg)
		if err != nil {
			return nil, natsx.WrapError("EnsureConsumer.Update", err)
		}
		c.log.Info("jetstream consumer updated",
			"stream", stream,
			"consumer", cfg.Durable,
		)
		return info, nil
	}

	// Consumer doesn't exist — create it.
	info, err = js.AddConsumer(stream, cfg)
	if err != nil {
		return nil, natsx.WrapError("EnsureConsumer.Create", err)
	}
	c.log.Info("jetstream consumer created",
		"stream", stream,
		"consumer", cfg.Durable,
	)
	return info, nil
}

// ConsumerInfo returns information about a consumer.
func (c *Client) ConsumerInfo(stream, consumer string) (*nats.ConsumerInfo, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}
	info, err := js.ConsumerInfo(stream, consumer)
	if err != nil {
		return nil, natsx.WrapError("ConsumerInfo", err)
	}
	return info, nil
}
