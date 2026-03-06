# natsx

A high-level, production-ready Go wrapper around [nats.go](https://github.com/nats-io/nats.go) — **robust, fast, and dead-simple**.

```
go get github.com/phoenix/natsx
```

## Features

| Feature | Description |
|---|---|
| **One-liner connect** | Sensible defaults, zero boilerplate |
| **Functional options** | `WithURL`, `WithAuth`, `WithTLS` … extensible without breaking |
| **Pub / Sub / Request** | Raw bytes, JSON, context-aware, headers — all covered |
| **Typed generics** | `Publish[T]`, `Subscribe[T]`, `Request[Req,Resp]` — compile-time type safety |
| **Message envelope** | Optional `Envelope[T]` with ID, timestamp, source, correlation-ID |
| **Queue groups** | Built-in load-balanced subscriptions |
| **JetStream** | Stream/consumer management, idempotent publish, DLQ, ack helpers |
| **Middleware** | Composable chain — recovery, logging, timeout, metrics, tracing, correlation-ID, concurrency limit, validation |
| **Retry / Backoff** | Exponential backoff with jitter, permanent error classification |
| **DLQ** | Dead-letter queue with automatic routing after N failed deliveries |
| **Subject validation** | Catches malformed subjects before they hit the wire |
| **Error classification** | `IsRetryable()` separates transient from permanent failures |
| **Pluggable logger** | `Logger` interface — works with slog, zap, zerolog |
| **Health checks** | `IsConnected()`, `Status()`, `Stats()` |
| **Graceful shutdown** | `Close()` and `Drain()` |

---

## Package Layout

```
natsx/                        # Core client: connect, publish, subscribe, request
├── middleware/               # Built-in middleware (8 middlewares)
├── jetstream/                # JetStream: streams, consumers, idempotent publish, DLQ
├── retry/                    # Retry policies with exponential backoff + jitter
├── typed/                    # Generic type-safe publish/subscribe/request
├── envelope/                 # Optional standard message envelope
└── examples/
    ├── basic/main.go         # Core NATS demo
    └── jetstream/main.go     # JetStream demo
```

---

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"
    "time"

    "github.com/phoenix/natsx"
    "github.com/phoenix/natsx/middleware"
)

func main() {
    // Connect with sensible defaults
    c, err := natsx.Connect(
        natsx.WithURL("nats://localhost:4222"),
        natsx.WithName("my-service"),
    )
    if err != nil {
        log.Fatal(err)
    }
    defer c.Drain()

    // Subscribe with middleware stack
    c.Subscribe("greet.*", func(msg *natsx.Msg) {
        cid := middleware.GetCorrelationID(msg.Context())
        fmt.Printf("[%s] cid=%s %s\n", msg.Subject, cid, msg.Data)
    },
        middleware.Recovery(c.Log()),
        middleware.Timeout(5*time.Second, c.Log()),
        middleware.CorrelationID("", nil),
        middleware.Metrics(middleware.NopMetrics{}),
    )

    // Publish (context-aware)
    ctx := context.Background()
    c.PublishJSONCtx(ctx, "greet.world", map[string]string{"hi": "there"})

    // Request / Reply
    c.Subscribe("echo", func(msg *natsx.Msg) {
        msg.Respond(msg.Data)
    })
    ctx, cancel := context.WithTimeout(ctx, time.Second)
    defer cancel()
    reply, _ := c.Request(ctx, "echo", []byte("ping"))
    fmt.Println(string(reply.Data)) // "ping"
}
```

---

## API Reference

### Connection

```go
c, err := natsx.Connect(opts ...Option)
c.Close()
c.Drain() error
c.Flush() error
c.Conn() *nats.Conn          // escape hatch
c.Log()  Logger              // get client logger
```

### Configuration Options

```go
natsx.WithURL(url)              natsx.WithName(name)
natsx.WithToken(token)          natsx.WithUserPass(user, pass)
natsx.WithCredsFile(path)       natsx.WithNKeyFile(path)
natsx.WithTLS(tlsConfig)        natsx.WithMaxReconnects(n)
natsx.WithReconnectWait(d)      natsx.WithConnectTimeout(d)
natsx.WithDrainTimeout(d)       natsx.WithPingInterval(d)
natsx.WithLogger(logger)        natsx.WithOnDisconnect(fn)
natsx.WithOnReconnect(fn)       natsx.WithOnClose(fn)
natsx.WithOnError(fn)
```

### Publishing

```go
c.Publish(subject, data)                 error   // raw bytes
c.PublishCtx(ctx, subject, data)         error   // context-aware
c.PublishJSON(subject, v)                error   // JSON marshal
c.PublishJSONCtx(ctx, subject, v)        error   // JSON + context
c.PublishMsg(msg)                        error   // custom nats.Msg
c.PublishWithHeaders(subj, hdr, data)    error   // with headers
```

All publish methods validate subjects automatically.

### Subscribing

```go
c.Subscribe(subj, handler, mw...)              (*nats.Subscription, error)
c.QueueSubscribe(subj, queue, handler, mw...)  (*nats.Subscription, error)
c.SubscribeChan(subj, ch)                      (*nats.Subscription, error)
```

### Request / Reply

```go
c.Request(ctx, subj, data)             (*Msg, error)
c.RequestJSON(ctx, subj, req, &resp)   error
```

### Msg Helpers

```go
msg.Decode(&v)              error              // JSON unmarshal
msg.RespondJSON(v)          error              // JSON reply
msg.Respond(data)           error              // raw reply
msg.Context()               context.Context    // get message context
msg.WithContext(ctx)        *Msg               // set message context

