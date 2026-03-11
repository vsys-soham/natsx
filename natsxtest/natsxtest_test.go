package natsxtest_test

import (
	"testing"
	"time"

	natslib "github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/natsxtest"
)

func TestNewEnv(t *testing.T) {
	env := natsxtest.NewEnv(t)
	if env.Server == nil {
		t.Fatal("expected non-nil Server")
	}
	if env.Client == nil {
		t.Fatal("expected non-nil Client")
	}
	if !env.Client.IsConnected() {
		t.Fatal("expected client to be connected")
	}
}

func TestNewJetStreamEnv(t *testing.T) {
	env := natsxtest.NewJetStreamEnv(t)
	if env.JS == nil {
		t.Fatal("expected non-nil JetStream context")
	}
	if env.Client == nil {
		t.Fatal("expected non-nil Client")
	}
}

func TestPublishAndSubscribe(t *testing.T) {
	env := natsxtest.NewEnv(t)

	ch := env.Subscribe(t, "test.pub")
	env.Publish(t, "test.pub", []byte("hello-world"))

	select {
	case msg := <-ch:
		natsxtest.AssertSubject(t, msg, "test.pub")
		natsxtest.AssertPayload(t, msg, []byte("hello-world"))
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestWaitForMessage(t *testing.T) {
	env := natsxtest.NewEnv(t)

	go func() {
		time.Sleep(30 * time.Millisecond)
		env.Publish(t, "delayed.msg", []byte("eventual"))
	}()

	msg := env.WaitForMessage(t, "delayed.msg", time.Second)
	natsxtest.AssertPayload(t, msg, []byte("eventual"))
}

func TestWaitForN(t *testing.T) {
	env := natsxtest.NewEnv(t)

	ch := env.Subscribe(t, "batch.>")

	for i := 0; i < 5; i++ {
		env.Publish(t, "batch.item", []byte("x"))
	}

	// consume 5 from the already-open channel
	msgs := make([]*natsx.Msg, 0, 5)
	deadline := time.After(2 * time.Second)
	for len(msgs) < 5 {
		select {
		case m := <-ch:
			msgs = append(msgs, m)
		case <-deadline:
			t.Fatalf("timeout: got %d/5 messages", len(msgs))
		}
	}
	if len(msgs) != 5 {
		t.Fatalf("expected 5 messages, got %d", len(msgs))
	}
}

func TestPublishJSON(t *testing.T) {
	type Item struct {
		Name string `json:"name"`
		Qty  int    `json:"qty"`
	}

	env := natsxtest.NewEnv(t)
	ch := env.Subscribe(t, "json.test")
	env.PublishJSON(t, "json.test", Item{Name: "widget", Qty: 3})

	select {
	case msg := <-ch:
		item := natsxtest.AssertJSONPayload[Item](t, msg)
		if item.Name != "widget" || item.Qty != 3 {
			t.Errorf("unexpected item: %+v", item)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestAssertHeader(t *testing.T) {
	env := natsxtest.NewEnv(t)
	ch := env.Subscribe(t, "hdr.test")

	// Publish via the raw nats.Conn to set a custom header
	m := natslib.NewMsg("hdr.test")
	m.Header.Set("X-Request-ID", "req-42")
	m.Data = []byte("body")
	if err := env.Client.Conn().PublishMsg(m); err != nil {
		t.Fatalf("PublishMsg: %v", err)
	}

	select {
	case msg := <-ch:
		natsxtest.AssertHeader(t, msg, "X-Request-ID", "req-42")
	case <-time.After(time.Second):
		t.Fatal("timed out")
	}
}

func TestAssertNoMessage(t *testing.T) {
	env := natsxtest.NewEnv(t)
	ch := env.Subscribe(t, "silent.subject")
	// Nothing published — should pass
	natsxtest.AssertNoMessage(t, ch, 100*time.Millisecond)
}

func TestRequest(t *testing.T) {
	env := natsxtest.NewEnv(t)

	// Set up a responder
	env.Client.Subscribe("echo", func(msg *natsx.Msg) {
		msg.Respond(msg.Data)
	})

	reply := env.Request(t, "echo", []byte("ping"), time.Second)
	natsxtest.AssertPayload(t, reply, []byte("ping"))
}

func TestMultipleAssertions(t *testing.T) {
	env := natsxtest.NewEnv(t)

	type Order struct {
		ID     string `json:"id"`
		Amount int    `json:"amount"`
	}

	ch := env.Subscribe(t, "orders.new")
	env.PublishJSON(t, "orders.new", Order{ID: "ord-1", Amount: 100})

	msg := <-ch
	natsxtest.AssertSubject(t, msg, "orders.new")

	order := natsxtest.AssertJSONPayload[Order](t, msg)
	if order.ID != "ord-1" {
		t.Errorf("expected ID=ord-1, got %q", order.ID)
	}
	if order.Amount != 100 {
		t.Errorf("expected Amount=100, got %d", order.Amount)
	}
}
