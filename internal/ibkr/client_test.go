package ibkr

import (
	"context"
	"errors"
	"testing"

	"github.com/laurenschristian/ibkrctl/internal/ibkr/ibkrtest"
)

func newClient(t *testing.T) (*Client, *ibkrtest.Server) {
	t.Helper()
	s := ibkrtest.New()
	t.Cleanup(s.Close)
	return New(s.URL), s
}

func TestNewTrimsSlash(t *testing.T) {
	if New("http://x/").BaseURL != "http://x" {
		t.Fatal("BaseURL")
	}
}

func TestAuthAndTickle(t *testing.T) {
	c, s := newClient(t)
	ctx := context.Background()
	st, err := c.AuthStatus(ctx)
	if err != nil || !st.Authenticated {
		t.Fatalf("auth %+v %v", st, err)
	}
	tk, err := c.Tickle(ctx)
	if err != nil || tk.Session != "sess-123" {
		t.Fatalf("tickle %+v %v", tk, err)
	}
	// Logged out -> auth/status maps 401 to not authenticated.
	s.Authenticated = false
	st, err = c.AuthStatus(ctx)
	if err != nil || st.Authenticated {
		t.Fatalf("logged out %+v %v", st, err)
	}
}

func TestReads(t *testing.T) {
	c, _ := newClient(t)
	ctx := context.Background()
	accts, err := c.Accounts(ctx)
	if err != nil || len(accts) != 1 || accts[0].ID != "U1234567" {
		t.Fatalf("accounts %+v %v", accts, err)
	}
	if _, err := c.Positions(ctx, "U1234567", 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Summary(ctx, "U1234567"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.PnL(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Orders(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Snapshot(ctx, []string{"265598"}, []string{"31", "55"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SecdefSearch(ctx, "AAPL"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Strikes(ctx, "265598", "OPT", "JAN27"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SecdefInfo(ctx, "265598", "OPT", "JAN27", "195", "C"); err != nil {
		t.Fatal(err)
	}
}

func TestPlaceReplyCancel(t *testing.T) {
	c, s := newClient(t)
	ctx := context.Background()
	res, err := c.PlaceOrder(ctx, "U1234567", map[string]any{"conid": 265598, "side": "BUY", "quantity": 1.0, "orderType": "MKT", "tif": "DAY"})
	if err != nil {
		t.Fatal(err)
	}
	arr, ok := res.([]any)
	if !ok || len(arr) == 0 {
		t.Fatalf("place resp %T", res)
	}
	if s.LastOrder["side"] != "BUY" {
		t.Fatalf("order not recorded: %+v", s.LastOrder)
	}
	if _, err := c.Reply(ctx, "reply-1", true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CancelOrder(ctx, "U1234567", "999"); err != nil {
		t.Fatal(err)
	}
	if s.Canceled != "999" {
		t.Fatalf("cancel not recorded: %q", s.Canceled)
	}
}

func TestLogoutAndRaw(t *testing.T) {
	c, s := newClient(t)
	ctx := context.Background()
	if _, err := c.Raw(ctx, "GET", "iserver/accounts", nil); err != nil {
		t.Fatal(err)
	}
	if err := c.Logout(ctx); err != nil {
		t.Fatal(err)
	}
	if s.Authenticated {
		t.Fatal("still authed")
	}
}

func TestErrorMapping(t *testing.T) {
	c := New("http://127.0.0.1:0")
	if _, err := c.PnL(context.Background()); err == nil {
		t.Fatal("want conn error")
	}
	ae := &APIError{Status: 500, Path: "x", Body: "boom"}
	if ae.Error() == "" {
		t.Fatal("empty error")
	}
	if !errors.Is(ErrNotAuthenticated, ErrNotAuthenticated) {
		t.Fatal("sentinel")
	}
}

func TestResearchEndpoints(t *testing.T) {
	c, _ := newClient(t)
	ctx := context.Background()
	if _, err := c.Ledger(ctx, "U1234567"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Allocation(ctx, "U1234567"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.History(ctx, "265598", "1m", "1d", true); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Trades(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SecdefByConid(ctx, []string{"265598"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ScannerParams(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.RunScanner(ctx, map[string]any{"instrument": "STK", "type": "TOP_PERC_GAIN", "location": "STK.US.MAJOR"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Fundamentals(ctx, "265598"); err != nil {
		t.Fatal(err)
	}
}

func TestMarketsEndpoints(t *testing.T) {
	c, _ := newClient(t)
	ctx := context.Background()
	if _, err := c.Watchlists(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Watchlist(ctx, "101"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.CreateWatchlist(ctx, "9", "Test", []string{"265598"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteWatchlist(ctx, "9"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.News(ctx, []string{"265598"}, 5); err != nil {
		t.Fatal(err)
	}
	if _, err := c.News(ctx, nil, 0); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Notifications(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.UnreadCount(ctx); err != nil {
		t.Fatal(err)
	}
	if _, err := c.ExchangeRate(ctx, "EUR", "USD"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Futures(ctx, []string{"ES"}); err != nil {
		t.Fatal(err)
	}
	if _, err := c.SearchSecType(ctx, "EUR", "CASH"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Alerts(ctx, "U1234567"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Alert(ctx, "U1234567", "a1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.DeleteAlert(ctx, "U1234567", "a1"); err != nil {
		t.Fatal(err)
	}
	if _, err := c.Transactions(ctx, "U1234567", []string{"265598"}, 30); err != nil {
		t.Fatal(err)
	}
}