// JetStream ack helpers
msg.Ack()                   error              // acknowledge
msg.AckSync()               error              // ack + wait for server confirm
msg.Nak()                   error              // negative ack (redeliver)
msg.NakWithDelay(d)         error              // nak with redelivery delay
msg.Term()                  error              // terminate (stop redelivery)
msg.InProgress()            error              // reset redelivery timer
msg.Metadata()              (*MsgMetadata, error)
```

### Subject Validation

```go
natsx.ValidateSubject(subject) error  // empty, whitespace, bad dots
```

### Error Classification

```go
natsx.IsRetryable(err) bool  // true for timeouts, disconnects; false for encoding, validation
```

### JetStream (`github.com/phoenix/natsx/jetstream`)

```go
js := jetstream.New(c)                       // create from natsx.Client
js.JetStream(opts...)                        (nats.JetStreamContext, error)

// Publish
js.Publish(subj, data, opts...)              (*nats.PubAck, error)
js.PublishJSON(subj, v, opts...)             (*nats.PubAck, error)
js.PublishIdempotent(subj, msgID, data)      (*nats.PubAck, error)  // dedup via Nats-Msg-Id
js.PublishJSONIdempotent(subj, msgID, v)     (*nats.PubAck, error)
js.PublishAsync(subj, data)                  (PubAckFuture, error)

// Subscribe
js.Subscribe(subj, handler, opts...)         (*nats.Subscription, error)
js.PullSubscribe(subj, durable, opts...)     (*nats.Subscription, error)
js.SubscribeWithDLQ(subj, handler, dlqCfg)   (*nats.Subscription, error)

// Stream management (idempotent — safe for startup)
js.EnsureStream(cfg)                         (*nats.StreamInfo, error)
js.EnsureConsumer(stream, cfg)               (*nats.ConsumerInfo, error)
js.DeleteStream(name)                        error
js.StreamInfo(name)                          (*nats.StreamInfo, error)
js.ConsumerInfo(stream, consumer)            (*nats.ConsumerInfo, error)
```

**DLQ (Dead-Letter Queue):**

```go
js.SubscribeWithDLQ("orders.>", handler, jetstream.DLQConfig{
    MaxDeliveries: 3,                  // route to DLQ after 3 failures
    DLQSubject:    "dlq.orders",       // defaults to "dlq.<subject>"
    OnDLQ: func(msg *natsx.Msg, count uint64) { /* callback */ },
})

// Reading from DLQ
dlq := jetstream.ParseDLQ(msg)
// dlq.OriginalSubject, dlq.DeliveryCount, dlq.Timestamp, dlq.Data
```

### Retry (`github.com/phoenix/natsx/retry`)

```go
p := retry.DefaultPolicy()        // 3 attempts, 100ms initial, 2x multiplier, 0.2 jitter
p := retry.NoRetry()              // single attempt

