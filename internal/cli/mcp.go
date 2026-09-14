package cli

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"
)

func mcpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "mcp",
		Short: "Run as an MCP server over stdio (for Claude, Cursor, etc.)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return mcpServer().Run(cmd.Context(), &mcp.StdioTransport{})
		},
	}
}

// rawOut wraps a generic value so the MCP output schema stays an object, not
// an array-of-integer (which a bare json.RawMessage would produce).
type rawOut struct {
	Data any `json:"data,omitempty"`
}

func wrap(v any, err error) (*mcp.CallToolResult, rawOut, error) {
	if err != nil {
		return nil, rawOut{}, err
	}
	return nil, rawOut{Data: redact(v)}, nil
}

// noArgs is a lean empty input type (no required fields).
type noArgs struct{}

type accountArg struct {
	Account string `json:"account,omitempty"`
	Page    int    `json:"page,omitempty"`
}

type quoteArg struct {
	Conids []string `json:"conids"`
	Fields []string `json:"fields,omitempty"`
}

type chainArg struct {
	Conid   string `json:"conid"`
	SecType string `json:"sectype,omitempty"`
	Month   string `json:"month"`
}

type conidArg struct {
	Conid string `json:"conid"`
}

type historyArg struct {
	Conid      string `json:"conid"`
	Period     string `json:"period,omitempty"`
	Bar        string `json:"bar,omitempty"`
	OutsideRth bool   `json:"outsideRth,omitempty"`
}

type symbolArg struct {
	Symbol string `json:"symbol"`
}

type scannerArg struct {
	Instrument string `json:"instrument,omitempty"`
	Type       string `json:"type,omitempty"`
	Location   string `json:"location,omitempty"`
}

type watchlistArg struct {
	ID string `json:"id"`
}

type newsArg struct {
	Conids []string `json:"conids,omitempty"`
	Num    int      `json:"num,omitempty"`
}

type fxArg struct {
	Currency string `json:"currency"`
	Source   string `json:"source,omitempty"`
}

type symbolsArg struct {
	Symbols []string `json:"symbols"`
}

type rawArg struct {
	Method string `json:"method,omitempty"`
	Path   string `json:"path"`
}

func mcpServer() *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "ibkrctl", Version: Version}, nil)

	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_status", Description: "Brokerage authentication and session state."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.AuthStatus(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_accounts", Description: "List brokerage accounts."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Accounts(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_positions", Description: "Positions for an account (default account if omitted)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in accountArg) (*mcp.CallToolResult, rawOut, error) {
			acct, err := resolveAccount(ctx, in.Account)
			if err != nil {
				return nil, rawOut{}, err
			}
			return wrap(client.Positions(ctx, acct, in.Page))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_pnl", Description: "Live profit and loss for the session."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.PnL(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_orders", Description: "List live orders."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Orders(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_summary", Description: "Account summary / ledger for an account."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in accountArg) (*mcp.CallToolResult, rawOut, error) {
			acct, err := resolveAccount(ctx, in.Account)
			if err != nil {
				return nil, rawOut{}, err
			}
			return wrap(client.Summary(ctx, acct))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_quote", Description: "Market-data snapshot for contract ids. fields are IBKR field ids."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in quoteArg) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Snapshot(ctx, in.Conids, in.Fields))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_chain", Description: "Option strikes for an underlying conid + month (e.g. JAN27)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in chainArg) (*mcp.CallToolResult, rawOut, error) {
			st := in.SecType
			if st == "" {
				st = "OPT"
			}
			return wrap(client.Strikes(ctx, in.Conid, st, in.Month))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_summary", Description: "Account summary: net liquidation, cash, buying power."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in accountArg) (*mcp.CallToolResult, rawOut, error) {
			acct, err := resolveAccount(ctx, in.Account)
			if err != nil {
				return nil, rawOut{}, err
			}
			return wrap(client.Summary(ctx, acct))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_ledger", Description: "Cash balances by currency for an account."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in accountArg) (*mcp.CallToolResult, rawOut, error) {
			acct, err := resolveAccount(ctx, in.Account)
			if err != nil {
				return nil, rawOut{}, err
			}
			return wrap(client.Ledger(ctx, acct))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_allocation", Description: "Positions grouped by asset class, sector, and group."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in accountArg) (*mcp.CallToolResult, rawOut, error) {
			acct, err := resolveAccount(ctx, in.Account)
			if err != nil {
				return nil, rawOut{}, err
			}
			return wrap(client.Allocation(ctx, acct))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_trades", Description: "Executions from the last seven days."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Trades(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_search", Description: "Resolve a symbol to contracts (conid, exchange, type)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in symbolArg) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.SecdefSearch(ctx, in.Symbol))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_info", Description: "Contract details for a conid."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in conidArg) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.SecdefByConid(ctx, []string{in.Conid}))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_history", Description: "Historical price bars. period (1d,5d,1m,6m,1y,5y), bar (1min,1h,1d,1w)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in historyArg) (*mcp.CallToolResult, rawOut, error) {
			period, bar := in.Period, in.Bar
			if period == "" {
				period = "1y"
			}
			if bar == "" {
				bar = "1d"
			}
			return wrap(client.History(ctx, in.Conid, period, bar, in.OutsideRth))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_fundamentals", Description: "Ratios snapshot: market cap, P/E, EPS, dividend yield, 52w range."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in conidArg) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Fundamentals(ctx, in.Conid))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_scanner", Description: "Run a market scanner. Omit args for TOP_PERC_GAIN on STK.US.MAJOR."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in scannerArg) (*mcp.CallToolResult, rawOut, error) {
			body := map[string]any{
				"instrument": orDefault(in.Instrument, "STK"),
				"type":       orDefault(in.Type, "TOP_PERC_GAIN"),
				"location":   orDefault(in.Location, "STK.US.MAJOR"),
			}
			return wrap(client.RunScanner(ctx, body))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_watchlists", Description: "List watchlists (system and user)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Watchlists(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_watchlist", Description: "Show one watchlist's instruments by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in watchlistArg) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Watchlist(ctx, in.ID))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_news", Description: "Top market news, optionally filtered to conids."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in newsArg) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.News(ctx, in.Conids, in.Num))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_notifications", Description: "IBKR account notifications (FYI)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Notifications(ctx))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_fx", Description: "Spot exchange rate for a currency vs a base (default USD)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in fxArg) (*mcp.CallToolResult, rawOut, error) {
			src := in.Source
			if src == "" {
				src = "USD"
			}
			return wrap(client.ExchangeRate(ctx, in.Currency, src))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_futures", Description: "Futures contracts for underlying symbols (e.g. ES, NQ, CL)."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in symbolsArg) (*mcp.CallToolResult, rawOut, error) {
			return wrap(client.Futures(ctx, in.Symbols))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_alerts", Description: "List price alerts for an account."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in accountArg) (*mcp.CallToolResult, rawOut, error) {
			acct, err := resolveAccount(ctx, in.Account)
			if err != nil {
				return nil, rawOut{}, err
			}
			return wrap(client.Alerts(ctx, acct))
		})
	mcp.AddTool(s, &mcp.Tool{Name: "ibkr_raw", Description: "Call any /v1/api path (GET unless method is set). Read-only use recommended."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in rawArg) (*mcp.CallToolResult, rawOut, error) {
			m := in.Method
			if m == "" {
				m = "GET"
			}
			return wrap(client.Raw(ctx, m, in.Path, nil))
		})
	return s
}

func orDefault(v, d string) string {
	if v == "" {
		return d
	}
	return v
}
