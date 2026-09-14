package ibkr

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// AuthStatus is the gateway's brokerage session state.
type AuthStatus struct {
	Authenticated bool   `json:"authenticated"`
	Competing     bool   `json:"competing"`
	Connected     bool   `json:"connected"`
	Fail          string `json:"fail,omitempty"`
	MessageAuth   struct {
		AuthenticatedFlag bool `json:"authenticatedFlag"`
	} `json:"MAA,omitempty"`
}

func (c *Client) AuthStatus(ctx context.Context) (*AuthStatus, error) {
	var s AuthStatus
	err := c.Post(ctx, "iserver/auth/status", nil, &s)
	if err != nil {
		// A logged-out gateway answers 401/403 or an empty body; that is a
		// valid "not authenticated" state, not a transport failure.
		var ae *APIError
		if errors.As(err, &ae) && (ae.Status == 401 || ae.Status == 403 || ae.Status == 404) {
			return &AuthStatus{Authenticated: false}, nil
		}
		if errors.Is(err, ErrNotAuthenticated) {
			return &AuthStatus{Authenticated: false}, nil
		}
		return nil, err
	}
	return &s, nil
}

// Tickle keeps the session alive and returns the session id.
type Tickle struct {
	Session    string `json:"session"`
	SSOExpires int    `json:"ssoExpires"`
	Iserver    struct {
		AuthStatus AuthStatus `json:"authStatus"`
	} `json:"iserver"`
}

func (c *Client) Tickle(ctx context.Context) (*Tickle, error) {
	var t Tickle
	if err := c.Post(ctx, "tickle", nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// Logout ends the brokerage session.
func (c *Client) Logout(ctx context.Context) error {
	return c.Post(ctx, "logout", nil, nil)
}

// Account is one brokerage account.
type Account struct {
	ID           string `json:"id"`
	AccountID    string `json:"accountId"`
	AccountTitle string `json:"accountTitle,omitempty"`
	DisplayName  string `json:"displayName,omitempty"`
	Currency     string `json:"currency,omitempty"`
	Type         string `json:"type,omitempty"`
}

// Accounts returns the accounts the gateway session can trade.
func (c *Client) Accounts(ctx context.Context) ([]Account, error) {
	var out struct {
		Accounts []string          `json:"accounts"`
		Aliases  map[string]string `json:"aliases"`
		Selected string            `json:"selectedAccount"`
	}
	if err := c.Get(ctx, "iserver/accounts", &out); err != nil {
		return nil, err
	}
	accts := make([]Account, 0, len(out.Accounts))
	for _, id := range out.Accounts {
		accts = append(accts, Account{ID: id, AccountID: id, DisplayName: out.Aliases[id]})
	}
	return accts, nil
}

// Summary returns account ledger/summary values.
func (c *Client) Summary(ctx context.Context, accountID string) (any, error) {
	return c.Raw(ctx, "GET", "portfolio/"+url.PathEscape(accountID)+"/summary", nil)
}

// Positions returns a page of positions for an account.
func (c *Client) Positions(ctx context.Context, accountID string, page int) (any, error) {
	return c.Raw(ctx, "GET", fmt.Sprintf("portfolio/%s/positions/%d", url.PathEscape(accountID), page), nil)
}

// PnL returns partitioned live PnL for the session.
func (c *Client) PnL(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "iserver/account/pnl/partitioned", nil)
}

// Orders returns live orders for the session.
func (c *Client) Orders(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "iserver/account/orders", nil)
}

// Snapshot returns market-data fields for conids. fields are IBKR field ids
// (31=last, 84=bid, 86=ask, 88=bidSize, 85=askSize, 87=volume, 55=symbol...).
func (c *Client) Snapshot(ctx context.Context, conids []string, fields []string) (any, error) {
	q := url.Values{}
	q.Set("conids", strings.Join(conids, ","))
	if len(fields) > 0 {
		q.Set("fields", strings.Join(fields, ","))
	}
	path := "iserver/marketdata/snapshot?" + q.Encode()
	// The first snapshot call only primes the subscription (sparse result); the
	// fields arrive on a follow-up call a moment later.
	out, err := c.Raw(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	if snapshotSparse(out) {
		select {
		case <-ctx.Done():
			return out, nil
		case <-time.After(1200 * time.Millisecond):
		}
		if out2, err := c.Raw(ctx, "GET", path, nil); err == nil {
			return out2, nil
		}
	}
	return out, nil
}

// snapshotSparse reports whether a snapshot row carries no quote fields yet.
func snapshotSparse(v any) bool {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return true
	}
	m, ok := arr[0].(map[string]any)
	if !ok {
		return true
	}
	for k := range m {
		if k != "conid" && k != "conidEx" && k != "server_id" && k != "_updated" {
			return false
		}
	}
	return true
}

// SecdefSearch resolves a symbol to contracts.
func (c *Client) SecdefSearch(ctx context.Context, symbol string) (any, error) {
	return c.Raw(ctx, "POST", "iserver/secdef/search", map[string]any{"symbol": symbol})
}

