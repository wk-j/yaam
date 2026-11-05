// Package config reads and writes yaam's home directory files: config.toml
// holds the Settings and the Connections, never a password.
package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

const (
	DefaultLogLimit     = 100000
	DefaultAgentTimeout = 30
	DefaultPort         = 5432
)

type Settings struct {
	AgentLogLimit     int `toml:"agent_log_limit"`
	ExecutionLogLimit int `toml:"execution_log_limit"`
}

type Connection struct {
	Host                string `toml:"host"`
	Port                int    `toml:"port"`
	Database            string `toml:"database"`
	Role                string `toml:"role"`
	SSLMode             string `toml:"sslmode"`
	AgentTimeoutSeconds int    `toml:"agent_timeout_seconds"`
}

type Config struct {
	Settings    Settings              `toml:"settings"`
	Connections map[string]Connection `toml:"connections"`
}

// Home returns yaam's home directory: $YAAM_HOME, or ~/.yaam.
func Home() (string, error) {
	if h := os.Getenv("YAAM_HOME"); h != "" {
		return filepath.Abs(h)
	}
	u, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(u, ".yaam"), nil
}

// EnsureHome creates the home directory with mode 0700.
func EnsureHome(dir string) error {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	return os.Chmod(dir, 0o700)
}

// Load reads path; a missing file gives the defaults.
func Load(path string) (*Config, error) {
	c := &Config{}
	if _, err := toml.DecodeFile(path, c); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	c.applyDefaults()
	return c, nil
}

func (c *Config) applyDefaults() {
	if c.Settings.AgentLogLimit <= 0 {
		c.Settings.AgentLogLimit = DefaultLogLimit
	}
	if c.Settings.ExecutionLogLimit <= 0 {
		c.Settings.ExecutionLogLimit = DefaultLogLimit
	}
	if c.Connections == nil {
		c.Connections = map[string]Connection{}
	}
	for name, conn := range c.Connections {
		if conn.Port == 0 {
			conn.Port = DefaultPort
		}
		if conn.SSLMode == "" {
			conn.SSLMode = "disable"
		}
		if conn.AgentTimeoutSeconds <= 0 {
			conn.AgentTimeoutSeconds = DefaultAgentTimeout
		}
		c.Connections[name] = conn
	}
}

// Save writes c to path atomically (temp file + rename) with mode 0600.
func Save(path string, c *Config) error {
	var buf bytes.Buffer
	if err := toml.NewEncoder(&buf).Encode(c); err != nil {
		return err
	}
	return WriteFileAtomic(path, buf.Bytes())
}

// WriteFileAtomic writes data to a temp file beside path, then renames it
// over path, so readers never see a half-written file.
func WriteFileAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	if err := f.Chmod(0o600); err != nil {
		f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
