package natsx_test

import (
	"sync/atomic"
	"testing"
	"time"

	"github.com/vsys-soham/natsx"
)

func TestSubscribeAndDecode(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	type event struct {
		Kind string `json:"kind"`
		Val  int    `json:"val"`
	}

	done := make(chan event, 1)
	_, err = c.Subscribe("events.>", func(msg *natsx.Msg) {
		var e event
		if err := msg.Decode(&e); err != nil {
			t.Errorf("Decode: %v", err)
			return
		}
		done <- e
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := c.PublishJSON("events.click", event{Kind: "click", Val: 42}); err != nil {
		t.Fatalf("PublishJSON: %v", err)
	}

	select {
	case e := <-done:
		if e.Kind != "click" || e.Val != 42 {
			t.Fatalf("unexpected event: %+v", e)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestQueueSubscribe(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	var count atomic.Int32

	handler := func(msg *natsx.Msg) {
		count.Add(1)
	}

	// Two queue members on the same queue — only one should get each message.
	_, err = c.QueueSubscribe("work.item", "workers", handler)
	if err != nil {
		t.Fatalf("QueueSubscribe 1: %v", err)
	}
	_, err = c.QueueSubscribe("work.item", "workers", handler)
	if err != nil {
		t.Fatalf("QueueSubscribe 2: %v", err)
	}

	const n = 10
	for i := 0; i < n; i++ {
		if err := c.Publish("work.item", []byte("job")); err != nil {
			t.Fatalf("Publish: %v", err)
		}
	}

	if err := c.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}
	time.Sleep(300 * time.Millisecond)

	// Exactly n messages should be delivered (one per message, not duplicated).
	if got := count.Load(); got != n {
		t.Fatalf("expected %d deliveries, got %d", n, got)
	}
}

func TestSubscribeWithMiddleware(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	var order []string
	done := make(chan struct{}, 1)

	mw1 := func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			order = append(order, "mw1-before")
			next(msg)
			order = append(order, "mw1-after")
		}
	}
	mw2 := func(next natsx.MsgHandler) natsx.MsgHandler {
		return func(msg *natsx.Msg) {
			order = append(order, "mw2-before")
			next(msg)
			order = append(order, "mw2-after")
		}
	}

	_, err = c.Subscribe("mw.test", func(msg *natsx.Msg) {
		order = append(order, "handler")
		done <- struct{}{}
	}, mw1, mw2)
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	c.Publish("mw.test", []byte("x"))
	c.Flush()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}

	expected := []string{"mw1-before", "mw2-before", "handler", "mw2-after", "mw1-after"}
	if len(order) != len(expected) {
		t.Fatalf("expected order %v, got %v", expected, order)
	}
	for i, v := range expected {
		if order[i] != v {
			t.Fatalf("order[%d] = %q, want %q", i, order[i], v)
		}
	}
}
