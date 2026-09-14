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
	return nil, rawOut{Data: v}, nil
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
