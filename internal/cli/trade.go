package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func positionsCmd() *cobra.Command {
	var account string
	var page int
	c := &cobra.Command{
		Use:   "positions",
		Short: "List positions for an account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.Positions(cmd.Context(), acct, page)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account id (default: config or first)")
	c.Flags().IntVar(&page, "page", 0, "positions page (100 per page)")
	return c
}

func pnlCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pnl",
		Short: "Live profit and loss for the session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := client.PnL(cmd.Context())
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func ordersCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "orders",
		Short: "List live orders",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := client.Orders(cmd.Context())
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func quoteCmd() *cobra.Command {
	var fields string
	c := &cobra.Command{
		Use:   "quote <conid> [conid...]",
		Short: "Market-data snapshot for one or more contract ids",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var fs []string
			if fields != "" {
				fs = strings.Split(fields, ",")
			} else {
				fs = []string{"31", "55", "84", "86", "87", "88", "85"} // last, symbol, bid, ask, volume, bidSize, askSize
			}
			data, err := client.Snapshot(cmd.Context(), args, fs)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&fields, "fields", "", "comma-separated IBKR field ids (default: common quote fields)")
	return c
}

func chainCmd() *cobra.Command {
	var month, right, secType string
	c := &cobra.Command{
		Use:   "chain <symbol|conid>",
		Short: "Option chain: resolve strikes (and a contract with --month/--strike/--right)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			arg := args[0]
			conid := arg
			// If it's not numeric, resolve the symbol to an underlying conid.
			if _, err := strconv.Atoi(arg); err != nil {
				res, err := client.SecdefSearch(ctx, arg)
				if err != nil {
					return err
				}
				id, ok := firstConid(res)
				if !ok {
					return fmt.Errorf("could not resolve %q to a conid; use `ibkrctl chain <conid>`", arg)
				}
				conid = id
			}
			if month == "" {
				return errors.New("--month is required, e.g. --month JAN27")
			}
			data, err := client.Strikes(ctx, conid, secType, month)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&month, "month", "", "expiry month, e.g. JAN27")
	c.Flags().StringVar(&right, "right", "", "C or P")
	c.Flags().StringVar(&secType, "sectype", "OPT", "security type (OPT, WAR, ...)")
	return c
}

// firstConid pulls a conid out of a secdef/search response.
func firstConid(v any) (string, bool) {
	arr, ok := v.([]any)
	if !ok || len(arr) == 0 {
		return "", false
	}
	m, ok := arr[0].(map[string]any)
	if !ok {
		return "", false
	}
	switch c := m["conid"].(type) {
	case string:
		return c, true
	case float64:
		return strconv.FormatFloat(c, 'f', -1, 64), true
	}
	return "", false
}

