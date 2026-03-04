package natsx_test

import (
	"testing"
	"time"

	"github.com/nats-io/nats-server/v2/server"
	natsserver "github.com/nats-io/nats-server/v2/test"
	"github.com/vsys-soham/natsx"
)

// startTestServer spins up an in-process NATS server for testing.
func startTestServer(t *testing.T) *server.Server {
	t.Helper()
	opts := natsserver.DefaultTestOptions
	opts.Port = -1 // random available port
	s := natsserver.RunServer(&opts)
	t.Cleanup(s.Shutdown)
	return s
}

func serverURL(s *server.Server) string {
	return s.ClientURL()
}

// ---------- Config tests ----------

func TestDefaultConfig(t *testing.T) {
	cfg := natsx.DefaultConfig()
	if cfg.MaxReconnects != -1 {
		t.Errorf("expected MaxReconnects=-1, got %d", cfg.MaxReconnects)
	}
	if cfg.ReconnectWait != 2*time.Second {
		t.Errorf("expected ReconnectWait=2s, got %v", cfg.ReconnectWait)
	}
	if cfg.ConnectTimeout != 5*time.Second {
		t.Errorf("expected ConnectTimeout=5s, got %v", cfg.ConnectTimeout)
	}
}

func TestFunctionalOptions(t *testing.T) {
	cfg := natsx.DefaultConfig()
	natsx.WithURL("nats://custom:4222")(&cfg)
	natsx.WithName("test-client")(&cfg)
	natsx.WithToken("s3cret")(&cfg)
	natsx.WithMaxReconnects(5)(&cfg)

	if cfg.URL != "nats://custom:4222" {
		t.Errorf("URL mismatch: %s", cfg.URL)
	}
	if cfg.Name != "test-client" {
		t.Errorf("Name mismatch: %s", cfg.Name)
	}
	if cfg.Token != "s3cret" {
		t.Errorf("Token mismatch: %s", cfg.Token)
	}
	if cfg.MaxReconnects != 5 {
		t.Errorf("MaxReconnects mismatch: %d", cfg.MaxReconnects)
	}
}

// ---------- Connect / Close / Drain ----------

func TestConnectAndClose(t *testing.T) {
	s := startTestServer(t)

	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithName("test"),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}
	defer c.Close()

	if !c.IsConnected() {
		t.Fatal("expected IsConnected=true")
	}
	if c.Status() != natsx.StatusConnected {
		t.Fatalf("expected StatusConnected, got %v", c.Status())
	}

	c.Close()
	if c.Status() != natsx.StatusClosed {
		t.Fatalf("expected StatusClosed after Close, got %v", c.Status())
	}
}

func TestDrain(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect failed: %v", err)
	}

	if err := c.Drain(); err != nil {
		t.Fatalf("Drain failed: %v", err)
	}

	// Wait for the drain to complete
	time.Sleep(200 * time.Millisecond)
}

func TestConnectFailure(t *testing.T) {
	_, err := natsx.Connect(
		natsx.WithURL("nats://127.0.0.1:1"), // nothing listening
		natsx.WithMaxReconnects(0),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err == nil {
		t.Fatal("expected connection error, got nil")
	}
}

// ---------- Health ----------

func TestHealthStatus(t *testing.T) {
	s := startTestServer(t)
	c, err := natsx.Connect(
		natsx.WithURL(serverURL(s)),
		natsx.WithLogger(natsx.NopLogger{}),
	)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}
	defer c.Close()

	if url := c.ConnectedURL(); url == "" {
		t.Error("ConnectedURL should not be empty")
	}
	if sid := c.ConnectedServerID(); sid == "" {
		t.Error("ConnectedServerID should not be empty")
	}

	_ = c.Stats() // should not panic
}
