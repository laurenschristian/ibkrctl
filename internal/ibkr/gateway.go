package ibkr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

// GatewayLabel is the launchd label for the always-on gateway agent.
const GatewayLabel = "com.laurenschristian.ibkrctl.gateway"

// KeepaliveLabel is the launchd label for the 60s tickle agent.
const KeepaliveLabel = "com.laurenschristian.ibkrctl.keepalive"

// GatewayMainClass is the clientportal.gw entry point.
const GatewayMainClass = "ibgroup.web.core.clientportal.gw.GatewayStart"

// Gateway manages a local Client Portal Gateway install and its launchd agents.
type Gateway struct {
	Dir     string // clientportal.gw directory
	Java    string // java executable
	Port    int
	Support string // ibkrctl support dir (for plists, logs)
}

// Classpath is the -cp value the gateway needs, all absolute.
func (g *Gateway) Classpath() string {
	return strings.Join([]string{
		filepath.Join(g.Dir, "root"),
		filepath.Join(g.Dir, "dist", "ibgroup.web.core.iblink.router.clientportal.gw.jar"),
		filepath.Join(g.Dir, "build", "lib", "runtime", "*"),
	}, ":")
}

// Args are the java args to launch the gateway (cwd = g.Dir).
func (g *Gateway) Args() []string {
	return []string{
		"-server",
		"-Djava.awt.headless=true",
		"-Xmx512m",
		"-Dvertx.disableDnsResolver=true",
		"-cp", g.Classpath(),
		GatewayMainClass,
		filepath.Join("root", "conf.yaml"),
	}
}

// Installed reports whether the gateway jar is present.
func (g *Gateway) Installed() bool {
	_, err := os.Stat(filepath.Join(g.Dir, "dist", "ibgroup.web.core.iblink.router.clientportal.gw.jar"))
	return err == nil
}

// PatchConf sets listenPort in root/conf.yaml.
func (g *Gateway) PatchConf() error {
	conf := filepath.Join(g.Dir, "root", "conf.yaml")
	b, err := os.ReadFile(conf)
	if err != nil {
		return err
	}
	re := regexp.MustCompile(`(?m)^listenPort: .*$`)
	out := re.ReplaceAllString(string(b), "listenPort: "+strconv.Itoa(g.Port))
	if !strings.Contains(out, "listenPort: "+strconv.Itoa(g.Port)) {
		out += "\nlistenPort: " + strconv.Itoa(g.Port) + "\n"
	}
	// The stock conf ships a malformed deny IP (octet > 255) that crashes the
	// gateway's IPv4 validator and 404s the login page. Drop the deny list.
	out = regexp.MustCompile(`(?m)^  deny:\n(?:    - .*\n)+`).ReplaceAllString(out, "")
	return os.WriteFile(conf, []byte(out), 0o644) //nolint:gosec
}

// CopyFrom copies a clientportal.gw tree from src into g.Dir.
func (g *Gateway) CopyFrom(src string) error {
	if _, err := os.Stat(filepath.Join(src, "dist", "ibgroup.web.core.iblink.router.clientportal.gw.jar")); err != nil {
		return fmt.Errorf("%s is not a clientportal.gw (no dist jar)", src)
	}
	if err := os.MkdirAll(filepath.Dir(g.Dir), 0o700); err != nil {
		return err
	}
	_ = os.RemoveAll(g.Dir)
	return copyTree(src, g.Dir)
}

func copyTree(src, dst string) error {
	cmd := exec.CommandContext(context.Background(), "cp", "-R", src+string(os.PathSeparator)+".", dst)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("copy %s: %w: %s", src, err, out)
	}
	return nil
}

// DiscoverGatewaySrc finds a bundled clientportal.gw (from the npx MCP package).
func DiscoverGatewaySrc() (string, error) {
	roots := []string{filepath.Join(os.Getenv("HOME"), ".npm", "_npx")}
	var found string
	for _, root := range roots {
		_ = filepath.WalkDir(root, func(p string, d os.DirEntry, err error) error {
			if err != nil || found != "" {
				return nil
			}
			if d.IsDir() && d.Name() == "clientportal.gw" {
				if _, e := os.Stat(filepath.Join(p, "dist", "ibgroup.web.core.iblink.router.clientportal.gw.jar")); e == nil {
					found = p
				}
			}
			return nil
		})
	}
	if found == "" {
		return "", errors.New("no bundled clientportal.gw found; pass --from <dir> or --download")
	}
	return found, nil
}

// DiscoverJava finds a java executable, preferring a JRE bundled with the npx package.
func DiscoverJava() (string, error) {
	arch := runtime.GOOS + "-" + runtime.GOARCH // e.g. darwin-arm64
	npx := filepath.Join(os.Getenv("HOME"), ".npm", "_npx")
	var bundled string
	_ = filepath.WalkDir(npx, func(p string, d os.DirEntry, err error) error {
		if err != nil || bundled != "" {
			return nil
		}
		if d.IsDir() && strings.HasSuffix(p, filepath.Join("runtime", arch, "bin")) {
			j := filepath.Join(p, "java")
			if fi, e := os.Stat(j); e == nil && fi.Mode()&0o111 != 0 {
				bundled = j
			}
		}
		return nil
	})
	if bundled != "" {
		return bundled, nil
	}
	if jh := os.Getenv("JAVA_HOME"); jh != "" {
		j := filepath.Join(jh, "bin", "java")
		if _, err := os.Stat(j); err == nil {
			return j, nil
		}
	}
	if runtime.GOOS == "darwin" {
		if out, err := exec.CommandContext(context.Background(), "/usr/libexec/java_home").Output(); err == nil {
			j := filepath.Join(strings.TrimSpace(string(out)), "bin", "java")
			if _, err := os.Stat(j); err == nil {
				return j, nil
			}
		}
	}
	if j, err := exec.LookPath("java"); err == nil {
		return j, nil
	}
	return "", errors.New("no java found; install a JRE or pass --java <path>")
}

// CopyJava copies the JRE home containing srcJava into <dst>/jre and returns the
// stable java path. If srcJava is a plain system java (no adjacent lib/), it is
// returned unchanged. This makes an install survive npx cache eviction.
func CopyJava(srcJava, dst string) (string, error) {
	// JRE home is the parent of the bin dir holding java.
	home := filepath.Dir(filepath.Dir(srcJava))
	if _, err := os.Stat(filepath.Join(home, "lib")); err != nil {
		return srcJava, nil // not a self-contained JRE tree; keep the path
	}
	target := filepath.Join(dst, "jre")
	_ = os.RemoveAll(target)
	if err := os.MkdirAll(dst, 0o700); err != nil {
		return "", err
	}
	if err := copyTree(home, target); err != nil {
		return "", err
	}
	j := filepath.Join(target, "bin", "java")
	if err := os.Chmod(j, 0o755); err != nil {
		return "", err
	}
	return j, nil
}

// LoginURL is the gateway login page.
func (g *Gateway) LoginURL() string {
	return fmt.Sprintf("https://localhost:%d/", g.Port)
}