// Strikes lists option strikes for an underlying conid + month.
func (c *Client) Strikes(ctx context.Context, conid, secType, month string) (any, error) {
	q := url.Values{}
	q.Set("conid", conid)
	q.Set("sectype", secType)
	q.Set("month", month)
	return c.Raw(ctx, "GET", "iserver/secdef/strikes?"+q.Encode(), nil)
}

// SecdefInfo resolves a specific option contract (conid+month+strike+right).
func (c *Client) SecdefInfo(ctx context.Context, conid, secType, month, strike, right string) (any, error) {
	q := url.Values{}
	q.Set("conid", conid)
	q.Set("sectype", secType)
	q.Set("month", month)
	if strike != "" {
		q.Set("strike", strike)
	}
	if right != "" {
		q.Set("right", right)
	}
	return c.Raw(ctx, "GET", "iserver/secdef/info?"+q.Encode(), nil)
}

// PlaceOrder submits one order and returns the gateway reply (which may be a
// confirmation prompt with an id to answer via Reply).
func (c *Client) PlaceOrder(ctx context.Context, accountID string, order map[string]any) (any, error) {
	body := map[string]any{"orders": []map[string]any{order}}
	return c.Raw(ctx, "POST", "iserver/account/"+url.PathEscape(accountID)+"/orders", body)
}

// Reply answers a placement confirmation prompt.
func (c *Client) Reply(ctx context.Context, replyID string, confirmed bool) (any, error) {
	return c.Raw(ctx, "POST", "iserver/reply/"+url.PathEscape(replyID), map[string]any{"confirmed": confirmed})
}

// CancelOrder cancels a live order.
func (c *Client) CancelOrder(ctx context.Context, accountID, orderID string) (any, error) {
	return c.Raw(ctx, "DELETE", "iserver/account/"+url.PathEscape(accountID)+"/order/"+url.PathEscape(orderID), nil)
}

// OrderStatus returns the status of one order.
func (c *Client) OrderStatus(ctx context.Context, orderID string) (any, error) {
	return c.Raw(ctx, "GET", "iserver/account/order/status/"+url.PathEscape(orderID), nil)
}

// Ledger returns cash balances by currency for an account.
func (c *Client) Ledger(ctx context.Context, accountID string) (any, error) {
	return c.Raw(ctx, "GET", "portfolio/"+url.PathEscape(accountID)+"/ledger", nil)
}

// Allocation returns positions grouped by asset class, sector, and group.
func (c *Client) Allocation(ctx context.Context, accountID string) (any, error) {
	return c.Raw(ctx, "GET", "portfolio/"+url.PathEscape(accountID)+"/allocation", nil)
}

// AccountPnL returns the account-level (not partitioned) PnL for the session.
func (c *Client) AccountPnL(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "iserver/account/pnl/partitioned", nil)
}

// History returns historical bars for a contract. period e.g. "1y", "6m", "5d";
// bar e.g. "1d", "1h", "5min". outsideRth includes pre/post market.
func (c *Client) History(ctx context.Context, conid, period, bar string, outsideRth bool) (any, error) {
	q := url.Values{}
	q.Set("conid", conid)
	q.Set("period", period)
	q.Set("bar", bar)
	if outsideRth {
		q.Set("outsideRth", "true")
	}
	return c.Raw(ctx, "GET", "iserver/marketdata/history?"+q.Encode(), nil)
}

// Trades returns executions from the current and prior six days.
func (c *Client) Trades(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "iserver/account/trades", nil)
}

// SecdefByConid returns contract detail for one or more conids.
func (c *Client) SecdefByConid(ctx context.Context, conids []string) (any, error) {
	q := url.Values{}
	q.Set("conids", strings.Join(conids, ","))
	return c.Raw(ctx, "GET", "trsrv/secdef?"+q.Encode(), nil)
}

// ScannerParams returns the market-scanner parameter catalog.
func (c *Client) ScannerParams(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "iserver/scanner/params", nil)
}

// RunScanner runs a market scanner. body is the scanner request (instrument,
// type, location, filter).
func (c *Client) RunScanner(ctx context.Context, body map[string]any) (any, error) {
	return c.Raw(ctx, "POST", "iserver/scanner/run", body)
}

// Fundamentals returns a fundamentals ratios snapshot for a contract, using the
// market-data snapshot fields that carry ratio data.
func (c *Client) Fundamentals(ctx context.Context, conid string) (any, error) {
	// 7051 = company name, 7289 = market cap, 7290 = P/E, 7291 = EPS, 7293 = 52w high,
	// 7294 = 52w low, 7295 = open, 7296 = close, 7286 = dividend amount, 7287 = dividend yield.
	fields := []string{"55", "7051", "7289", "7290", "7291", "7287", "7293", "7294"}
	return c.Snapshot(ctx, []string{conid}, fields)
}

// MarketDataUnsubscribe releases the market-data line for a conid.
func (c *Client) MarketDataUnsubscribe(ctx context.Context, conid string) error {
	return c.Post(ctx, "iserver/marketdata/"+url.PathEscape(conid)+"/unsubscribe", nil, nil)
}
