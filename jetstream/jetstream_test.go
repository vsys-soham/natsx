package jetstream_test

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/nats-io/nats.go"
	"github.com/vsys-soham/natsx"
	"github.com/vsys-soham/natsx/jetstream"
)

// startJetStreamServer starts an in-process NATS server with JetStream enabled.
func startJetStreamServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1
	opts.JetStream = true
	opts.StoreDir = t.TempDir()
	s := natsserver.RunServer(&opts)
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

// ---------- Stream Management ----------

func TestEnsureStreamCreateAndUpdate(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	cfg := &nats.StreamConfig{
		Name:     "TEST_STREAM",
		Subjects: []string{"test.>"},
		Storage:  nats.MemoryStorage,
	}

	// Create
	info, err := js.EnsureStream(cfg)
	if err != nil {
		t.Fatalf("EnsureStream create: %v", err)
	}
	if info.Config.Name != "TEST_STREAM" {
		t.Fatalf("expected stream name TEST_STREAM, got %s", info.Config.Name)
	}

	// Update (idempotent call)
	info2, err := js.EnsureStream(cfg)
	if err != nil {
		t.Fatalf("EnsureStream update: %v", err)
	}
	if info2.Config.Name != "TEST_STREAM" {
		t.Fatalf("expected stream name TEST_STREAM, got %s", info2.Config.Name)
	}
}

func TestStreamInfo(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "INFO_STREAM",
		Subjects: []string{"info.>"},
		Storage:  nats.MemoryStorage,
	})

	info, err := js.StreamInfo("INFO_STREAM")
	if err != nil {
		t.Fatalf("StreamInfo: %v", err)
	}
	if info.Config.Name != "INFO_STREAM" {
		t.Fatalf("wrong stream name: %s", info.Config.Name)
	}
}

func TestDeleteStream(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "DEL_STREAM",
		Subjects: []string{"del.>"},
		Storage:  nats.MemoryStorage,
	})

	if err := js.DeleteStream("DEL_STREAM"); err != nil {
		t.Fatalf("DeleteStream: %v", err)
	}

	// Should fail now
	_, err := js.StreamInfo("DEL_STREAM")
	if err == nil {
		t.Fatal("expected error for deleted stream")
	}
}

// ---------- Consumer Management ----------

func TestEnsureConsumer(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "CON_STREAM",
		Subjects: []string{"con.>"},
		Storage:  nats.MemoryStorage,
	})

	cfg := &nats.ConsumerConfig{
		Durable:   "my-consumer",
		AckPolicy: nats.AckExplicitPolicy,
	}

	info, err := js.EnsureConsumer("CON_STREAM", cfg)
	if err != nil {
		t.Fatalf("EnsureConsumer create: %v", err)
	}
	if info.Config.Durable != "my-consumer" {
		t.Fatalf("expected durable=my-consumer, got %s", info.Config.Durable)
	}

	// Idempotent call
	info2, err := js.EnsureConsumer("CON_STREAM", cfg)
	if err != nil {
		t.Fatalf("EnsureConsumer update: %v", err)
	}
	if info2.Config.Durable != "my-consumer" {
		t.Fatalf("expected durable=my-consumer, got %s", info2.Config.Durable)
	}
}

func TestConsumerInfo(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "CI_STREAM",
		Subjects: []string{"ci.>"},
		Storage:  nats.MemoryStorage,
	})
	js.EnsureConsumer("CI_STREAM", &nats.ConsumerConfig{
		Durable:   "ci-consumer",
		AckPolicy: nats.AckExplicitPolicy,
	})

	info, err := js.ConsumerInfo("CI_STREAM", "ci-consumer")
	if err != nil {
		t.Fatalf("ConsumerInfo: %v", err)
	}
	if info.Config.Durable != "ci-consumer" {
		t.Fatal("wrong consumer")
	}
}

// ---------- Idempotent Publish ----------

func TestPublishIdempotent(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "IDEM_STREAM",
		Subjects: []string{"idem.>"},
		Storage:  nats.MemoryStorage,
	})

	// First publish
	ack1, err := js.PublishIdempotent("idem.test", "msg-001", []byte("hello"))
	if err != nil {
		t.Fatalf("PublishIdempotent 1: %v", err)
	}
	if ack1.Duplicate {
		t.Fatal("first publish should not be duplicate")
	}

	// Same msgID — should be detected as duplicate
	ack2, err := js.PublishIdempotent("idem.test", "msg-001", []byte("hello"))
	if err != nil {
		t.Fatalf("PublishIdempotent 2: %v", err)
	}
	if !ack2.Duplicate {
		t.Fatal("second publish with same msgID should be duplicate")
	}

	// Different msgID — not a duplicate
	ack3, err := js.PublishIdempotent("idem.test", "msg-002", []byte("world"))
	if err != nil {
		t.Fatalf("PublishIdempotent 3: %v", err)
	}
	if ack3.Duplicate {
		t.Fatal("different msgID should not be duplicate")
	}
}

