package natsx

import "github.com/nats-io/nats.go"

// ConnectionStatus represents the current state of the NATS connection.
type ConnectionStatus int

const (
	StatusDisconnected ConnectionStatus = iota
	StatusConnected
	StatusReconnecting
	StatusClosed
	StatusDraining
)

// String returns a human-readable connection status.
func (s ConnectionStatus) String() string {
	switch s {
	case StatusConnected:
		return "connected"
	case StatusReconnecting:
		return "reconnecting"
	case StatusDisconnected:
		return "disconnected"
	case StatusClosed:
		return "closed"
	case StatusDraining:
		return "draining"
	default:
		return "unknown"
	}
}

// IsConnected returns true if the client is currently connected to a NATS server.
func (c *Client) IsConnected() bool {
	return c.nc.IsConnected()
}

// Status returns the current connection status.
func (c *Client) Status() ConnectionStatus {
	switch c.nc.Status() {
	case nats.CONNECTED:
		return StatusConnected
	case nats.RECONNECTING:
		return StatusReconnecting
	case nats.DISCONNECTED:
		return StatusDisconnected
	case nats.CLOSED:
		return StatusClosed
	case nats.DRAINING_SUBS, nats.DRAINING_PUBS:
		return StatusDraining
	default:
		return StatusDisconnected
	}
}

// Stats returns the raw NATS connection statistics (messages in/out, bytes, reconnects, etc.).
func (c *Client) Stats() nats.Statistics {
	return c.nc.Statistics
}

// ConnectedURL returns the URL of the server the client is currently connected to.
func (c *Client) ConnectedURL() string {
	return c.nc.ConnectedUrl()
}

// ConnectedServerID returns the server ID the client is connected to.
func (c *Client) ConnectedServerID() string {
	return c.nc.ConnectedServerId()
}
