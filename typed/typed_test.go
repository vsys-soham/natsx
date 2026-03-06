package typed_test

import (
	"context"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natstestsrv "github.com/nats-io/nats-server/v2/test"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/typed"
)

func startTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natstestsrv.DefaultTestOptions
	opts.Port = -1
	s := natstestsrv.RunServer(&opts)
	t.Cleanup(s.Shutdown)
	return s
}

func connectClient(t *testing.T, s *server.Server) *natsx.Client {
	t.Helper()
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	t.Cleanup(c.Close)
	return c
}

type Event struct {
	Kind string `json:"kind"`
	Val  int    `json:"val"`
}

func TestTypedPublishAndSubscribe(t *testing.T) {
	s := startTestServer(t)
	c := connectClient(t, s)

	received := make(chan Event, 1)

	_, err := typed.Subscribe[Event](c, "events.>", func(_ context.Context, subject string, e Event) error {
		received <- e
		return nil
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx := context.Background()
	if err := typed.Publish(c, ctx, "events.click", Event{Kind: "click", Val: 42}); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case e := <-received:
		if e.Kind != "click" || e.Val != 42 {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for typed message")
	}
}

func TestTypedRequest(t *testing.T) {
	s := startTestServer(t)
	c := connectClient(t, s)

	type AddReq struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	type AddResp struct {
		Sum int `json:"sum"`
	}

	// Responder
	typed.Subscribe[AddReq](c, "math.add", func(_ context.Context, _ string, req AddReq) error {
		return nil // we'll use the raw handler for the response
	})

	// Use raw subscribe for the responder since it needs to Respond
	c.Subscribe("math.add", func(msg *natsx.Msg) {
		var req AddReq
		msg.Decode(&req)
		msg.RespondJSON(AddResp{Sum: req.A + req.B})
	})

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	resp, err := typed.Request[AddReq, AddResp](c, ctx, "math.add", AddReq{A: 10, B: 15})
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if resp.Sum != 25 {
		t.Fatalf("expected Sum=25, got %d", resp.Sum)
	}
}

func TestTypedPublishInvalid(t *testing.T) {
	s := startTestServer(t)
	c := connectClient(t, s)

	ctx := context.Background()
	// channels cannot be JSON-marshaled
	err := typed.Publish(c, ctx, "bad.payload", make(chan int))
	if err == nil {
		t.Fatal("expected encoding error")
	}
}

func TestTypedSubscribeDecodeError(t *testing.T) {
	s := startTestServer(t)
	c := connectClient(t, s)

	// Subscribe expecting Event, but we'll send garbage
	received := make(chan struct{}, 1)
	typed.Subscribe[Event](c, "bad.decode", func(_ context.Context, _ string, _ Event) error {
		received <- struct{}{} // should NOT be called
		return nil
	})

	// Publish raw invalid JSON
	c.Publish("bad.decode", []byte("not json at all"))
	c.Flush()
	time.Sleep(200 * time.Millisecond)

	select {
	case <-received:
		t.Fatal("handler should NOT have been called for bad JSON")
	default:
		// expected — handler was skipped
	}
}

func TestTypedQueueSubscribe(t *testing.T) {
	s := startTestServer(t)
	c := connectClient(t, s)

	received := make(chan Event, 5)

	_, err := typed.QueueSubscribe[Event](c, "q.events", "workers", func(_ context.Context, _ string, e Event) error {
		received <- e
		return nil
	})
	if err != nil {
		t.Fatalf("QueueSubscribe: %v", err)
	}

	ctx := context.Background()
	typed.Publish(c, ctx, "q.events", Event{Kind: "q", Val: 1})
	c.Flush()

	select {
	case e := <-received:
		if e.Kind != "q" {
			t.Fatalf("unexpected: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}
