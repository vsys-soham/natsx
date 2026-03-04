package natsx

import (
	"crypto/tls"
	"time"

	"github.com/nats-io/nats.go"
)

// Config holds all configuration for a Client connection.
type Config struct {
	// Connection
	URL  string // NATS server URL (default: nats://localhost:4222)
	Name string // client connection name for monitoring

	// Authentication
	Token     string
	User      string
	Password  string
	CredsFile string // path to a .creds file (JWT + NKey)
	NKeyFile  string // path to an NKey seed file

	// TLS
	TLSConfig *tls.Config

	// Reconnection
	MaxReconnects    int           // -1 = unlimited (default)
	ReconnectWait    time.Duration // delay between attempts (default: 2s)
	ReconnectJitter  time.Duration // max random jitter added (default: 100ms)
	ReconnectBufSize int           // bytes buffered during reconnect (default: 8MB)

	// Timeouts
	ConnectTimeout time.Duration // dial timeout (default: 5s)
	DrainTimeout   time.Duration // max time to drain subs on close (default: 30s)
	FlusherTimeout time.Duration // flush timeout (default: 1s)
	PingInterval   time.Duration // interval for server pings (default: 2m)

	// Callbacks
	OnDisconnect func(err error)
	OnReconnect  func()
	OnClose      func()
	OnError      func(sub *nats.Subscription, err error)

	// Logging
	Logger Logger
}

// Option is a functional option for configuring a Client.
type Option func(*Config)

// DefaultConfig returns a Config with production-safe defaults.
func DefaultConfig() Config {
	return Config{
		URL:              nats.DefaultURL,
		MaxReconnects:    -1, // unlimited
		ReconnectWait:    2 * time.Second,
		ReconnectJitter:  100 * time.Millisecond,
		ReconnectBufSize: 8 * 1024 * 1024, // 8 MB
		ConnectTimeout:   5 * time.Second,
		DrainTimeout:     30 * time.Second,
		FlusherTimeout:   1 * time.Second,
		PingInterval:     2 * time.Minute,
		Logger:           newDefaultLogger(),
	}
}

// ---------- functional option helpers ----------

// WithURL sets the NATS server URL(s). Comma-separated for clusters.
func WithURL(url string) Option {
	return func(c *Config) { c.URL = url }
}

// WithName sets the client connection name visible in NATS monitoring.
func WithName(name string) Option {
	return func(c *Config) { c.Name = name }
}

// WithToken sets token-based authentication.
func WithToken(token string) Option {
	return func(c *Config) { c.Token = token }
}

// WithUserPass sets user/password authentication.
func WithUserPass(user, password string) Option {
	return func(c *Config) { c.User = user; c.Password = password }
}

// WithCredsFile sets the path to a NATS credentials file (.creds).
func WithCredsFile(path string) Option {
	return func(c *Config) { c.CredsFile = path }
}

// WithNKeyFile sets the NKey seed file for authentication.
func WithNKeyFile(path string) Option {
	return func(c *Config) { c.NKeyFile = path }
}

// WithTLS provides a custom TLS configuration.
func WithTLS(cfg *tls.Config) Option {
	return func(c *Config) { c.TLSConfig = cfg }
}

// WithMaxReconnects sets the max reconnection attempts (-1 = unlimited).
func WithMaxReconnects(n int) Option {
	return func(c *Config) { c.MaxReconnects = n }
}

// WithReconnectWait sets the wait duration between reconnection attempts.
func WithReconnectWait(d time.Duration) Option {
	return func(c *Config) { c.ReconnectWait = d }
}

// WithReconnectJitter sets the max random jitter added to reconnect wait.
func WithReconnectJitter(d time.Duration) Option {
	return func(c *Config) { c.ReconnectJitter = d }
}

// WithReconnectBufSize sets the size of the reconnect buffer in bytes.
func WithReconnectBufSize(size int) Option {
	return func(c *Config) { c.ReconnectBufSize = size }
}

// WithConnectTimeout sets the initial connection dial timeout.
func WithConnectTimeout(d time.Duration) Option {
	return func(c *Config) { c.ConnectTimeout = d }
}

// WithDrainTimeout sets the maximum time to drain subscriptions on close.
func WithDrainTimeout(d time.Duration) Option {
	return func(c *Config) { c.DrainTimeout = d }
}

// WithPingInterval sets the interval for sending pings to the server.
func WithPingInterval(d time.Duration) Option {
	return func(c *Config) { c.PingInterval = d }
}

// WithLogger sets a custom Logger implementation.
func WithLogger(l Logger) Option {
	return func(c *Config) { c.Logger = l }
}

// WithOnDisconnect registers a callback invoked on disconnection.
func WithOnDisconnect(fn func(err error)) Option {
	return func(c *Config) { c.OnDisconnect = fn }
}

// WithOnReconnect registers a callback invoked on successful reconnection.
func WithOnReconnect(fn func()) Option {
	return func(c *Config) { c.OnReconnect = fn }
}

// WithOnClose registers a callback invoked when the connection is permanently closed.
func WithOnClose(fn func()) Option {
	return func(c *Config) { c.OnClose = fn }
}

// WithOnError registers a callback for asynchronous errors (e.g. slow consumers).
func WithOnError(fn func(sub *nats.Subscription, err error)) Option {
	return func(c *Config) { c.OnError = fn }
}
