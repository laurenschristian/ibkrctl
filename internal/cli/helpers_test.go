package cli

import (
	"context"
	"strings"
	"testing"

	"github.com/laurenschristian/ibkrctl/internal/config"
	"github.com/laurenschristian/ibkrctl/internal/ibkr"
	"github.com/laurenschristian/ibkrctl/internal/ibkr/ibkrtest"
)

func TestFirstConid(t *testing.T) {
	if id, ok := firstConid([]any{map[string]any{"conid": "265598"}}); !ok || id != "265598" {
		t.Fatalf("string conid %q %v", id, ok)
	}
	if id, ok := firstConid([]any{map[string]any{"conid": float64(111)}}); !ok || id != "111" {
		t.Fatalf("num conid %q %v", id, ok)
	}
	if _, ok := firstConid([]any{}); ok {
		t.Fatal("empty should fail")
	}
	if _, ok := firstConid("nope"); ok {
		t.Fatal("non-array should fail")
	}
	if _, ok := firstConid([]any{map[string]any{}}); ok {
		t.Fatal("no conid should fail")
	}
}

func TestRawBody(t *testing.T) {
	if v := rawBody(`{"a":1}`); v.(map[string]any)["a"].(float64) != 1 {
		t.Fatalf("json %v", v)
	}
	if v := rawBody("plain"); v != "plain" {
		t.Fatalf("plain %v", v)
	}
}

func TestOrNoneAndNoSetup(t *testing.T) {
	if orNone("") != "(unset)" || orNone("x") != "x" {
		t.Fatal("orNone")
	}
	root := Root()
	for _, c := range root.Commands() {
		if c.Name() == "completion" && !noSetup(c) {
			t.Fatal("completion should be exempt")
		}
	}
}

func TestResolveAccount(t *testing.T) {
	s := ibkrtest.New()
	defer s.Close()
	client = ibkr.New(s.URL)
	cfg = &config.Config{}
	ctx := context.Background()
	if a, err := resolveAccount(ctx, "FLAG"); err != nil || a != "FLAG" {
		t.Fatalf("flag %q %v", a, err)
	}
	cfg.Account = "CFG"
	if a, _ := resolveAccount(ctx, ""); a != "CFG" {
		t.Fatalf("cfg %q", a)
	}
	cfg.Account = ""
	if a, err := resolveAccount(ctx, ""); err != nil || a != "U1234567" {
		t.Fatalf("first %q %v", a, err)
	}
}

func TestAnswerReplies(t *testing.T) {
	s := ibkrtest.New()
	defer s.Close()
	client = ibkr.New(s.URL)
	// A prompt (has id+message) should be auto-confirmed into an ack.
	prompt := []any{map[string]any{"id": "reply-1", "message": []any{"Confirm"}}}
	out, err := answerReplies(context.Background(), prompt)
	if err != nil {
		t.Fatal(err)
	}
	arr := out.([]any)
	if arr[0].(map[string]any)["order_status"] != "Submitted" {
		t.Fatalf("not confirmed: %v", out)
	}
	// A plain ack (no message) passes through unchanged.
	ack := []any{map[string]any{"order_id": "1"}}
	if out, _ := answerReplies(context.Background(), ack); !strings.Contains(out.([]any)[0].(map[string]any)["order_id"].(string), "1") {
		t.Fatalf("ack changed: %v", out)
	}
}
