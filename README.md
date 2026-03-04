# natsx

A high-level, production-ready Go wrapper around [nats.go](https://github.com/nats-io/nats.go) — **robust, fast, and dead-simple**.

```
go get github.com/vsys-soham/natsx
```

## Features

| Feature | Description |
|---|---|
| **One-liner connect** | Sensible defaults, zero boilerplate |
| **Functional options** | `WithURL`, `WithAuth`, `WithTLS` … extensible without breaking |
| **Pub / Sub / Request** | Raw bytes, JSON, headers — all covered |
| **Queue groups** | Built-in load-balanced subscriptions |
| **JetStream** | First-class publish, subscribe, pull-subscribe via `jetstream` package |
| **Middleware** | Composable handler chain — recovery, logging, custom via `middleware` package |
| **Pluggable logger** | `Logger` interface — works with slog, zap, zerolog |
| **Health checks** | `IsConnected()`, `Status()`, `Stats()` |
| **Graceful shutdown** | `Close()` and `Drain()` |

---

## Package Layout

```
natsx/                      # Core client: connect, publish, subscribe, request
├── middleware/              # Built-in middleware: Recovery, Logging
├── jetstream/               # JetStream helpers: publish, subscribe, pull-subscribe
└── examples/
    ├── basic/main.go        # Core NATS demo
    └── jetstream/main.go    # JetStream demo
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

    "github.com/vsys-soham/natsx"
    "github.com/vsys-soham/natsx/middleware"
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

    // Subscribe with middleware
    c.Subscribe("greet.*", func(msg *natsx.Msg) {
        fmt.Printf("[%s] %s\n", msg.Subject, msg.Data)
    }, middleware.Recovery(c.Log()))

    // Publish
    c.Publish("greet.world", []byte("Hello!"))
    c.PublishJSON("greet.json", map[string]string{"hi": "there"})

    // Request / Reply
    c.Subscribe("echo", func(msg *natsx.Msg) {
        msg.Respond(msg.Data)
    })
    ctx, cancel := context.WithTimeout(context.Background(), time.Second)
    defer cancel()
    reply, _ := c.Request(ctx, "echo", []byte("ping"))
    fmt.Println(string(reply.Data)) // "ping"
}
```

---

## API Cheatsheet

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
natsx.WithURL(url)
natsx.WithName(name)
natsx.WithToken(token)
natsx.WithUserPass(user, pass)
natsx.WithCredsFile(path)
natsx.WithNKeyFile(path)
natsx.WithTLS(tlsConfig)
natsx.WithMaxReconnects(n)
natsx.WithReconnectWait(d)
natsx.WithConnectTimeout(d)
natsx.WithDrainTimeout(d)
natsx.WithPingInterval(d)
natsx.WithLogger(logger)
natsx.WithOnDisconnect(fn)
natsx.WithOnReconnect(fn)
natsx.WithOnClose(fn)
natsx.WithOnError(fn)
```

### Publishing

```go
c.Publish(subject, data)            error
c.PublishJSON(subject, v)           error
c.PublishMsg(msg)                   error
c.PublishWithHeaders(subj, hdr, d)  error
```

### Subscribing

```go
c.Subscribe(subj, handler, mw...)          (*nats.Subscription, error)
c.QueueSubscribe(subj, queue, handler, mw...)  (*nats.Subscription, error)
c.SubscribeChan(subj, ch)                  (*nats.Subscription, error)
```

### Request / Reply

```go
c.Request(ctx, subj, data)             (*Msg, error)
c.RequestJSON(ctx, subj, req, &resp)   error
```

### JetStream (`github.com/vsys-soham/natsx/jetstream`)

```go
js := jetstream.New(c)                       // create from natsx.Client
js.JetStream(opts...)                        (nats.JetStreamContext, error)
js.Publish(subj, data, opts...)              (*nats.PubAck, error)
js.PublishJSON(subj, v, opts...)             (*nats.PubAck, error)
js.Subscribe(subj, handler, opts...)         (*nats.Subscription, error)
js.PullSubscribe(subj, durable, opts...)     (*nats.Subscription, error)
```

### Msg Helpers

```go
msg.Decode(&v)        error   // JSON unmarshal
msg.RespondJSON(v)    error   // JSON reply
msg.Respond(data)     error   // raw reply
```

### Middleware (`github.com/vsys-soham/natsx/middleware`)

```go
// Core (root package)
natsx.Chain(handler, mw1, mw2)         MsgHandler

// Built-in middleware
middleware.Recovery(logger)            Middleware  // catch panics
middleware.Logging(logger)             Middleware  // log messages
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
func Tracing() natsx.Middleware {
    return func(next natsx.MsgHandler) natsx.MsgHandler {
        return func(msg *natsx.Msg) {
            start := time.Now()
            next(msg)
            log.Printf("[%s] %v", msg.Subject, time.Since(start))
        }
    }
}

c.Subscribe("events.>", handler, middleware.Recovery(log), Tracing())
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