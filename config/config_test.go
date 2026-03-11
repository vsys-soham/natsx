package config_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/vsys-soham/natsx/config"
)

func TestDefault(t *testing.T) {
	cfg := config.Default()

	if cfg.URL != "nats://localhost:4222" {
		t.Errorf("URL=%q, want nats://localhost:4222", cfg.URL)
	}
	if cfg.MaxReconnects != -1 {
		t.Errorf("MaxReconnects=%d, want -1", cfg.MaxReconnects)
	}
	if cfg.ReconnectWait != 2*time.Second {
		t.Errorf("ReconnectWait=%v, want 2s", cfg.ReconnectWait)
	}
	if cfg.ConnectTimeout != 5*time.Second {
		t.Errorf("ConnectTimeout=%v, want 5s", cfg.ConnectTimeout)
	}
	if cfg.DrainTimeout != 10*time.Second {
		t.Errorf("DrainTimeout=%v, want 10s", cfg.DrainTimeout)
	}
}

func TestFromEnv_Defaults(t *testing.T) {
	// Clear any existing NATS env vars.
	clearNATSEnv(t)

	cfg := config.FromEnv()
	if cfg.URL != "nats://localhost:4222" {
		t.Errorf("expected default URL, got %q", cfg.URL)
	}
}

func TestFromEnv_ReadsEnvVars(t *testing.T) {
	clearNATSEnv(t)
	t.Setenv("NATS_URL", "nats://remote:4222")
	t.Setenv("NATS_NAME", "my-service")
	t.Setenv("NATS_TOKEN", "secret-token")
	t.Setenv("NATS_USERNAME", "alice")
	t.Setenv("NATS_PASSWORD", "pass123")
	t.Setenv("NATS_MAX_RECONNECTS", "5")
	t.Setenv("NATS_RECONNECT_WAIT", "3s")
	t.Setenv("NATS_CONNECT_TIMEOUT", "10s")
	t.Setenv("NATS_DRAIN_TIMEOUT", "30s")

	cfg := config.FromEnv()

	if cfg.URL != "nats://remote:4222" {
		t.Errorf("URL=%q, want nats://remote:4222", cfg.URL)
	}
	if cfg.Name != "my-service" {
		t.Errorf("Name=%q, want my-service", cfg.Name)
	}
	if cfg.Token != "secret-token" {
		t.Errorf("Token=%q", cfg.Token)
	}
	if cfg.Username != "alice" || cfg.Password != "pass123" {
		t.Errorf("Username/Password mismatch")
	}
	if cfg.MaxReconnects != 5 {
		t.Errorf("MaxReconnects=%d, want 5", cfg.MaxReconnects)
	}
	if cfg.ReconnectWait != 3*time.Second {
		t.Errorf("ReconnectWait=%v, want 3s", cfg.ReconnectWait)
	}
	if cfg.ConnectTimeout != 10*time.Second {
		t.Errorf("ConnectTimeout=%v, want 10s", cfg.ConnectTimeout)
	}
	if cfg.DrainTimeout != 30*time.Second {
		t.Errorf("DrainTimeout=%v, want 30s", cfg.DrainTimeout)
	}
}

func TestLoadFile_JSON(t *testing.T) {
	json := `{
		"url": "nats://fileserver:4222",
		"name": "file-svc",
		"max_reconnects": 3,
		"reconnect_wait": "5s"
	}`
	path := writeTemp(t, "config.json", json)

	cfg, err := config.LoadFile(path)
	if err != nil {
		t.Fatalf("LoadFile: %v", err)
	}

	if cfg.URL != "nats://fileserver:4222" {
		t.Errorf("URL=%q", cfg.URL)
	}
	if cfg.Name != "file-svc" {
		t.Errorf("Name=%q", cfg.Name)
	}
	if cfg.MaxReconnects != 3 {
		t.Errorf("MaxReconnects=%d", cfg.MaxReconnects)
	}
	if cfg.ReconnectWait != 5*time.Second {
		t.Errorf("ReconnectWait=%v", cfg.ReconnectWait)
	}
}

func TestLoadFile_NotFound(t *testing.T) {
	_, err := config.LoadFile("/nonexistent/path/config.json")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadFile_InvalidJSON(t *testing.T) {
	path := writeTemp(t, "bad.json", `{not valid json`)
	_, err := config.LoadFile(path)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestValidate_Valid(t *testing.T) {
	cases := []string{
		"nats://localhost:4222",
		"tls://secure:4222",
		"ws://websocket:8080",
		"wss://secure-ws:8080",
	}
	for _, url := range cases {
		cfg := config.Config{URL: url}
		if err := cfg.Validate(); err != nil {
			t.Errorf("URL=%q: unexpected error: %v", url, err)
		}
	}
}

func TestValidate_Invalid(t *testing.T) {
	cases := []struct {
		url string
		err bool
	}{
		{"", true},
		{"   ", true},
		{"http://bad-scheme:4222", true},
		{"nats://ok:4222", false},
	}
	for _, tc := range cases {
		cfg := config.Config{URL: tc.url}
		err := cfg.Validate()
		if tc.err && err == nil {
			t.Errorf("URL=%q: expected error", tc.url)
		}
		if !tc.err && err != nil {
			t.Errorf("URL=%q: unexpected error: %v", tc.url, err)
		}
	}
}

func TestToOptions_BasicURL(t *testing.T) {
	cfg := config.Config{URL: "nats://localhost:4222"}
	opts := cfg.ToOptions()
	if len(opts) == 0 {
		t.Fatal("expected at least one option")
	}
}

func TestToOptions_FullConfig(t *testing.T) {
	cfg := config.Config{
		URL:            "nats://localhost:4222",
		Name:           "svc",
		Token:          "tok",
		MaxReconnects:  10,
		ReconnectWait:  2 * time.Second,
		ConnectTimeout: 5 * time.Second,
		DrainTimeout:   15 * time.Second,
	}
	opts := cfg.ToOptions()
	// Just verify all options are returned without panic
	if len(opts) < 5 {
		t.Errorf("expected >=5 options, got %d", len(opts))
	}
}

// helpers

func clearNATSEnv(t *testing.T) {
	t.Helper()
	for _, k := range []string{
		"NATS_URL", "NATS_NAME", "NATS_TOKEN",
		"NATS_USERNAME", "NATS_PASSWORD",
		"NATS_CREDS_FILE", "NATS_NKEY_FILE",
		"NATS_MAX_RECONNECTS", "NATS_RECONNECT_WAIT",
		"NATS_CONNECT_TIMEOUT", "NATS_DRAIN_TIMEOUT",
	} {
		old := os.Getenv(k)
		os.Unsetenv(k)
		t.Cleanup(func() {
			if old != "" {
				os.Setenv(k, old)
			}
		})
	}
}

func writeTemp(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writeTemp: %v", err)
	}
	return path
}
