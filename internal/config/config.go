// Package config resolves settings: flags > env > config file.
package config

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config holds ibkrctl settings. IBKR login is browser-based (no password is
// ever stored); these are the local Client Portal Gateway details only.
type Config struct {
	URL        string `yaml:"url,omitempty"`         // gateway base, e.g. https://localhost:5001
	Port       int    `yaml:"port,omitempty"`        // gateway listen port
	Account    string `yaml:"account,omitempty"`     // default account id for positions/pnl/orders
	GatewayDir string `yaml:"gateway_dir,omitempty"` // path to an installed clientportal.gw
	JavaBin    string `yaml:"java_bin,omitempty"`    // java executable used to run the gateway
	// Login automation (optional). The password itself is never stored here;
	// PasswordCmd prints it (e.g. a macOS Keychain lookup).
	Username    string `yaml:"username,omitempty"`
	PasswordCmd string `yaml:"password_cmd,omitempty"`
	TwoFA       string `yaml:"twofa,omitempty"` // ibkey | code | none
	// Accounts maps real account ids to stable aliases. With Redact on, output
	// shows the alias and account ids are stripped, so the agent never sees the
	// real numbers. Commands accept either the alias or the real id.
	Accounts []AccountAlias `yaml:"accounts,omitempty"`
	Redact   bool           `yaml:"redact,omitempty"`
}

type AccountAlias struct {
	ID    string `yaml:"id"`
	Alias string `yaml:"alias"`
}

// AliasFor returns the alias for a real id, or the id unchanged.
func (c *Config) AliasFor(id string) string {
	for _, a := range c.Accounts {
		if a.ID == id {
			return a.Alias
		}
	}
	return id
}

// IDFor resolves an alias (or a real id) to the real id.
func (c *Config) IDFor(aliasOrID string) string {
	for _, a := range c.Accounts {
		if a.Alias == aliasOrID {
			return a.ID
		}
	}
	return aliasOrID
}

// DefaultAccountID returns the configured default account id: the explicit
// Account (alias or id) if set, else the first alias entry.
func (c *Config) DefaultAccountID() string {
	if c.Account != "" {
		return c.IDFor(c.Account)
	}
	if len(c.Accounts) > 0 {
		return c.Accounts[0].ID
	}
	return ""
}

// RedactMap returns real-id -> alias for output redaction (empty if Redact off).
func (c *Config) RedactMap() map[string]string {
	if !c.Redact {
		return nil
	}
	m := make(map[string]string, len(c.Accounts))
	for _, a := range c.Accounts {
		m[a.ID] = a.Alias
	}
	return m
}

const (
	DefaultPort = 5001
)

func supportDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "ibkrctl")
}

func Path() string {
	if p := os.Getenv("IBKR_CONFIG"); p != "" {
		return p
	}
	return filepath.Join(supportDir(), "config.yaml")
}

// GatewayHome is where `gateway install` places its self-contained copy.
func GatewayHome() string {
	return filepath.Join(supportDir(), "gateway")
}

func Load() (*Config, error) {
	c := &Config{Port: DefaultPort}
	if b, err := os.ReadFile(Path()); err == nil {
		if err := yaml.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}
	if v := os.Getenv("IBKR_URL"); v != "" {
		c.URL = v
	}
	if v := os.Getenv("IBKR_PORT"); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			c.Port = n
		}
	}
	if v := os.Getenv("IBKR_ACCOUNT"); v != "" {
		c.Account = v
	}
	if v := os.Getenv("IBKR_USER"); v != "" {
		c.Username = v
	}
	if v := os.Getenv("IBKR_PASS_CMD"); v != "" {
		c.PasswordCmd = v
	}
	if v := os.Getenv("IBKR_GATEWAY_DIR"); v != "" {
		c.GatewayDir = v
	}
	if v := os.Getenv("IBKR_JAVA"); v != "" {
		c.JavaBin = v
	}
	if c.Port == 0 {
		c.Port = DefaultPort
	}
	if c.URL == "" {
		c.URL = "https://localhost:" + strconv.Itoa(c.Port)
	}
	if c.GatewayDir == "" {
		c.GatewayDir = filepath.Join(GatewayHome(), "clientportal.gw")
	}
	c.URL = strings.TrimRight(c.URL, "/")
	return c, nil
}

func Save(c *Config) error {
	p := Path()
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	b, err := yaml.Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o600)
}

// Password runs PasswordCmd and returns its trimmed stdout. Empty if unset.
func (c *Config) Password() (string, error) {
	if c.PasswordCmd == "" {
		return "", nil
	}
	out, err := execCommand(c.PasswordCmd)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

func execCommand(sh string) ([]byte, error) {
	return exec.CommandContext(context.Background(), "sh", "-c", sh).Output()
}
