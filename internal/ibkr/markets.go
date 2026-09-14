package ibkr

import (
	"context"
	"net/url"
	"strings"
)

// Watchlists returns all watchlists (system + user).
func (c *Client) Watchlists(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "iserver/watchlists", nil)
}

// Watchlist returns one watchlist's instruments.
func (c *Client) Watchlist(ctx context.Context, id string) (any, error) {
	return c.Raw(ctx, "GET", "iserver/watchlist?id="+url.QueryEscape(id), nil)
}

// CreateWatchlist creates a watchlist from conids.
func (c *Client) CreateWatchlist(ctx context.Context, id, name string, conids []string) (any, error) {
	rows := make([]map[string]any, 0, len(conids))
	for _, cid := range conids {
		rows = append(rows, map[string]any{"C": cid})
	}
	return c.Raw(ctx, "POST", "iserver/watchlist", map[string]any{"id": id, "name": name, "rows": rows})
}

// DeleteWatchlist removes a watchlist.
func (c *Client) DeleteWatchlist(ctx context.Context, id string) (any, error) {
	return c.Raw(ctx, "DELETE", "iserver/watchlist?id="+url.QueryEscape(id), nil)
}

// News returns top news, optionally filtered to conids.
func (c *Client) News(ctx context.Context, conids []string, num int) (any, error) {
	q := url.Values{}
	if len(conids) > 0 {
		q.Set("conids", strings.Join(conids, ","))
	}
	if num > 0 {
		q.Set("num", itoa(num))
	}
	p := "iserver/news/top"
	if e := q.Encode(); e != "" {
		p += "?" + e
	}
	return c.Raw(ctx, "GET", p, nil)
}

// Notifications returns recent FYI notifications.
func (c *Client) Notifications(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "fyi/notifications", nil)
}

// UnreadCount returns the number of unread FYI notifications.
func (c *Client) UnreadCount(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "fyi/unreadnumber", nil)
}

// ExchangeRate returns the spot rate for a currency pair.
func (c *Client) ExchangeRate(ctx context.Context, target, source string) (any, error) {
	q := url.Values{}
	q.Set("target", target)
	q.Set("source", source)
	return c.Raw(ctx, "GET", "iserver/exchangerate?"+q.Encode(), nil)
}

// Futures returns the futures contracts for underlying symbols.
func (c *Client) Futures(ctx context.Context, symbols []string) (any, error) {
	q := url.Values{}
	q.Set("symbols", strings.Join(symbols, ","))
	return c.Raw(ctx, "GET", "trsrv/futures?"+q.Encode(), nil)
}

// SearchSecType resolves a symbol for a specific security type (STK, CASH, FUT, ...).
func (c *Client) SearchSecType(ctx context.Context, symbol, secType string) (any, error) {
	body := map[string]any{"symbol": symbol}
	if secType != "" {
		body["secType"] = secType
	}
	return c.Raw(ctx, "POST", "iserver/secdef/search", body)
}

// Alerts returns the alerts configured for an account.
func (c *Client) Alerts(ctx context.Context, accountID string) (any, error) {
	return c.Raw(ctx, "GET", "iserver/account/"+url.PathEscape(accountID)+"/alerts", nil)
}

// Alert returns one alert's detail.
func (c *Client) Alert(ctx context.Context, accountID, alertID string) (any, error) {
	return c.Raw(ctx, "GET", "iserver/account/"+url.PathEscape(accountID)+"/alert/"+url.PathEscape(alertID), nil)
}

// CreateAlert creates or updates an alert. body follows the IBKR alert schema
// (alertName, alertMessage, alertRepeatable, conditions[]).
func (c *Client) CreateAlert(ctx context.Context, accountID string, body map[string]any) (any, error) {
	return c.Raw(ctx, "POST", "iserver/account/"+url.PathEscape(accountID)+"/alert", body)
}

