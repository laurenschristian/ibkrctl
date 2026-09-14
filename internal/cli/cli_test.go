package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/laurenschristian/ibkrctl/internal/ibkr/ibkrtest"
)

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	flagJSON = false
	root := Root()
	root.SetArgs(args)
	err := root.Execute()
	_ = w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	return buf.String(), err
}

// withGateway points config at a fake gateway and an isolated config file.
func withGateway(t *testing.T) *ibkrtest.Server {
	t.Helper()
	s := ibkrtest.New()
	t.Cleanup(s.Close)
	t.Setenv("IBKR_CONFIG", os.DevNull)
	t.Setenv("IBKR_URL", s.URL)
	return s
}

func TestStatusAndDoctor(t *testing.T) {
	withGateway(t)
	out, err := run(t, "status")
	if err != nil || !strings.Contains(out, "authenticated  true") {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, _ := run(t, "status", "--json"); !strings.Contains(out, "\"authenticated\"") {
		t.Fatalf("json %s", out)
	}
	out, err = run(t, "doctor")
	if err != nil || !strings.Contains(out, "authenticated   true") {
		t.Fatalf("doctor %v\n%s", err, out)
	}
	if out, _ := run(t, "doctor", "--json"); !strings.Contains(out, "\"authenticated\"") {
		t.Fatalf("doctor json %s", out)
	}
}

func TestAccountAndTickle(t *testing.T) {
	withGateway(t)
	out, err := run(t, "account")
	if err != nil || !strings.Contains(out, "U1234567") {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, _ := run(t, "account", "--json"); !strings.Contains(out, "U1234567") {
		t.Fatalf("json %s", out)
	}
	out, err = run(t, "tickle")
	if err != nil || !strings.Contains(out, "sess-123") {
		t.Fatalf("tickle %v\n%s", err, out)
	}
	if _, err := run(t, "tickle", "--quiet"); err != nil {
		t.Fatalf("quiet tickle %v", err)
	}
}

func TestReadCommands(t *testing.T) {
	withGateway(t)
	for _, c := range [][]string{
		{"positions"}, {"pnl"}, {"orders"}, {"quote", "265598"}, {"chain", "AAPL", "--month", "JAN27"}, {"raw", "iserver/accounts"},
	} {
		if out, err := run(t, c...); err != nil {
			t.Fatalf("%v -> %v\n%s", c, err, out)
		}
	}
}

func TestPlaceDryRunAndConfirm(t *testing.T) {
	withGateway(t)
	out, err := run(t, "place", "265598", "--side", "BUY", "--qty", "1")
	if err != nil || !strings.Contains(out, "DRY RUN") {
		t.Fatalf("dry run %v\n%s", err, out)
	}
	out, err = run(t, "place", "265598", "--side", "BUY", "--qty", "1", "--confirm")
	if err != nil || !strings.Contains(out, "order_id") {
		t.Fatalf("confirm %v\n%s", err, out)
	}
	// Bad side.
	if _, err := run(t, "place", "265598", "--side", "HODL", "--qty", "1", "--confirm"); err == nil {
		t.Fatal("want bad side error")
	}
	// LMT without price.
	if _, err := run(t, "place", "265598", "--side", "BUY", "--qty", "1", "--type", "LMT", "--confirm"); err == nil {
		t.Fatal("want price required error")
	}
}

func TestCancel(t *testing.T) {
	s := withGateway(t)
	if _, err := run(t, "cancel", "999"); err != nil {
		t.Fatal(err)
	}
	if s.Canceled != "999" {
		t.Fatalf("canceled %q", s.Canceled)
	}
}

func TestGatewayStatus(t *testing.T) {
	withGateway(t)
	out, err := run(t, "gateway", "status")
	if err != nil || !strings.Contains(out, "installed") {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, _ := run(t, "gateway", "status", "--json"); !strings.Contains(out, "\"installed\"") {
		t.Fatalf("json %s", out)
	}
}

func TestMCPToolsList(t *testing.T) {
	withGateway(t)
	cfgReload(t)
	s := mcpServer()
	ct, st := mcp.NewInMemoryTransports()
	go func() { _ = s.Run(context.Background(), st) }()
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "t"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	tools, err := sess.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) < 8 {
		t.Fatalf("%v tools=%d", err, len(tools.Tools))
	}
	for _, name := range []string{"ibkr_status", "ibkr_accounts", "ibkr_positions", "ibkr_pnl", "ibkr_orders", "ibkr_quote", "ibkr_chain", "ibkr_raw"} {
		res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: mcpArgs(name)})
		if err != nil || res.IsError {
			t.Fatalf("tool %s: %v isErr=%v", name, err, res != nil && res.IsError)
		}
	}
}

func mcpArgs(name string) map[string]any {
	switch name {
	case "ibkr_quote":
		return map[string]any{"conids": []string{"265598"}}
	case "ibkr_chain":
		return map[string]any{"conid": "265598", "month": "JAN27"}
	case "ibkr_raw":
		return map[string]any{"path": "iserver/accounts"}
	}
	return map[string]any{}
}

// cfgReload makes sure the package globals reflect the current env (Root's
// PersistentPreRunE sets them; the MCP test builds the server directly).
func cfgReload(t *testing.T) {
	t.Helper()
	if _, err := run(t, "status"); err != nil {
		t.Fatalf("warmup: %v", err)
	}
}

func TestResearchCommands(t *testing.T) {
	withGateway(t)
	for _, c := range [][]string{
		{"summary"}, {"ledger"}, {"allocation"}, {"trades"},
		{"search", "AAPL"}, {"info", "265598"}, {"history", "265598", "--period", "1m"},
		{"fundamentals", "265598"}, {"scanner", "--list"}, {"scanner"},
	} {
		if out, err := run(t, c...); err != nil {
			t.Fatalf("%v -> %v\n%s", c, err, out)
		}
	}
}

func TestAccountAliasAndRedact(t *testing.T) {
	s := withGateway(t)
	_ = s
	cfgPath := t.TempDir() + "/c.yaml"
	t.Setenv("IBKR_CONFIG", cfgPath)
	if _, err := run(t, "account", "alias", "U1234567", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "account", "redact", "on"); err != nil {
		t.Fatal(err)
	}
	// With redaction on, positions should show the alias, not the real id.
	out, err := run(t, "positions", "--account", "main")
	if err != nil || strings.Contains(out, "U1234567") || !strings.Contains(out, "main") {
		t.Fatalf("redaction not applied: %v\n%s", err, out)
	}
	if _, err := run(t, "account", "redact", "off"); err != nil {
		t.Fatal(err)
	}
}