func TestPublishJSONIdempotent(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "JIDEM",
		Subjects: []string{"jidem.>"},
		Storage:  nats.MemoryStorage,
	})

	type Payload struct {
		Name string `json:"name"`
	}

	ack, err := js.PublishJSONIdempotent("jidem.test", "j-001", Payload{Name: "alice"})
	if err != nil {
		t.Fatalf("PublishJSONIdempotent: %v", err)
	}
	if ack.Duplicate {
		t.Fatal("should not be duplicate")
	}
}

// ---------- DLQ ----------

func TestSubscribeWithDLQ(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "DLQ_STREAM",
		Subjects: []string{"dlq_test.>"},
		Storage:  nats.MemoryStorage,
	})

	var handlerCalls atomic.Int32
	var dlqCalled atomic.Int32

	_, err := js.SubscribeWithDLQ("dlq_test.>", func(msg *natsx.Msg) {
		handlerCalls.Add(1)
		// Simulate failure — don't ack (SubscribeWithDLQ handles ack itself)
		panic("processing error")
	}, jetstream.DLQConfig{
		MaxDeliveries: 2,
		DLQSubject:    "my.dlq",
		OnDLQ: func(msg *natsx.Msg, count uint64) {
			dlqCalled.Add(1)
		},
	}, nats.AckWait(200*time.Millisecond))
	if err != nil {
		t.Fatalf("SubscribeWithDLQ: %v", err)
	}

	// Subscribe to the DLQ subject to verify routing
	dlqReceived := make(chan *natsx.Msg, 1)
	c.Subscribe("my.dlq", func(msg *natsx.Msg) {
		dlqReceived <- msg
	})

	// Publish a message
	js.Publish("dlq_test.item", []byte("poison-pill"))

	// Wait for DLQ routing (handler panics → nak → redeliver → hit max → DLQ)
	select {
	case dlqMsg := <-dlqReceived:
		parsed := jetstream.ParseDLQ(dlqMsg)
		if parsed.OriginalSubject != "dlq_test.item" {
			t.Fatalf("expected original subject dlq_test.item, got %q", parsed.OriginalSubject)
		}
		if string(parsed.Data) != "poison-pill" {
			t.Fatalf("expected data 'poison-pill', got %q", string(parsed.Data))
		}
		if parsed.Timestamp == "" {
			t.Fatal("expected DLQ timestamp")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for DLQ message")
	}

	if dlqCalled.Load() == 0 {
		t.Fatal("OnDLQ callback should have been called")
	}
}

// ---------- Publish + Subscribe round-trip ----------

func TestPublishAndSubscribe(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "RT_STREAM",
		Subjects: []string{"rt.>"},
		Storage:  nats.MemoryStorage,
	})

	received := make(chan string, 1)

	_, err := js.Subscribe("rt.>", func(msg *natsx.Msg) {
		received <- string(msg.Data)
		msg.Ack()
	}, nats.Durable("rt-consumer"))
	if err != nil {
		t.Fatalf("Subscribe: %v", err)
	}

	_, err = js.Publish("rt.hello", []byte("world"))
	if err != nil {
		t.Fatalf("Publish: %v", err)
	}

	select {
	case data := <-received:
		if data != "world" {
			t.Fatalf("expected 'world', got %q", data)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestPublishJSON(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "JSON_STREAM",
		Subjects: []string{"json_test.>"},
		Storage:  nats.MemoryStorage,
	})

	type Item struct {
		Name string `json:"name"`
		Qty  int    `json:"qty"`
	}

	received := make(chan Item, 1)

	js.Subscribe("json_test.>", func(msg *natsx.Msg) {
		var item Item
		msg.Decode(&item)
		received <- item
		msg.Ack()
	}, nats.Durable("json-consumer"))

	ack, err := js.PublishJSON("json_test.item", Item{Name: "Widget", Qty: 5})
	if err != nil {
		t.Fatalf("PublishJSON: %v", err)
	}
	if ack.Stream != "JSON_STREAM" {
		t.Fatalf("expected stream=JSON_STREAM, got %s", ack.Stream)
	}

	select {
	case item := <-received:
		if item.Name != "Widget" || item.Qty != 5 {
			t.Fatalf("unexpected item: %+v", item)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out")
	}
}

func TestPullSubscribe(t *testing.T) {
	s := startJetStreamServer(t)
	c := connectClient(t, s)
	js := jetstream.New(c)

	js.EnsureStream(&nats.StreamConfig{
		Name:     "PULL_STREAM",
		Subjects: []string{"pull.>"},
		Storage:  nats.MemoryStorage,
	})

	// Publish first
	for i := 0; i < 3; i++ {
		js.Publish(fmt.Sprintf("pull.item.%d", i), []byte(fmt.Sprintf("msg-%d", i)))
	}

	sub, err := js.PullSubscribe("pull.>", "pull-consumer")
	if err != nil {
		t.Fatalf("PullSubscribe: %v", err)
	}

	msgs, err := sub.Fetch(3, nats.MaxWait(2*time.Second))
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(msgs))
	}

	for _, m := range msgs {
		m.Ack()
	}
}
