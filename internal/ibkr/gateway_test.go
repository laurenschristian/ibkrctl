package ibkr

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestClasspathAndArgs(t *testing.T) {
	g := &Gateway{Dir: "/gw", Java: "/j/java", Port: 5001}
	cp := g.Classpath()
	if !strings.Contains(cp, "/gw/root") || !strings.Contains(cp, "clientportal.gw.jar") {
		t.Fatalf("classpath %s", cp)
	}
	args := g.Args()
	if args[len(args)-1] != filepath.Join("root", "conf.yaml") || args[len(args)-2] != GatewayMainClass {
		t.Fatalf("args %v", args)
	}
	if g.LoginURL() != "https://localhost:5001/" {
		t.Fatal(g.LoginURL())
	}
}

func TestInstalledAndPatchConf(t *testing.T) {
	dir := t.TempDir()
	g := &Gateway{Dir: dir, Port: 5001}
	if g.Installed() {
		t.Fatal("should not be installed")
	}
	// Fake the dist jar + conf.
	_ = os.MkdirAll(filepath.Join(dir, "dist"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "dist", "ibgroup.web.core.iblink.router.clientportal.gw.jar"), []byte("x"), 0o644)
	if !g.Installed() {
		t.Fatal("should be installed")
	}
	_ = os.MkdirAll(filepath.Join(dir, "root"), 0o755)
	conf := "listenPort: 5000\nips:\n  allow:\n    - 127.0.0.1\n  deny:\n    - 212.90.324.10\n"
	_ = os.WriteFile(filepath.Join(dir, "root", "conf.yaml"), []byte(conf), 0o644)
	if err := g.PatchConf(); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "root", "conf.yaml"))
	got := string(b)
	if !strings.Contains(got, "listenPort: 5001") {
		t.Fatalf("port not patched: %s", got)
	}
	if strings.Contains(got, "212.90.324.10") || strings.Contains(got, "deny:") {
		t.Fatalf("deny not stripped: %s", got)
	}
}

func TestCopyFromRejectsNonGateway(t *testing.T) {
	g := &Gateway{Dir: filepath.Join(t.TempDir(), "out")}
	if err := g.CopyFrom(t.TempDir()); err == nil {
		t.Fatal("want error for non-gateway src")
	}
}

func TestCopyFromCopies(t *testing.T) {
	src := t.TempDir()
	_ = os.MkdirAll(filepath.Join(src, "dist"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "dist", "ibgroup.web.core.iblink.router.clientportal.gw.jar"), []byte("x"), 0o644)
	_ = os.MkdirAll(filepath.Join(src, "root"), 0o755)
	_ = os.WriteFile(filepath.Join(src, "root", "conf.yaml"), []byte("listenPort: 5000\n"), 0o644)
	g := &Gateway{Dir: filepath.Join(t.TempDir(), "gw"), Port: 5001}
	if err := g.CopyFrom(src); err != nil {
		t.Fatal(err)
	}
	if !g.Installed() {
		t.Fatal("copied gateway not installed")
	}
}

func TestPlistGeneration(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	g := &Gateway{Dir: "/gw", Java: "/j/java", Port: 5001, Support: home}
	gp, err := g.WriteGatewayPlist()
	if err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(gp)
	s := string(b)
	if !strings.Contains(s, GatewayLabel) || !strings.Contains(s, "<key>KeepAlive</key>") || !strings.Contains(s, "/j/java") {
		t.Fatalf("gateway plist:\n%s", s)
	}
	kp, err := g.WriteKeepalivePlist("/bin/ibkrctl")
	if err != nil {
		t.Fatal(err)
	}
	b, _ = os.ReadFile(kp)
	s = string(b)
	if !strings.Contains(s, KeepaliveLabel) || !strings.Contains(s, "<integer>60</integer>") || !strings.Contains(s, "tickle") {
		t.Fatalf("keepalive plist:\n%s", s)
	}
}
