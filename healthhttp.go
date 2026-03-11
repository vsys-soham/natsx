package natsx

import (
	"encoding/json"
	"net/http"
	"time"
)

// HealthStatus represents the health of the NATS connection for probes.
type HealthStatus struct {
	// Status is "ok", "degraded", or "unavailable".
	Status string `json:"status"`

	// Connected indicates if the client is currently connected.
	Connected bool `json:"connected"`

	// ConnectionState is the human-readable connection state.
	ConnectionState string `json:"connection_state"`

	// ServerURL is the currently connected server URL.
	ServerURL string `json:"server_url,omitempty"`

	// ServerID is the connected server's ID.
	ServerID string `json:"server_id,omitempty"`

	// Uptime is how long the health check endpoint has been running.
	Uptime string `json:"uptime,omitempty"`
}

// HealthCheck returns the current health status of the client.
func (c *Client) HealthCheck() HealthStatus {
	connected := c.IsConnected()
	status := "ok"
	if !connected {
		switch c.Status() {
		case StatusReconnecting:
			status = "degraded"
		default:
			status = "unavailable"
		}
	}

	return HealthStatus{
		Status:          status,
		Connected:       connected,
		ConnectionState: c.Status().String(),
		ServerURL:       c.ConnectedURL(),
		ServerID:        c.ConnectedServerID(),
	}
}

// ReadinessHandler returns an http.HandlerFunc for readiness probes (/readyz).
// Returns 200 if connected, 503 if not.
func (c *Client) ReadinessHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hs := c.HealthCheck()
		w.Header().Set("Content-Type", "application/json")
		if hs.Connected {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(hs)
	}
}

// LivenessHandler returns an http.HandlerFunc for liveness probes (/healthz).
// Returns 200 if the connection is not permanently closed, 503 if closed.
func (c *Client) LivenessHandler() http.HandlerFunc {
	startTime := time.Now()
	return func(w http.ResponseWriter, r *http.Request) {
		hs := c.HealthCheck()
		hs.Uptime = time.Since(startTime).Round(time.Second).String()

		w.Header().Set("Content-Type", "application/json")
		if c.Status() == StatusClosed {
			w.WriteHeader(http.StatusServiceUnavailable)
		} else {
			w.WriteHeader(http.StatusOK)
		}
		json.NewEncoder(w).Encode(hs)
	}
}
