package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func clearEnv(t *testing.T) {
	for _, k := range []string{"IBKR_URL", "IBKR_PORT", "IBKR_ACCOUNT", "IBKR_USER", "IBKR_PASS_CMD", "IBKR_GATEWAY_DIR", "IBKR_JAVA"} {
		t.Setenv(k, "")
	}
}

func TestSaveLoadPassword(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	t.Setenv("IBKR_CONFIG", p)
	clearEnv(t)
	if err := Save(&Config{Username: "u", PasswordCmd: "echo secret"}); err != nil {
		t.Fatal(err)
	}
	st, _ := os.Stat(p)
	if st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %v", st.Mode())
	}
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.Port != DefaultPort || c.URL != "https://localhost:5001" {
		t.Fatalf("defaults %+v", c)
	}
	if !strings.HasSuffix(c.GatewayDir, filepath.Join("gateway", "clientportal.gw")) {
		t.Fatalf("gatewaydir %s", c.GatewayDir)
	}
	pw, err := c.Password()
	if err != nil || pw != "secret" {
		t.Fatalf("password %q %v", pw, err)
	}
}

func TestPasswordEmpty(t *testing.T) {
	c := &Config{}
	if pw, err := c.Password(); err != nil || pw != "" {
		t.Fatalf("want empty, got %q %v", pw, err)
	}
}

func TestEnvOverride(t *testing.T) {
	p := filepath.Join(t.TempDir(), "c.yaml")
	t.Setenv("IBKR_CONFIG", p)
	clearEnv(t)
	t.Setenv("IBKR_URL", "https://host:9000/")
	t.Setenv("IBKR_PORT", "9000")
	t.Setenv("IBKR_ACCOUNT", "U1")
	t.Setenv("IBKR_USER", "bob")
	t.Setenv("IBKR_PASS_CMD", "echo x")
	t.Setenv("IBKR_JAVA", "/j/java")
	c, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if c.URL != "https://host:9000" || c.Port != 9000 || c.Account != "U1" || c.Username != "bob" || c.JavaBin != "/j/java" {
		t.Fatalf("%+v", c)
	}
}

func TestPathsAndBadYAML(t *testing.T) {
	t.Setenv("IBKR_CONFIG", "")
	if !strings.HasSuffix(Path(), filepath.Join("ibkrctl", "config.yaml")) {
		t.Fatal(Path())
	}
	if !strings.HasSuffix(GatewayHome(), filepath.Join("ibkrctl", "gateway")) {
		t.Fatal(GatewayHome())
	}
	p := filepath.Join(t.TempDir(), "c.yaml")
	t.Setenv("IBKR_CONFIG", p)
	_ = os.WriteFile(p, []byte("url: [x"), 0o600)
	if _, err := Load(); err == nil {
		t.Fatal("want yaml error")
	}
}

func TestAliasResolution(t *testing.T) {
	c := &Config{Accounts: []AccountAlias{{ID: "U111", Alias: "account-1"}, {ID: "U222", Alias: "account-2"}}}
	if c.AliasFor("U111") != "account-1" || c.AliasFor("U999") != "U999" {
		t.Fatal("AliasFor")
	}
	if c.IDFor("account-2") != "U222" || c.IDFor("U111") != "U111" || c.IDFor("nope") != "nope" {
		t.Fatal("IDFor")
	}
	if c.DefaultAccountID() != "U111" {
		t.Fatalf("default %s", c.DefaultAccountID())
	}
	c.Account = "account-2"
	if c.DefaultAccountID() != "U222" {
		t.Fatalf("default with account %s", c.DefaultAccountID())
	}
	if c.RedactMap() != nil {
		t.Fatal("redact off should be nil")
	}
	c.Redact = true
	if c.RedactMap()["U111"] != "account-1" {
		t.Fatal("RedactMap")
	}
}