// DeleteAlert removes an alert.
func (c *Client) DeleteAlert(ctx context.Context, accountID, alertID string) (any, error) {
	return c.Raw(ctx, "DELETE", "iserver/account/"+url.PathEscape(accountID)+"/alert/"+url.PathEscape(alertID), nil)
}

// Transactions returns transaction history for conids over a window of days.
func (c *Client) Transactions(ctx context.Context, accountID string, conids []string, days int) (any, error) {
	ids := make([]any, 0, len(conids))
	for _, cid := range conids {
		ids = append(ids, cid)
	}
	if days <= 0 {
		days = 90
	}
	body := map[string]any{"acctIds": []any{accountID}, "conids": ids, "currency": "USD", "days": days}
	return c.Raw(ctx, "POST", "pa/transactions", body)
}

// Performance returns time-weighted returns / NAV history for accounts over a
// period (1D, 1M, 1Y, YTD, ...). Portfolio Analyst endpoint (POST).
func (c *Client) Performance(ctx context.Context, accountIDs []string, period string) (any, error) {
	body := map[string]any{"acctIds": accountIDs, "period": period}
	return c.Raw(ctx, "POST", "pa/performance", body)
}

// AllPeriods returns performance across every standard period at once (POST).
func (c *Client) AllPeriods(ctx context.Context, accountIDs []string) (any, error) {
	body := map[string]any{"acctIds": accountIDs}
	return c.Raw(ctx, "POST", "pa/allperiods", body)
}

// FundamentalsSummary returns the Refinitiv company overview and analyst
// forecast for a contract (richer than the ratios snapshot).
func (c *Client) FundamentalsSummary(ctx context.Context, conid string) (any, error) {
	return c.Raw(ctx, "GET", "iserver/fundamentals/"+url.PathEscape(conid)+"/summary", nil)
}

// OrdersFiltered returns live/historical orders filtered by status
// (e.g. Filled, Cancelled, Submitted, Inactive). Empty filters = all live.
func (c *Client) OrdersFiltered(ctx context.Context, filters string) (any, error) {
	path := "iserver/account/orders"
	if filters != "" {
		path += "?filters=" + url.QueryEscape(filters)
	}
	return c.Raw(ctx, "GET", path, nil)
}

// StocksBySymbol resolves several stock symbols to contracts in one call.
func (c *Client) StocksBySymbol(ctx context.Context, symbols []string) (any, error) {
	q := url.Values{}
	q.Set("symbols", strings.Join(symbols, ","))
	return c.Raw(ctx, "GET", "trsrv/stocks?"+q.Encode(), nil)
}

// CurrencyPairs lists tradable FX pairs for a base currency.
func (c *Client) CurrencyPairs(ctx context.Context, currency string) (any, error) {
	q := url.Values{}
	q.Set("currency", currency)
	return c.Raw(ctx, "GET", "iserver/currency/pairs?"+q.Encode(), nil)
}

// TradingSchedule returns trading hours/sessions for a symbol.
func (c *Client) TradingSchedule(ctx context.Context, assetClass, symbol, exchange, exchangeFilter string) (any, error) {
	q := url.Values{}
	q.Set("assetClass", assetClass)
	q.Set("symbol", symbol)
	if exchange != "" {
		q.Set("exchange", exchange)
	}
	if exchangeFilter != "" {
		q.Set("exchangeFilter", exchangeFilter)
	}
	return c.Raw(ctx, "GET", "trsrv/secdef/schedule?"+q.Encode(), nil)
}

// MarkNotificationRead marks a single FYI notification read.
func (c *Client) MarkNotificationRead(ctx context.Context, id string) (any, error) {
	return c.Raw(ctx, "PUT", "fyi/notifications/"+url.PathEscape(id), nil)
}

// NotificationSettings returns FYI notification type settings.
func (c *Client) NotificationSettings(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "fyi/settings", nil)
}

// DeliveryOptions returns configured notification delivery devices/emails.
func (c *Client) DeliveryOptions(ctx context.Context) (any, error) {
	return c.Raw(ctx, "GET", "fyi/deliveryoptions", nil)
}