result := retry.Do(ctx, p, func(ctx context.Context, attempt int) error {
    return c.PublishCtx(ctx, "orders.new", data)
})
// result.Attempts, result.Err

// Mark errors as non-retryable
return &retry.Permanent{Err: ErrBadInput}
retry.IsPermanent(err)             // check
retry.ClassifyPermanent(err) bool  // use as Policy.Classifier
```

### Middleware (`github.com/phoenix/natsx/middleware`)

```go
// Chain applies middlewares in order (first = outermost)
natsx.Chain(handler, mw1, mw2)

// Built-in middleware
middleware.Recovery(logger)                         // catch panics
middleware.Logging(logger)                          // log each message
middleware.Timeout(duration, logger)                // deadline + warning
middleware.CorrelationID(header, generateFn)        // extract/inject correlation ID
middleware.ConcurrencyLimit(maxInFlight)             // semaphore limiter
middleware.Validation(validatorFn, onRejectFn)       // pre-handler validation
middleware.Metrics(recorder)                         // counters + histograms
middleware.Tracing(tracer, operationName)             // distributed tracing spans

// Retrieve values set by middleware
middleware.GetCorrelationID(ctx) string
middleware.GetSpanContext(ctx)   SpanContext
```

**Metrics** uses the `MetricsRecorder` interface — implement for Prometheus, OTel, StatsD, etc.:

```go
type MetricsRecorder interface {
    IncMessagesReceived(subject string)
    IncMessagesProcessed(subject string)
    IncMessagesFailed(subject string)
    ObserveDuration(subject string, duration time.Duration)
}
```

**Tracing** uses the `Tracer` interface — implement for OpenTelemetry, Jaeger, etc.:

```go
type Tracer interface {
    StartSpan(ctx context.Context, op string, msg *natsx.Msg) (context.Context, SpanContext, func())
}
```

### Typed Generics (`github.com/phoenix/natsx/typed`)

```go
// Compile-time type safety — no manual json.Marshal/Unmarshal
typed.Publish[Order](c, ctx, "orders.new", order)
typed.Subscribe[Order](c, "orders.>", func(ctx context.Context, subj string, o Order) error {
    return processOrder(o)
})
resp, err := typed.Request[AddReq, AddResp](c, ctx, "math.add", req)
typed.QueueSubscribe[Event](c, "events.>", "workers", handler)
```

### Envelope (`github.com/phoenix/natsx/envelope`)

```go
// Optional metadata wrapper — opt-in, not required
env := envelope.Wrap(order)                                    // auto ID + timestamp
env := envelope.WrapWith(order, "order.created", "svc", cid)   // explicit metadata
data, _ := env.Marshal()
decoded, _ := envelope.Unmarshal[Order](data)
// decoded.ID, decoded.Type, decoded.Source, decoded.CorrelationID, decoded.Timestamp, decoded.Data
```

### Health

```go
c.IsConnected()        bool
c.Status()             ConnectionStatus
c.Stats()              nats.Statistics
c.ConnectedURL()       string
c.ConnectedServerID()  string
```

---

## Custom Middleware

Write your own middleware with the `natsx.Middleware` type:

```go
func RateLimit(rps int) natsx.Middleware {
    limiter := rate.NewLimiter(rate.Limit(rps), rps)
    return func(next natsx.MsgHandler) natsx.MsgHandler {
        return func(msg *natsx.Msg) {
            limiter.Wait(msg.Context())
            next(msg)
        }
    }
}

c.Subscribe("events.>", handler,
    middleware.Recovery(log),
    middleware.Timeout(5*time.Second, log),
    middleware.Metrics(myRecorder),
    RateLimit(100),
)
```

---

## Examples

See [`examples/basic/main.go`](examples/basic/main.go) and
[`examples/jetstream/main.go`](examples/jetstream/main.go) for full runnable demos.

---

## Testing

Tests use an **in-process NATS server** — no external dependencies needed:

```bash
go test -v -race -count=1 ./...
```