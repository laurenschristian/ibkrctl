// Package ibkrtest is an in-memory fake Client Portal Gateway for tests.
package ibkrtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
)

// Server is a stateful fake gateway. Authenticated toggles the session state.
type Server struct {
	*httptest.Server
	Authenticated bool
	LastOrder     map[string]any
	Canceled      string
}

// New returns a started fake gateway. Close it when done.
func New() *Server {
	s := &Server{Authenticated: true}
	mux := http.NewServeMux()
	base := "/v1/api/"

	write := func(w http.ResponseWriter, v any) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(v)
	}

	mux.HandleFunc(base+"iserver/auth/status", func(w http.ResponseWriter, _ *http.Request) {
		if !s.Authenticated {
			http.Error(w, "Access Denied", http.StatusUnauthorized)
			return
		}
		write(w, map[string]any{"authenticated": true, "connected": true, "competing": false})
	})
	mux.HandleFunc(base+"tickle", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"session": "sess-123", "iserver": map[string]any{"authStatus": map[string]any{"authenticated": s.Authenticated}}})
	})
	mux.HandleFunc(base+"logout", func(w http.ResponseWriter, _ *http.Request) {
		s.Authenticated = false
		write(w, map[string]any{"status": true})
	})
	mux.HandleFunc(base+"iserver/accounts", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"accounts": []string{"U1234567"}, "aliases": map[string]string{"U1234567": "Individual"}, "selectedAccount": "U1234567"})
	})
	mux.HandleFunc(base+"iserver/account/pnl/partitioned", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"upnl": map[string]any{"U1234567.Core": map[string]any{"dpl": 12.5}}})
	})
	mux.HandleFunc(base+"iserver/account/orders", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"orders": []any{}})
	})
	mux.HandleFunc(base+"iserver/marketdata/snapshot", func(w http.ResponseWriter, r *http.Request) {
		ids := r.URL.Query().Get("conids")
		var out []any
		for _, id := range strings.Split(ids, ",") {
			if id == "" {
				continue
			}
			n, _ := strconv.Atoi(id)
			out = append(out, map[string]any{"conid": n, "31": "195.00", "55": "SYM", "84": "194.90", "86": "195.10", "87": "1000"})
		}
		if len(out) == 0 {
			out = []any{map[string]any{"conid": 265598, "31": "195.00", "55": "AAPL"}}
		}
		write(w, out)
	})
	mux.HandleFunc(base+"iserver/secdef/search", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []any{map[string]any{"conid": "265598", "symbol": "AAPL"}})
	})
	mux.HandleFunc(base+"iserver/secdef/strikes", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"call": []float64{190, 195, 200}, "put": []float64{190, 195, 200}})
	})
	mux.HandleFunc(base+"iserver/secdef/info", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []any{map[string]any{"conid": 111, "strike": 195, "right": "C"}})
	})
	// portfolio/{acct}/positions/{page} and /summary
	mux.HandleFunc(base+"portfolio/", func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/summary"):
			write(w, map[string]any{"netliquidation": map[string]any{"amount": 100000.0}, "accountcode": map[string]any{"value": "U1234567"}})
		case strings.HasSuffix(r.URL.Path, "/ledger"):
			write(w, map[string]any{"USD": map[string]any{"cashbalance": 3771.98, "currency": "USD", "acctcode": "U1234567"}})
		case strings.HasSuffix(r.URL.Path, "/allocation"):
			write(w, map[string]any{"assetClass": map[string]any{"long": map[string]any{"STK": 93047.1, "CASH": 3771.98}}})
		case strings.HasSuffix(r.URL.Path, "/meta"):
			write(w, map[string]any{"accountId": "U1234567", "accountTitle": "Jane Public", "type": "INDIVIDUAL"})
		case strings.Contains(r.URL.Path, "/position/"):
			write(w, []any{map[string]any{"acctId": "U1234567", "conid": 265598, "contractDesc": "AAPL", "position": 100.0}})
		default:
			write(w, []any{map[string]any{"acctId": "U1234567", "conid": 265598, "contractDesc": "AAPL", "position": 100.0, "mktValue": 19500.0}})
		}
	})
	// place, reply, cancel, status under iserver/account/
	mux.HandleFunc(base+"iserver/account/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, base+"iserver/account/")
		switch {
		case strings.HasSuffix(p, "/orders/whatif") && r.Method == http.MethodPost:
			write(w, map[string]any{
				"amount":  map[string]any{"amount": "100.00 USD", "commission": "1.00 USD", "total": "101.00 USD"},
				"equity":  map[string]any{"current": "100000", "change": "0", "after": "100000"},
				"initial": map[string]any{"current": "10000", "change": "100", "after": "10100"},
				"warn":    "",
			})
		case strings.Contains(p, "/order/") && r.Method == http.MethodPost:
			write(w, []any{map[string]any{"order_id": "888", "order_status": "Submitted"}})
		case strings.HasSuffix(p, "/orders") && r.Method == http.MethodPost:
			var body struct {
				Orders []map[string]any `json:"orders"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			if len(body.Orders) > 0 {
				s.LastOrder = body.Orders[0]
			}
			// First response is a confirmation prompt.
			write(w, []any{map[string]any{"id": "reply-1", "message": []any{"Confirm your order"}}})
		case strings.Contains(p, "order/") && r.Method == http.MethodDelete:
			i := strings.Index(p, "order/")
			s.Canceled = p[i+len("order/"):]
			write(w, map[string]any{"msg": "Request was submitted", "order_id": s.Canceled})
		case strings.HasSuffix(p, "/alerts"):
			write(w, []any{})
		case strings.Contains(p, "/alert/") && r.Method == http.MethodDelete:
			write(w, map[string]any{"deleted": true})
		case strings.Contains(p, "/alert/"):
			write(w, map[string]any{"alertId": "a1", "alertName": "x"})
		default:
			write(w, map[string]any{})
		}
	})
	mux.HandleFunc(base+"iserver/reply/", func(w http.ResponseWriter, _ *http.Request) {
		// After confirming, return an order ack (no message field).
		write(w, []any{map[string]any{"order_id": "999", "order_status": "Submitted"}})
	})

	mux.HandleFunc(base+"iserver/marketdata/history", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"symbol": "AAPL", "data": []any{map[string]any{"o": 100.0, "c": 101.0, "h": 102.0, "l": 99.0, "t": 1789405846192}}})
	})
	mux.HandleFunc(base+"iserver/account/trades", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []any{map[string]any{"execution_id": "e1", "symbol": "AAPL", "side": "B", "size": 10.0, "account": "U1234567"}})
	})
	mux.HandleFunc(base+"trsrv/secdef", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"secdef": []any{map[string]any{"conid": 265598, "ticker": "AAPL"}}})
	})
	mux.HandleFunc(base+"iserver/scanner/params", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"scan_type_list": []any{map[string]any{"code": "TOP_PERC_GAIN"}}})
	})
	mux.HandleFunc(base+"iserver/scanner/run", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"contracts": []any{map[string]any{"conid": 265598, "symbol": "AAPL"}}})
	})
	mux.HandleFunc(base+"iserver/watchlists", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"data": map[string]any{"user_lists": []any{map[string]any{"id": "101", "name": "Signal Watchlist"}}}})
	})
	mux.HandleFunc(base+"iserver/watchlist", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			write(w, map[string]any{"deleted": r.URL.Query().Get("id")})
			return
		}
		if r.Method == http.MethodPost {
			write(w, map[string]any{"created": true})
			return
		}
		write(w, map[string]any{"id": r.URL.Query().Get("id"), "instruments": []any{map[string]any{"ticker": "GOOGL", "conid": 208813719}}})
	})
	mux.HandleFunc(base+"iserver/news/top", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"news": []any{map[string]any{"headline": "AI capex accelerates"}}})
	})
	mux.HandleFunc(base+"fyi/notifications", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []any{map[string]any{"ID": "n1", "MD": "note"}})
	})
	mux.HandleFunc(base+"fyi/unreadnumber", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"BN": 3})
	})
	mux.HandleFunc(base+"iserver/exchangerate", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"rate": 1.0855})
	})
	mux.HandleFunc(base+"trsrv/futures", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"ES": []any{map[string]any{"conid": 515416632, "expirationDate": 20261218}}})
	})
	mux.HandleFunc(base+"pa/transactions", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"transactions": []any{map[string]any{"cur": "USD", "amt": -100.0}}})
	})
	mux.HandleFunc(base+"iserver/contract/rules", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"orderTypes": []any{"limit", "market"}, "canTradeAcctIds": []any{"U1234567"}})
	})
	mux.HandleFunc(base+"iserver/contract/", func(w http.ResponseWriter, _ *http.Request) {
		write(w, map[string]any{"con_id": 265598, "company_name": "APPLE INC", "orderTypes": []any{"limit"}})
	})
	s.Server = httptest.NewServer(mux)
	return s
}
