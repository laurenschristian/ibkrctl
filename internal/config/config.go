// Package config resolves settings: flags > env > config file.
package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	URL      string `yaml:"url,omitempty"`
	Username string `yaml:"username,omitempty"`
	Password string `yaml:"password,omitempty"`
	// PasswordCmd is run and its stdout used as the secret, e.g. a keychain or `op read`.
	PasswordCmd string `yaml:"password_cmd,omitempty"`
}

func Path() string {
	if p := os.Getenv("IBKR_CONFIG"); p != "" {
		return p
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		dir = filepath.Join(os.Getenv("HOME"), ".config")
	}
	return filepath.Join(dir, "ibkrctl", "config.yaml")
}

func Load() (*Config, error) {
	c := &Config{}
	if b, err := os.ReadFile(Path()); err == nil {
		if err := yaml.Unmarshal(b, c); err != nil {
			return nil, err
		}
	}
	if v := os.Getenv("IBKR_URL"); v != "" {
		c.URL = v
	}
	if v := os.Getenv("IBKR_USER"); v != "" {
		c.Username = v
	}
	if v := os.Getenv("IBKR_PASS"); v != "" {
		c.Password = v
	}
	return c, nil
}

// Resolve fills Password from PasswordCmd when needed.
func (c *Config) Resolve() error {
	if c.Password == "" && c.PasswordCmd != "" {
		out, err := exec.CommandContext(context.Background(), "sh", "-c", c.PasswordCmd).Output()
		if err != nil {
			return errors.New("password_cmd failed: " + err.Error())
		}
		c.Password = strings.TrimSpace(string(out))
	}
	c.URL = strings.TrimRight(c.URL, "/")
	return nil
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
