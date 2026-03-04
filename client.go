package natsx

import (
	"github.com/nats-io/nats.go"
)

// Client wraps a nats.Conn with high-level helpers and lifecycle management.
type Client struct {
	nc  *nats.Conn
	cfg Config
	log Logger
}

// Connect creates a new Client, applying all functional options, and connects
// to the NATS server. Returns an error if the initial connection fails.
func Connect(opts ...Option) (*Client, error) {
	cfg := DefaultConfig()
	for _, o := range opts {
		o(&cfg)
	}

	log := cfg.Logger
	if log == nil {
		log = newDefaultLogger()
	}

	natsOpts := []nats.Option{
		nats.MaxReconnects(cfg.MaxReconnects),
		nats.ReconnectWait(cfg.ReconnectWait),
		nats.ReconnectJitter(cfg.ReconnectJitter, cfg.ReconnectJitter*2),
		nats.ReconnectBufSize(cfg.ReconnectBufSize),
		nats.Timeout(cfg.ConnectTimeout),
		nats.DrainTimeout(cfg.DrainTimeout),
		nats.FlusherTimeout(cfg.FlusherTimeout),
		nats.PingInterval(cfg.PingInterval),
	}

	if cfg.Name != "" {
		natsOpts = append(natsOpts, nats.Name(cfg.Name))
	}

	// Authentication
	if cfg.Token != "" {
		natsOpts = append(natsOpts, nats.Token(cfg.Token))
	}
	if cfg.User != "" {
		natsOpts = append(natsOpts, nats.UserInfo(cfg.User, cfg.Password))
	}
	if cfg.CredsFile != "" {
		natsOpts = append(natsOpts, nats.UserCredentials(cfg.CredsFile))
	}
	if cfg.NKeyFile != "" {
		opt, err := nats.NkeyOptionFromSeed(cfg.NKeyFile)
		if err != nil {
			return nil, WrapError("Connect", err)
		}
		natsOpts = append(natsOpts, opt)
	}

	// TLS
	if cfg.TLSConfig != nil {
		natsOpts = append(natsOpts, nats.Secure(cfg.TLSConfig))
	}

	// Callbacks
	natsOpts = append(natsOpts, nats.DisconnectErrHandler(func(_ *nats.Conn, err error) {
		log.Warn("nats disconnected", "error", err)
		if cfg.OnDisconnect != nil {
			cfg.OnDisconnect(err)
		}
	}))
	natsOpts = append(natsOpts, nats.ReconnectHandler(func(nc *nats.Conn) {
		log.Info("nats reconnected", "url", nc.ConnectedUrl())
		if cfg.OnReconnect != nil {
			cfg.OnReconnect()
		}
	}))
	natsOpts = append(natsOpts, nats.ClosedHandler(func(_ *nats.Conn) {
		log.Info("nats connection closed")
		if cfg.OnClose != nil {
			cfg.OnClose()
		}
	}))
	natsOpts = append(natsOpts, nats.ErrorHandler(func(_ *nats.Conn, sub *nats.Subscription, err error) {
		subj := ""
		if sub != nil {
			subj = sub.Subject
		}
		log.Error("nats async error", "subject", subj, "error", err)
		if cfg.OnError != nil {
			cfg.OnError(sub, err)
		}
	}))

	nc, err := nats.Connect(cfg.URL, natsOpts...)
	if err != nil {
		return nil, WrapError("Connect", err)
	}

	log.Info("nats connected", "url", nc.ConnectedUrl(), "name", cfg.Name)

	return &Client{
		nc:  nc,
		cfg: cfg,
		log: log,
	}, nil
}

// Close immediately closes the connection to NATS.
func (c *Client) Close() {
	c.log.Info("closing nats connection")
	c.nc.Close()
}

// Drain gracefully drains all subscriptions and then closes the connection.
// In-flight messages will be processed before the connection is closed.
func (c *Client) Drain() error {
	c.log.Info("draining nats connection")
	if err := c.nc.Drain(); err != nil {
		return WrapError("Drain", err)
	}
	return nil
}

// Conn returns the underlying *nats.Conn for advanced use-cases.
// Use with caution — prefer the high-level helpers when possible.
func (c *Client) Conn() *nats.Conn {
	return c.nc
}

// Log returns the client's logger.
func (c *Client) Log() Logger {
	return c.log
}

// Flush flushes the connection, ensuring all published messages have been
// sent to the server.
func (c *Client) Flush() error {
	if err := c.nc.FlushTimeout(c.cfg.FlusherTimeout); err != nil {
		return WrapError("Flush", err)
	}
	return nil
}
