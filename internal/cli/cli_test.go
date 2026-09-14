package cli

import (
	"bytes"
	"context"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
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

func TestDoctorAndVersion(t *testing.T) {
	t.Setenv("IBKR_CONFIG", os.DevNull)
	t.Setenv("IBKR_URL", "http://x")
	out, err := run(t, "doctor")
	if err != nil || !strings.Contains(out, "url") {
		t.Fatalf("%v\n%s", err, out)
	}
	if out, _ := run(t, "doctor", "--json"); !strings.Contains(out, "\"url\"") {
		t.Fatalf("json %s", out)
	}
}

func TestMCPPing(t *testing.T) {
	s := mcpServer()
	ct, st := mcp.NewInMemoryTransports()
	go func() { _ = s.Run(context.Background(), st) }()
	sess, err := mcp.NewClient(&mcp.Implementation{Name: "t"}, nil).Connect(context.Background(), ct, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sess.Close() }()
	tools, err := sess.ListTools(context.Background(), nil)
	if err != nil || len(tools.Tools) != 1 {
		t.Fatalf("%v tools=%d", err, len(tools.Tools))
	}
	res, err := sess.CallTool(context.Background(), &mcp.CallToolParams{Name: "ibkr_ping"})
	if err != nil || res.IsError {
		t.Fatalf("ping %v", err)
	}
}
