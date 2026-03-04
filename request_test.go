package natsx_test

import (
	"context"
	"testing"
	"time"

	"github.com/vsys-soham/natsx"
)

func TestRequestReply(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// Responder
	_, err = c.Subscribe("echo", func(msg *natsx.Msg) {
		msg.Respond(msg.Data)
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	reply, err := c.Request(ctx, "echo", []byte("ping"))
	if err != nil {
		t.Fatalf("Request: %v", err)
	}
	if string(reply.Data) != "ping" {
		t.Fatalf("expected 'ping', got %q", string(reply.Data))
	}
}

func TestRequestJSON(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	type Req struct {
		A int `json:"a"`
		B int `json:"b"`
	}
	type Resp struct {
		Sum int `json:"sum"`
	}

	// Responder: adds A + B
	_, err = c.Subscribe("math.add", func(msg *natsx.Msg) {
		var r Req
		if err := msg.Decode(&r); err != nil {
			t.Errorf("Decode: %v", err)
			return
		}
		msg.RespondJSON(Resp{Sum: r.A + r.B})
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	var resp Resp
	if err := c.RequestJSON(ctx, "math.add", Req{A: 3, B: 4}, &resp); err != nil {
		t.Fatalf("RequestJSON: %v", err)
	}
	if resp.Sum != 7 {
		t.Fatalf("expected sum=7, got %d", resp.Sum)
	}
}

func TestRequestTimeout(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// No responder — should time out
	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()

	_, err = c.Request(ctx, "nobody.home", []byte("hello"))
	if err == nil {
		t.Fatal("expected timeout error, got nil")
	}
}