func placeCmd() *cobra.Command {
	var account, side, orderType, tif string
	var qty float64
	var price float64
	var confirm, preview bool
	c := &cobra.Command{
		Use:   "place <conid>",
		Short: "Place an order (requires --confirm to actually submit)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			acct, err := resolveAccount(ctx, account)
			if err != nil {
				return err
			}
			conid, err := strconv.Atoi(args[0])
			if err != nil {
				return fmt.Errorf("conid must be numeric: %w", err)
			}
			side = strings.ToUpper(side)
			if side != "BUY" && side != "SELL" {
				return errors.New("--side must be BUY or SELL")
			}
			order := map[string]any{
				"conid":     conid,
				"side":      side,
				"quantity":  qty,
				"orderType": strings.ToUpper(orderType),
				"tif":       strings.ToUpper(tif),
			}
			if strings.EqualFold(orderType, "LMT") {
				if price <= 0 {
					return errors.New("--price is required for a LMT order")
				}
				order["price"] = price
			}
			if preview {
				data, err := client.WhatIf(ctx, acct, order)
				if err != nil {
					return err
				}
				return emit(data)
			}
			if !confirm {
				fmt.Printf("DRY RUN (pass --preview for margin/commission, --confirm to submit):\n")
				return emit(map[string]any{"account": acct, "order": order})
			}
			data, err := client.PlaceOrder(ctx, acct, order)
			if err != nil {
				return err
			}
			// Auto-answer the initial confirmation prompt(s) the gateway returns.
			data, err = answerReplies(ctx, data)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account id (default: config or first)")
	c.Flags().StringVar(&side, "side", "", "BUY or SELL")
	c.Flags().Float64Var(&qty, "qty", 0, "quantity")
	c.Flags().StringVar(&orderType, "type", "MKT", "order type: MKT or LMT")
	c.Flags().Float64Var(&price, "price", 0, "limit price (for LMT)")
	c.Flags().StringVar(&tif, "tif", "DAY", "time in force: DAY, GTC, IOC")
	c.Flags().BoolVar(&preview, "preview", false, "preview margin/commission/impact (whatif) without submitting")
	c.Flags().BoolVar(&confirm, "confirm", false, "actually submit the order")
	return c
}

// answerReplies confirms the gateway's suppressible order-confirmation prompts.
func answerReplies(ctx context.Context, data any) (any, error) {
	for i := 0; i < 5; i++ {
		arr, ok := data.([]any)
		if !ok || len(arr) == 0 {
			return data, nil
		}
		m, ok := arr[0].(map[string]any)
		if !ok {
			return data, nil
		}
		id, ok := m["id"].(string)
		if !ok || m["message"] == nil {
			return data, nil // no more prompts (it's an order ack)
		}
		next, err := client.Reply(ctx, id, true)
		if err != nil {
			return nil, err
		}
		data = next
	}
	return data, nil
}

func cancelCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "cancel <orderId>",
		Short: "Cancel a live order",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.CancelOrder(cmd.Context(), acct, args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account id (default: config or first)")
	return c
}

func rawCmd() *cobra.Command {
	var method string
	var body string
	c := &cobra.Command{
		Use:   "raw <path>",
		Short: "Call any /v1/api path, e.g. `ibkrctl raw iserver/accounts`",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var b any
			if body != "" {
				b = rawBody(body)
			}
			data, err := client.Raw(cmd.Context(), method, args[0], b)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVarP(&method, "method", "X", "GET", "HTTP method")
	c.Flags().StringVar(&body, "body", "", "JSON request body")
	return c
}

// rawBody parses a JSON string into a value, or returns the string as-is.
func rawBody(s string) any {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err == nil {
		return v
	}
	return s
}

func rulesCmd() *cobra.Command {
	var sell bool
	c := &cobra.Command{
		Use:   "rules <conid>",
		Short: "Order rules for a contract (valid order types, increments)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.ContractRules(cmd.Context(), args[0], !sell)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().BoolVar(&sell, "sell", false, "rules for a sell (default: buy)")
	return c
}

func positionCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "position <conid>",
		Short: "Position for a single contract",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.Position(cmd.Context(), acct, args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	return c
}

func modifyCmd() *cobra.Command {
	var account, side, orderType, tif string
	var qty, price float64
	var confirm bool
	c := &cobra.Command{
		Use:   "modify <orderId>",
		Short: "Modify a live order (requires --confirm)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			acct, err := resolveAccount(ctx, account)
			if err != nil {
				return err
			}
			order := map[string]any{}
			if side != "" {
				order["side"] = strings.ToUpper(side)
			}
			if qty > 0 {
				order["quantity"] = qty
			}
			if orderType != "" {
				order["orderType"] = strings.ToUpper(orderType)
			}
			if price > 0 {
				order["price"] = price
			}
			if tif != "" {
				order["tif"] = strings.ToUpper(tif)
			}
			if !confirm {
				fmt.Printf("DRY RUN (pass --confirm to submit):\n")
				return emit(map[string]any{"account": acct, "orderId": args[0], "changes": order})
			}
			data, err := client.ModifyOrder(ctx, acct, args[0], order)
			if err != nil {
				return err
			}
			data, err = answerReplies(ctx, data)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	c.Flags().StringVar(&side, "side", "", "BUY or SELL")
	c.Flags().Float64Var(&qty, "qty", 0, "new quantity")
	c.Flags().StringVar(&orderType, "type", "", "new order type")
	c.Flags().Float64Var(&price, "price", 0, "new limit price")
	c.Flags().StringVar(&tif, "tif", "", "new time in force")
	c.Flags().BoolVar(&confirm, "confirm", false, "actually submit the modification")
	return c
}
