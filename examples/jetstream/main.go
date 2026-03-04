package main

import (
	"fmt"
	"log"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/jetstream"
)

func main() {
	// ── Connect ─────────────────────────────────────────────
	client, err := natsx.Connect(
		natsx.WithURL("nats://localhost:4222"),
		natsx.WithName("jetstream-example"),
	)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Drain()

	// ── Create JetStream client ──────────────────────────────
	js := jetstream.New(client)

	// Get the underlying JetStreamContext to manage streams
	jsCtx, err := js.JetStream()
	if err != nil {
		log.Fatalf("jetstream: %v", err)
	}

	// Create stream (idempotent — ignores "already exists" error)
	_, err = jsCtx.AddStream(&nats.StreamConfig{
		Name:     "ORDERS",
		Subjects: []string{"orders.>"},
		Storage:  nats.FileStorage,
	})
	if err != nil {
		log.Fatalf("add stream: %v", err)
	}
	fmt.Println("✓ stream ORDERS ready")

	// ── Subscribe (durable consumer) ────────────────────────
	_, err = js.Subscribe("orders.>", func(msg *natsx.Msg) {
		fmt.Printf("  ▶ [%s] %s\n", msg.Subject, string(msg.Data))
		msg.Ack() // acknowledge
	}, nats.Durable("order-processor"))
	if err != nil {
		log.Fatalf("js subscribe: %v", err)
	}

	// ── Publish ─────────────────────────────────────────────
	type Order struct {
		ID   string `json:"id"`
		Item string `json:"item"`
		Qty  int    `json:"qty"`
	}

	orders := []Order{
		{ID: "001", Item: "Widget", Qty: 5},
		{ID: "002", Item: "Gadget", Qty: 2},
		{ID: "003", Item: "Doohickey", Qty: 10},
	}

	for _, o := range orders {
		ack, err := js.PublishJSON(fmt.Sprintf("orders.%s", o.ID), o)
		if err != nil {
			log.Printf("publish error: %v", err)
			continue
		}
		fmt.Printf("  ▶ published order %s  (stream=%s, seq=%d)\n", o.ID, ack.Stream, ack.Sequence)
	}

	time.Sleep(500 * time.Millisecond)
	fmt.Println("✓ done")
}
