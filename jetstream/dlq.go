package jetstream

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
)

// DLQConfig configures dead-letter-queue behavior for a subscription.
type DLQConfig struct {
	// MaxDeliveries is the number of delivery attempts before routing to the DLQ.
	// Default: 3.
	MaxDeliveries int

	// DLQSubject is the subject to publish failed messages to.
	// If empty, defaults to "dlq.<original-subject>".
	DLQSubject string

	// OnDLQ is an optional callback invoked when a message is sent to the DLQ.
	OnDLQ func(msg *natsx.Msg, deliveryCount uint64)
}

func (cfg DLQConfig) maxDeliveries() int {
	if cfg.MaxDeliveries <= 0 {
		return 3
	}
	return cfg.MaxDeliveries
}

func (cfg DLQConfig) dlqSubject(originalSubject string) string {
	if cfg.DLQSubject != "" {
		return cfg.DLQSubject
	}
	return "dlq." + originalSubject
}

// SubscribeWithDLQ creates a JetStream subscription that routes messages to a
// dead-letter-queue after MaxDeliveries failed attempts.
//
// On each delivery, it checks the delivery count from JetStream metadata:
//   - If deliveries < MaxDeliveries: call the handler. If handler returns,
//     the message is acked. If the handler panics, the message is nak'd.
//   - If deliveries >= MaxDeliveries: the message is published to the DLQ
//     subject, then term'd (stop redelivery).
func (c *Client) SubscribeWithDLQ(subject string, handler natsx.MsgHandler, dlqCfg DLQConfig, opts ...nats.SubOpt) (*nats.Subscription, error) {
	js, err := c.JetStream()
	if err != nil {
		return nil, err
	}

	maxDel := dlqCfg.maxDeliveries()

	sub, err := js.Subscribe(subject, func(m *nats.Msg) {
		msg := &natsx.Msg{Msg: m}

		// Get delivery count from JetStream metadata.
		md, err := m.Metadata()
		if err != nil {
			// Can't get metadata — treat as normal message.
			c.handleWithRecovery(msg, handler)
			return
		}

		deliveryCount := md.NumDelivered

		if deliveryCount >= uint64(maxDel) {
			// Route to DLQ.
			c.routeToDLQ(msg, dlqCfg, deliveryCount)
			return
		}

		// Normal processing with panic recovery.
		c.handleWithRecovery(msg, handler)
	}, opts...)

	if err != nil {
		return nil, natsx.WrapError("SubscribeWithDLQ", err)
	}
	c.log.Debug("jetstream subscribed with DLQ",
		"subject", subject,
		"max_deliveries", maxDel,
		"dlq_subject", dlqCfg.dlqSubject(subject),
	)
	return sub, nil
}

// handleWithRecovery runs the handler and catches panics
func (c *Client) handleWithRecovery(msg *natsx.Msg, handler natsx.MsgHandler) {
	defer func() {
		if r := recover(); r != nil {
			c.log.Error("jetstream handler panic",
				"subject", msg.Subject,
				"panic", fmt.Sprintf("%v", r),
			)
			// Nak so it gets redelivered.
			msg.Nak()
		}
	}()

	handler(msg)
	msg.Ack()
}

// routeToDLQ publishes message data to the DLQ subject and terminates the original.
func (c *Client) routeToDLQ(msg *natsx.Msg, cfg DLQConfig, deliveryCount uint64) {
	dlqSubject := cfg.dlqSubject(msg.Subject)

	// Build DLQ message with original metadata in headers.
	dlqMsg := &nats.Msg{
		Subject: dlqSubject,
		Data:    msg.Data,
		Header: nats.Header{
			"X-DLQ-Original-Subject": []string{msg.Subject},
			"X-DLQ-Delivery-Count":   []string{fmt.Sprintf("%d", deliveryCount)},
			"X-DLQ-Timestamp":        []string{time.Now().UTC().Format(time.RFC3339)},
		},
	}

	// Copy original headers.
	for k, v := range msg.Header {
		if _, exists := dlqMsg.Header[k]; !exists {
			dlqMsg.Header[k] = v
		}
	}

	// Publish to DLQ via core NATS (not JetStream — DLQ may not be a JS stream).
	if err := c.nc.PublishMsg(dlqMsg); err != nil {
		c.log.Error("failed to publish to DLQ",
			"dlq_subject", dlqSubject,
			"original_subject", msg.Subject,
			"error", err,
		)
	} else {
		c.log.Warn("message routed to DLQ",
			"dlq_subject", dlqSubject,
			"original_subject", msg.Subject,
			"delivery_count", deliveryCount,
		)
	}

	// Callback
	if cfg.OnDLQ != nil {
		cfg.OnDLQ(msg, deliveryCount)
	}

	// Terminate — stop redelivery.
	msg.Term()
}

// DLQMessage represents a message read from a dead-letter queue.
// It includes the original subject and delivery count metadata.
type DLQMessage struct {
	OriginalSubject string
	DeliveryCount   string
	Timestamp       string
	Data            []byte
	Headers         nats.Header
}

// ParseDLQ extracts DLQ metadata from a message that was routed to a DLQ.
func ParseDLQ(msg *natsx.Msg) DLQMessage {
	return DLQMessage{
		OriginalSubject: msg.Header.Get("X-DLQ-Original-Subject"),
		DeliveryCount:   msg.Header.Get("X-DLQ-Delivery-Count"),
		Timestamp:       msg.Header.Get("X-DLQ-Timestamp"),
		Data:            msg.Data,
		Headers:         msg.Header,
	}
}

// MustDecodeJSON is a convenience for DLQ processors to decode the payload.
func (d DLQMessage) MustDecodeJSON(v any) error {
	if err := json.Unmarshal(d.Data, v); err != nil {
		return natsx.WrapError("DLQMessage.Decode", natsx.ErrDecoding)
	}
	return nil
}
