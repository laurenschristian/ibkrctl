// Package ibkrtest is an in-memory fake Client Portal Gateway for tests.
package ibkrtest

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	mux.HandleFunc(base+"iserver/marketdata/snapshot", func(w http.ResponseWriter, _ *http.Request) {
		write(w, []any{map[string]any{"conid": 265598, "31": "195.00", "55": "AAPL"}})
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
		default:
			write(w, []any{map[string]any{"acctId": "U1234567", "conid": 265598, "contractDesc": "AAPL", "position": 100.0, "mktValue": 19500.0}})
		}
	})
	// place, reply, cancel, status under iserver/account/
	mux.HandleFunc(base+"iserver/account/", func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, base+"iserver/account/")
		switch {
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
	s.Server = httptest.NewServer(mux)
	return s
}
