package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestReviewCommand(t *testing.T) {
	withGateway(t)
	out, err := run(t, "review")
	if err != nil || !strings.Contains(out, "Positions") || !strings.Contains(out, "Summary") {
		t.Fatalf("review table %v\n%s", err, out)
	}
	if !strings.Contains(out, "Open orders") {
		t.Fatalf("missing orders line\n%s", out)
	}
	if out, err := run(t, "review", "--json"); err != nil || !strings.Contains(out, "\"summary\"") {
		t.Fatalf("review json %v\n%s", err, out)
	}
}

func TestHumanTables(t *testing.T) {
	withGateway(t)
	cases := []struct{ cmd, want string }{
		{"positions", "SYMBOL"},
		{"summary", "Net liquidation"},
		{"ledger", "CASH"},
		{"allocation", "assetClass"},
	}
	for _, c := range cases {
		out, err := run(t, c.cmd)
		if err != nil || !strings.Contains(out, c.want) {
			t.Fatalf("%s table %v\n%s", c.cmd, err, out)
		}
		// --json falls back to raw JSON, not the table header.
		if j, err := run(t, c.cmd, "--json"); err != nil || strings.Contains(j, c.want+"\t") {
			t.Fatalf("%s json %v\n%s", c.cmd, err, j)
		}
	}
}

func TestPreviewHumanized(t *testing.T) {
	withGateway(t)
	out, err := run(t, "place", "265598", "--side", "BUY", "--qty", "1", "--type", "LMT", "--price", "100", "--preview")
	if err != nil || !strings.Contains(out, "Commission") || !strings.Contains(out, "Initial margin") {
		t.Fatalf("preview %v\n%s", err, out)
	}
}

func TestBracketOrder(t *testing.T) {
	s := withGateway(t)
	out, err := run(t, "place", "265598", "--side", "BUY", "--qty", "1", "--type", "LMT",
		"--price", "100", "--bracket", "--take-profit", "110", "--stop", "95")
	if err != nil || !strings.Contains(out, "DRY RUN bracket") {
		t.Fatalf("bracket dry %v\n%s", err, out)
	}
	out, err = run(t, "place", "265598", "--side", "BUY", "--qty", "1", "--type", "LMT",
		"--price", "100", "--bracket", "--take-profit", "110", "--stop", "95", "--confirm")
	if err != nil || !strings.Contains(out, "order_id") {
		t.Fatalf("bracket confirm %v\n%s", err, out)
	}
	// The parent carried a client order id; children referenced it.
	if s.LastOrder == nil || s.LastOrder["cOID"] == nil {
		t.Fatalf("parent cOID not set: %+v", s.LastOrder)
	}
	// Bracket needs at least one child price.
	if _, err := run(t, "place", "265598", "--side", "BUY", "--qty", "1", "--bracket"); err == nil {
		t.Fatal("expected error for bracket without tp/stop")
	}
}

func TestPresets(t *testing.T) {
	withGateway(t)
	t.Setenv("IBKR_CONFIG", t.TempDir()+"/c.yaml")
	if _, err := run(t, "presets", "add", "scalp", "--side", "BUY", "--qty", "10",
		"--take-profit-pct", "0.05", "--stop-loss-pct", "0.03"); err != nil {
		t.Fatal(err)
	}
	out, err := run(t, "presets")
	if err != nil || !strings.Contains(out, "scalp") {
		t.Fatalf("list %v\n%s", err, out)
	}
	// Preset drives a bracket priced off --price entry.
	out, err = run(t, "place", "265598", "--preset", "scalp", "--price", "100")
	if err != nil || !strings.Contains(out, "DRY RUN bracket") {
		t.Fatalf("preset place %v\n%s", err, out)
	}
	if !strings.Contains(out, "105") || !strings.Contains(out, "97") {
		t.Fatalf("preset bracket prices wrong\n%s", out)
	}
	if _, err := run(t, "presets", "rm", "scalp"); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, "presets", "rm", "nope"); err == nil {
		t.Fatal("expected rm error for missing preset")
	}
}

func TestHistoryCache(t *testing.T) {
	withGateway(t)
	t.Setenv("IBKR_CACHE_DIR", t.TempDir())
	if _, err := run(t, "history", "265598", "--period", "1m"); err != nil {
		t.Fatal(err)
	}
	// Second read is served from cache; still valid output.
	out, err := run(t, "history", "265598", "--period", "1m")
	if err != nil || !strings.Contains(out, "AAPL") {
		t.Fatalf("cached history %v\n%s", err, out)
	}
	if _, err := run(t, "history", "265598", "--period", "1m", "--refresh"); err != nil {
		t.Fatalf("refresh %v", err)
	}
}

func TestWatchlistQuotes(t *testing.T) {
	withGateway(t)
	out, err := run(t, "watchlists", "get", "101", "--quotes")
	if err != nil || !strings.Contains(out, "quote") {
		t.Fatalf("watchlist quotes %v\n%s", err, out)
	}
}

func TestMCPNewTools(t *testing.T) {
	withGateway(t)
	cfgReload(t)
	ctx := context.Background()
	c1, c2 := mcp.NewInMemoryTransports()
	srv := mcpServer()
	go func() { _ = srv.Run(ctx, c1) }()
	cl := mcp.NewClient(&mcp.Implementation{Name: "t", Version: "0"}, nil)
	sess, err := cl.Connect(ctx, c2, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	tools, err := sess.ListTools(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	names := map[string]bool{}
	for _, tl := range tools.Tools {
		names[tl.Name] = true
	}
	for _, want := range []string{"ibkr_review", "ibkr_summary", "ibkr_watchlist"} {
		if !names[want] {
			t.Fatalf("missing MCP tool %s", want)
		}
	}
	res, err := sess.CallTool(ctx, &mcp.CallToolParams{Name: "ibkr_review"})
	if err != nil || res.IsError {
		t.Fatalf("call review: %v %+v", err, res)
	}
}
