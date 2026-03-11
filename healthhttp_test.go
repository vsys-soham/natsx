package natsx_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/vsys-soham/natsx"
)

func startHealthServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1
	s := natsserver.RunServer(&opts)
	t.Cleanup(s.Shutdown)
	return s
}

func TestHealthCheck_Connected(t *testing.T) {
	s := startHealthServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	hs := c.HealthCheck()
	if hs.Status != "ok" {
		t.Errorf("expected status=ok, got %q", hs.Status)
	}
	if !hs.Connected {
		t.Error("expected Connected=true")
	}
	if hs.ConnectionState != "connected" {
		t.Errorf("expected ConnectionState=connected, got %q", hs.ConnectionState)
	}
	if hs.ServerURL == "" {
		t.Error("expected non-empty ServerURL")
	}
}

func TestHealthCheck_Closed(t *testing.T) {
	s := startHealthServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	c.Close()

	hs := c.HealthCheck()
	if hs.Status != "unavailable" {
		t.Errorf("expected status=unavailable after close, got %q", hs.Status)
	}
	if hs.Connected {
		t.Error("expected Connected=false")
	}
}

func TestReadinessHandler_OK(t *testing.T) {
	s := startHealthServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	handler := c.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var hs natsx.HealthStatus
	if err := json.NewDecoder(rr.Body).Decode(&hs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if hs.Status != "ok" {
		t.Errorf("expected status=ok, got %q", hs.Status)
	}
}

func TestReadinessHandler_Unavailable(t *testing.T) {
	s := startHealthServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	c.Close() // disconnect

	handler := c.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Errorf("expected 503, got %d", rr.Code)
	}
}

func TestLivenessHandler(t *testing.T) {
	s := startHealthServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	handler := c.LivenessHandler()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}

	var hs natsx.HealthStatus
	if err := json.NewDecoder(rr.Body).Decode(&hs); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if hs.Uptime == "" {
		t.Error("expected non-empty Uptime")
	}
}

func TestReadinessHandler_ContentType(t *testing.T) {
	s := startHealthServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(s.ClientURL()),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	handler := c.ReadinessHandler()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	rr := httptest.NewRecorder()
	handler(rr, req)

	ct := rr.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("expected Content-Type=application/json, got %q", ct)
	}
}
