package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/middleware"
)

func main() {
	// ── Connect ─────────────────────────────────────────────
	client, err := natsx.Connect(
		natsx.WithURL("nats://localhost:4222"),
		natsx.WithName("basic-example"),
		natsx.WithMaxReconnects(-1),
		natsx.WithOnDisconnect(func(err error) {
			log.Printf("disconnected: %v", err)
		}),
		natsx.WithOnReconnect(func() {
			log.Println("reconnected!")
		}),
	)
	if err != nil {
		log.Fatalf("connect: %v", err)
	}
	defer client.Drain()

	fmt.Printf("✓ connected to %s  (status: %s)\n", client.ConnectedURL(), client.Status())

	// ── Subscribe ───────────────────────────────────────────
	_, err = client.Subscribe("greet.*", func(msg *natsx.Msg) {
		fmt.Printf("  ▶ [%s] %s\n", msg.Subject, string(msg.Data))
	}, middleware.Recovery(natsx.NopLogger{})) // middleware: catch panics
	if err != nil {
		log.Fatalf("subscribe: %v", err)
	}

	// ── Publish ─────────────────────────────────────────────
	client.Publish("greet.world", []byte("Hello, NATS!"))
	client.PublishJSON("greet.json", map[string]string{"msg": "hi from JSON"})
	client.Flush()
	time.Sleep(100 * time.Millisecond) // let subscriber process

	// ── Request / Reply ─────────────────────────────────────
	client.Subscribe("math.double", func(msg *natsx.Msg) {
		type req struct{ N int }
		type resp struct{ Result int }
		var r req
		msg.Decode(&r)
		msg.RespondJSON(resp{Result: r.N * 2})
	})

	type DoubleReq struct{ N int }
	type DoubleResp struct{ Result int }

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	var result DoubleResp
	if err := client.RequestJSON(ctx, "math.double", DoubleReq{N: 21}, &result); err != nil {
		log.Fatalf("request: %v", err)
	}
	fmt.Printf("  ▶ 21 × 2 = %d\n", result.Result)

	// ── Queue Subscribe (load balancing) ────────────────────
	for i := 0; i < 3; i++ {
		id := i
		client.QueueSubscribe("tasks.process", "workers", func(msg *natsx.Msg) {
			fmt.Printf("  ▶ worker-%d got: %s\n", id, string(msg.Data))
		})
	}
	for i := 0; i < 5; i++ {
		client.Publish("tasks.process", []byte(fmt.Sprintf("job-%d", i)))
	}
	client.Flush()
	time.Sleep(200 * time.Millisecond)

	fmt.Println("✓ done")
}
