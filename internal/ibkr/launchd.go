package ibkr

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// AgentDir is where launchd looks for user agents.
func AgentDir() string {
	return filepath.Join(os.Getenv("HOME"), "Library", "LaunchAgents")
}

func plistPath(label string) string {
	return filepath.Join(AgentDir(), label+".plist")
}

type plist struct {
	XMLName xml.Name `xml:"plist"`
	Version string   `xml:"version,attr"`
	Dict    dict     `xml:"dict"`
}

// dict marshals an ordered set of key/value entries as a plist <dict>.
type dict struct {
	Entries []entry
}

type entry struct {
	Key string
	Val any
}

func (d dict) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "dict"}}); err != nil {
		return err
	}
	for _, en := range d.Entries {
		if err := e.EncodeElement(en.Key, xml.StartElement{Name: xml.Name{Local: "key"}}); err != nil {
			return err
		}
		if err := encodeVal(e, en.Val); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "dict"}})
}

func encodeVal(e *xml.Encoder, v any) error {
	switch t := v.(type) {
	case bool:
		name := "false"
		if t {
			name = "true"
		}
		if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: name}}); err != nil {
			return err
		}
		return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: name}})
	case int:
		return e.EncodeElement(strconv.Itoa(t), xml.StartElement{Name: xml.Name{Local: "integer"}})
	case string:
		return e.EncodeElement(t, xml.StartElement{Name: xml.Name{Local: "string"}})
	case []string:
		if err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "array"}}); err != nil {
			return err
		}
		for _, s := range t {
			if err := e.EncodeElement(s, xml.StartElement{Name: xml.Name{Local: "string"}}); err != nil {
				return err
			}
		}
		return e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "array"}})
	}
	return fmt.Errorf("unsupported plist value %T", v)
}

func writePlist(label string, entries []entry) (string, error) {
	if err := os.MkdirAll(AgentDir(), 0o755); err != nil {
		return "", err
	}
	p := plist{Version: "1.0", Dict: dict{Entries: entries}}
	b, err := xml.MarshalIndent(p, "", "  ")
	if err != nil {
		return "", err
	}
	doc := xml.Header +
		"<!DOCTYPE plist PUBLIC \"-//Apple//DTD PLIST 1.0//EN\" \"http://www.apple.com/DTDs/PropertyList-1.0.dtd\">\n" +
		string(b) + "\n"
	path := plistPath(label)
	return path, os.WriteFile(path, []byte(doc), 0o644) //nolint:gosec
}

// WriteGatewayPlist writes the always-on gateway launchd agent.
func (g *Gateway) WriteGatewayPlist() (string, error) {
	logf := filepath.Join(g.Support, "gateway.log")
	args := append([]string{g.Java}, g.Args()...)
	return writePlist(GatewayLabel, []entry{
		{"Label", GatewayLabel},
		{"ProgramArguments", args},
		{"WorkingDirectory", g.Dir},
		{"RunAtLoad", true},
		{"KeepAlive", true},
		{"StandardOutPath", logf},
		{"StandardErrorPath", logf},
	})
}

// WriteKeepalivePlist writes the 60s tickle agent that runs ibkrctlPath tickle.
func (g *Gateway) WriteKeepalivePlist(ibkrctlPath string) (string, error) {
	logf := filepath.Join(g.Support, "keepalive.log")
	return writePlist(KeepaliveLabel, []entry{
		{"Label", KeepaliveLabel},
		{"ProgramArguments", []string{ibkrctlPath, "tickle", "--quiet"}},
		{"StartInterval", 60},
		{"RunAtLoad", true},
		{"StandardOutPath", logf},
		{"StandardErrorPath", logf},
	})
}

func launchctl(args ...string) (string, error) {
	out, err := exec.CommandContext(context.Background(), "launchctl", args...).CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

// guiTarget is the per-user GUI launchd domain (gui/<uid>). Running from a
// non-Aqua shell, `launchctl load` targets the wrong domain and silently
// no-ops, so we bootstrap into gui/<uid> explicitly.
func guiTarget() string {
	return "gui/" + strconv.Itoa(os.Getuid())
}

// Load (re)loads a launchd agent by label using bootstrap, falling back to load.
func Load(label string) error {
	p := plistPath(label)
	_, _ = launchctl("bootout", guiTarget()+"/"+label)
	if out, err := launchctl("bootstrap", guiTarget(), p); err != nil {
		// Fallback for older macOS.
		_, _ = launchctl("unload", p)
		if out2, err2 := launchctl("load", "-w", p); err2 != nil {
			return fmt.Errorf("launchctl load failed: %w (%s); bootstrap: %s (%s)", err2, out2, err.Error(), out)
		}
	}
	_, _ = launchctl("enable", guiTarget()+"/"+label)
	_, _ = launchctl("kickstart", guiTarget()+"/"+label)
	return nil
}

// Unload stops and removes a launchd agent.
func Unload(label string) error {
	p := plistPath(label)
	if out, err := launchctl("bootout", guiTarget()+"/"+label); err != nil {
		if out2, err2 := launchctl("unload", "-w", p); err2 != nil {
			return fmt.Errorf("launchctl unload failed: %w (%s); bootout: %s (%s)", err2, out2, err.Error(), out)
		}
	}
	return nil
}

// AgentLoaded reports whether launchd currently lists the label.
func AgentLoaded(label string) bool {
	if out, err := launchctl("print", guiTarget()+"/"+label); err == nil && strings.Contains(out, label) {
		return true
	}
	out, err := launchctl("list")
	if err != nil {
		return false
	}
	return strings.Contains(out, label)
}
