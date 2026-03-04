package natsx_test

import (
	"testing"
	"time"

	"github.com/vsys-soham/natsx"
)

func TestPublish(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	if err := c.Publish("test.raw", []byte("hello")); err != nil {
		t.Fatalf("Publish: %v", err)
	}
}

func TestPublishJSON(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	if err := c.PublishJSON("test.json", payload{Name: "alice", Age: 30}); err != nil {
		t.Fatalf("PublishJSON: %v", err)
	}
}

func TestPublishJSONInvalid(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	// channels cannot be marshalled to JSON
	err = c.PublishJSON("test.bad", make(chan int))
	if err == nil {
		t.Fatal("expected encoding error, got nil")
	}
}

func TestPublishAndSubscribe(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	received := make(chan string, 1)

	_, err = c.Subscribe("test.pubsub", func(msg *natsx.Msg) {
		received <- string(msg.Data)
	})
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	if err := c.Publish("test.pubsub", []byte("world")); err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case data := <-received:
		if data != "world" {
			t.Fatalf("expected 'world', got %q", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for message")
	}
}
