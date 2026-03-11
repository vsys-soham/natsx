// Package config provides optional configuration loading for natsx clients
// from environment variables and/or struct tags.
//
// The Config struct can be filled by:
//   - Direct field assignment
//   - FromEnv() — reads well-known NATS_* environment variables
//   - A YAML/JSON file (see LoadFile)
//
// This package has no external dependencies beyond the standard library.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/vsys-soham/natsx"
)

// Config holds natsx client configuration that can be loaded from the
// environment or a file.
type Config struct {
	// URL is the NATS server URL (default: nats://localhost:4222).
	URL string `json:"url" env:"NATS_URL"`

	// Name is the client name sent to the server.
	Name string `json:"name" env:"NATS_NAME"`

	// Token is the auth token.
	Token string `json:"token" env:"NATS_TOKEN"`

	// Username and Password for user/password auth.
	Username string `json:"username" env:"NATS_USERNAME"`
	Password string `json:"password" env:"NATS_PASSWORD"`

	// CredsFile is the path to a NATS credentials file.
	CredsFile string `json:"creds_file" env:"NATS_CREDS_FILE"`

	// NKeyFile is the path to an NKey seed file.
	NKeyFile string `json:"nkey_file" env:"NATS_NKEY_FILE"`

	// MaxReconnects is the maximum number of reconnect attempts (-1 = unlimited).
	MaxReconnects int `json:"max_reconnects" env:"NATS_MAX_RECONNECTS"`

	// ReconnectWait is the time to wait between reconnect attempts.
	ReconnectWait time.Duration `json:"reconnect_wait" env:"NATS_RECONNECT_WAIT"`

	// ConnectTimeout is the maximum time to wait for a connection.
	ConnectTimeout time.Duration `json:"connect_timeout" env:"NATS_CONNECT_TIMEOUT"`

	// DrainTimeout is the maximum time to wait for a graceful drain.
	DrainTimeout time.Duration `json:"drain_timeout" env:"NATS_DRAIN_TIMEOUT"`
}

// Default returns a Config with sensible defaults.
func Default() Config {
	return Config{
		URL:            "nats://localhost:4222",
		MaxReconnects:  -1,
		ReconnectWait:  2 * time.Second,
		ConnectTimeout: 5 * time.Second,
		DrainTimeout:   10 * time.Second,
	}
}

// FromEnv reads configuration from NATS_* environment variables, falling
// back to defaults for unset variables.
func FromEnv() Config {
	cfg := Default()

	if v := os.Getenv("NATS_URL"); v != "" {
		cfg.URL = v
	}
	if v := os.Getenv("NATS_NAME"); v != "" {
		cfg.Name = v
	}
	if v := os.Getenv("NATS_TOKEN"); v != "" {
		cfg.Token = v
	}
	if v := os.Getenv("NATS_USERNAME"); v != "" {
		cfg.Username = v
	}
	if v := os.Getenv("NATS_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("NATS_CREDS_FILE"); v != "" {
		cfg.CredsFile = v
	}
	if v := os.Getenv("NATS_NKEY_FILE"); v != "" {
		cfg.NKeyFile = v
	}
	if v := os.Getenv("NATS_MAX_RECONNECTS"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			cfg.MaxReconnects = n
		}
	}
	if v := os.Getenv("NATS_RECONNECT_WAIT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ReconnectWait = d
		}
	}
	if v := os.Getenv("NATS_CONNECT_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.ConnectTimeout = d
		}
	}
	if v := os.Getenv("NATS_DRAIN_TIMEOUT"); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			cfg.DrainTimeout = d
		}
	}

	return cfg
}

// LoadFile reads a JSON config file and returns a Config.
// Duration fields (reconnect_wait, connect_timeout, drain_timeout) accept
// Go duration strings like "2s", "500ms", "1m30s".
func LoadFile(path string) (Config, error) {
	cfg := Default()

	data, err := os.ReadFile(path)
	if err != nil {
		return cfg, fmt.Errorf("config.LoadFile: %w", err)
	}

	// Strip // comments for convenience (JSONC-style)
	data = stripComments(data)

	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, fmt.Errorf("config.LoadFile: %w", err)
	}
	return cfg, nil
}

// UnmarshalJSON implements json.Unmarshaler. Duration fields accept either a
// Go duration string ("2s", "500ms") or a nanosecond integer.
func (c *Config) UnmarshalJSON(data []byte) error {
	// Intermediate struct with string-typed duration fields.
	type fileConfig struct {
		URL            string `json:"url"`
		Name           string `json:"name"`
		Token          string `json:"token"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		CredsFile      string `json:"creds_file"`
		NKeyFile       string `json:"nkey_file"`
		MaxReconnects  int    `json:"max_reconnects"`
		ReconnectWait  string `json:"reconnect_wait"`
		ConnectTimeout string `json:"connect_timeout"`
		DrainTimeout   string `json:"drain_timeout"`
	}

	var fc fileConfig
	if err := json.Unmarshal(data, &fc); err != nil {
		return err
	}

	if fc.URL != "" {
		c.URL = fc.URL
	}
	if fc.Name != "" {
		c.Name = fc.Name
	}
	if fc.Token != "" {
		c.Token = fc.Token
	}
	if fc.Username != "" {
		c.Username = fc.Username
	}
	if fc.Password != "" {
		c.Password = fc.Password
	}
	if fc.CredsFile != "" {
		c.CredsFile = fc.CredsFile
	}
	if fc.NKeyFile != "" {
		c.NKeyFile = fc.NKeyFile
	}
	if fc.MaxReconnects != 0 {
		c.MaxReconnects = fc.MaxReconnects
	}
	if fc.ReconnectWait != "" {
		d, err := time.ParseDuration(fc.ReconnectWait)
		if err != nil {
			return fmt.Errorf("config: invalid reconnect_wait %q: %w", fc.ReconnectWait, err)
		}
		c.ReconnectWait = d
	}
	if fc.ConnectTimeout != "" {
		d, err := time.ParseDuration(fc.ConnectTimeout)
		if err != nil {
			return fmt.Errorf("config: invalid connect_timeout %q: %w", fc.ConnectTimeout, err)
		}
		c.ConnectTimeout = d
	}
	if fc.DrainTimeout != "" {
		d, err := time.ParseDuration(fc.DrainTimeout)
		if err != nil {
			return fmt.Errorf("config: invalid drain_timeout %q: %w", fc.DrainTimeout, err)
		}
		c.DrainTimeout = d
	}
	return nil
}


// ToOptions converts Config to a slice of natsx.Option suitable for
// passing to natsx.Connect.
func (c Config) ToOptions() []natsx.Option {
	opts := []natsx.Option{
		natsx.WithURL(c.URL),
	}
	if c.Name != "" {
		opts = append(opts, natsx.WithName(c.Name))
	}
	if c.Token != "" {
		opts = append(opts, natsx.WithToken(c.Token))
	}
	if c.Username != "" && c.Password != "" {
		opts = append(opts, natsx.WithUserPass(c.Username, c.Password))
	}
	if c.CredsFile != "" {
		opts = append(opts, natsx.WithCredsFile(c.CredsFile))
	}
	if c.NKeyFile != "" {
		opts = append(opts, natsx.WithNKeyFile(c.NKeyFile))
	}
	if c.MaxReconnects != 0 {
		opts = append(opts, natsx.WithMaxReconnects(c.MaxReconnects))
	}
	if c.ReconnectWait > 0 {
		opts = append(opts, natsx.WithReconnectWait(c.ReconnectWait))
	}
	if c.ConnectTimeout > 0 {
		opts = append(opts, natsx.WithConnectTimeout(c.ConnectTimeout))
	}
	if c.DrainTimeout > 0 {
		opts = append(opts, natsx.WithDrainTimeout(c.DrainTimeout))
	}
	return opts
}

// Connect is a convenience function: load config then connect.
func Connect(cfg Config, extra ...natsx.Option) (*natsx.Client, error) {
	opts := append(cfg.ToOptions(), extra...)
	return natsx.Connect(opts...)
}

// Validate returns an error if required fields are missing or invalid.
func (c Config) Validate() error {
	if strings.TrimSpace(c.URL) == "" {
		return fmt.Errorf("config: URL is required")
	}
	if !strings.HasPrefix(c.URL, "nats://") &&
		!strings.HasPrefix(c.URL, "tls://") &&
		!strings.HasPrefix(c.URL, "ws://") &&
		!strings.HasPrefix(c.URL, "wss://") {
		return fmt.Errorf("config: URL must start with nats://, tls://, ws://, or wss://")
	}
	return nil
}

// stripComments removes // line comments from JSON data, but only when the //
// appears outside a quoted string literal. This avoids corrupting values like
// "nats://localhost:4222".
func stripComments(data []byte) []byte {
	lines := strings.Split(string(data), "\n")
	result := make([]string, 0, len(lines))
	for _, line := range lines {
		result = append(result, stripLineComment(line))
	}
	return []byte(strings.Join(result, "\n"))
}

// stripLineComment removes a // comment from a single JSON line, skipping
// any // that appear inside a double-quoted string.
func stripLineComment(line string) string {
	inStr := false
	for i := 0; i < len(line); i++ {
		ch := line[i]
		if ch == '\\' && inStr {
			i++ // skip escaped character
			continue
		}
		if ch == '"' {
			inStr = !inStr
			continue
		}
		if !inStr && ch == '/' && i+1 < len(line) && line[i+1] == '/' {
			return line[:i]
		}
	}
	return line
}
