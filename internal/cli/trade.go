package cli

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

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
			return show(data, renderPositions)
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
	var filter string
	c := &cobra.Command{
		Use:   "orders",
		Short: "List orders (--filter Filled|Cancelled|Submitted|Inactive)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := client.OrdersFiltered(cmd.Context(), filter)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&filter, "filter", "", "order status filter (comma-separated)")
	c.AddCommand(ordersCancelAllCmd())
	return c
}

func ordersCancelAllCmd() *cobra.Command {
	var account string
	var confirm bool
	c := &cobra.Command{
		Use:   "cancel-all",
		Short: "Cancel every live order (requires --confirm)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			acct, err := resolveAccount(ctx, account)
			if err != nil {
				return err
			}
			ids := liveOrderIDs(ctx)
			if len(ids) == 0 {
				fmt.Println("no live orders")
				return nil
			}
			if !confirm {
				fmt.Printf("DRY RUN cancel-all (pass --confirm): would cancel %d order(s):\n", len(ids))
				return emit(ids)
			}
			results := map[string]any{}
			for _, id := range ids {
				if data, err := client.CancelOrder(ctx, acct, id); err != nil {
					results[id] = map[string]any{"error": err.Error()}
				} else {
					results[id] = data
				}
			}
			return emit(results)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	c.Flags().BoolVar(&confirm, "confirm", false, "actually cancel")
	return c
}

// liveOrderIDs pulls order ids from the live orders payload.
func liveOrderIDs(ctx context.Context) []string {
	data, err := client.Orders(ctx)
	if err != nil {
		return nil
	}
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	list, ok := m["orders"].([]any)
	if !ok {
		return nil
	}
	var ids []string
	for _, o := range list {
		om, ok := o.(map[string]any)
		if !ok {
			continue
		}
		switch v := om["orderId"].(type) {
		case string:
			ids = append(ids, v)
		case float64:
			ids = append(ids, strconv.FormatInt(int64(v), 10))
		}
	}
	return ids
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
	var account, side, orderType, tif, preset, trailingType string
	var qty float64
	var price, takeProfit, stop, auxPrice, trailingAmt float64
	var confirm, preview, bracket, closePos bool
	c := &cobra.Command{
		Use:   "place <conid>",
		Short: "Place an order (requires --confirm to actually submit)",
		Long: "Place an order. Dry-run by default; --preview shows margin/commission, " +
			"--confirm submits. --bracket adds a take-profit and/or stop child order. " +
			"--preset applies a saved order shape (see `ibkrctl presets`).",
		Args: cobra.ExactArgs(1),
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
			if closePos {
				cs, cq, err := closingOrder(ctx, acct, args[0])
				if err != nil {
					return err
				}
				side, qty, orderType = cs, cq, "MKT"
			}
			// A preset fills unset fields and can imply a bracket via pct offsets.
			if preset != "" {
				ps, ok := cfg.PresetByName(preset)
				if !ok {
					return fmt.Errorf("no preset named %q (see `ibkrctl presets`)", preset)
				}
				if side == "" {
					side = ps.Side
				}
				if !cmd.Flags().Changed("type") && ps.Type != "" {
					orderType = ps.Type
				}
				if !cmd.Flags().Changed("tif") && ps.TIF != "" {
					tif = ps.TIF
				}
				if qty == 0 {
					qty = ps.Qty
				}
				if ps.TakeProfitPct > 0 || ps.StopLossPct > 0 {
					if price <= 0 {
						return errors.New("--price (entry) is required to price a preset bracket")
					}
					bracket = true
					if takeProfit == 0 && ps.TakeProfitPct > 0 {
						takeProfit = bracketPrice(price, ps.TakeProfitPct, side, true)
					}
					if stop == 0 && ps.StopLossPct > 0 {
						stop = bracketPrice(price, ps.StopLossPct, side, false)
					}
				}
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
			switch strings.ToUpper(orderType) {
			case "LMT":
				if price <= 0 {
					return errors.New("--price is required for a LMT order")
				}
				order["price"] = price
			case "STP":
				if price <= 0 {
					return errors.New("--price (stop trigger) is required for a STP order")
				}
				order["price"] = price
			case "STP_LMT", "STOP_LIMIT":
				if price <= 0 || auxPrice <= 0 {
					return errors.New("STP_LMT needs --price (limit) and --aux-price (stop trigger)")
				}
				order["orderType"] = "STOP_LIMIT"
				order["price"] = price
				order["auxPrice"] = auxPrice
			case "TRAIL", "TRAILING_STOP":
				if trailingAmt <= 0 {
					return errors.New("TRAIL needs --trailing-amt")
				}
				order["orderType"] = "TRAIL"
				order["trailingAmt"] = trailingAmt
				tt := strings.ToLower(trailingType)
				if tt == "" {
					tt = "amt"
				}
				order["trailingType"] = tt
			}
			if preview {
				return show(mustWhatIf(ctx, acct, order), renderWhatIf)
			}
			if bracket {
				return placeBracket(ctx, cmd, acct, order, side, takeProfit, stop, confirm)
			}
			if !confirm {
				fmt.Printf("DRY RUN (pass --preview for margin/commission, --confirm to submit):\n")
				return emit(map[string]any{"account": acct, "order": order})
			}
			data, err := client.PlaceOrder(ctx, acct, order)
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
	c.Flags().StringVar(&account, "account", "", "account id (default: config or first)")
	c.Flags().StringVar(&side, "side", "", "BUY or SELL")
	c.Flags().Float64Var(&qty, "qty", 0, "quantity")
	c.Flags().StringVar(&orderType, "type", "MKT", "order type: MKT or LMT")
	c.Flags().Float64Var(&price, "price", 0, "limit price (for LMT / bracket entry)")
	c.Flags().StringVar(&tif, "tif", "DAY", "time in force: DAY, GTC, IOC")
	c.Flags().BoolVar(&bracket, "bracket", false, "attach take-profit and/or stop child orders")
	c.Flags().Float64Var(&takeProfit, "take-profit", 0, "bracket take-profit limit price")
	c.Flags().Float64Var(&stop, "stop", 0, "bracket stop price")
	c.Flags().StringVar(&preset, "preset", "", "apply a saved preset (see `ibkrctl presets`)")
	c.Flags().BoolVar(&preview, "preview", false, "preview margin/commission/impact (whatif) without submitting")
	c.Flags().Float64Var(&auxPrice, "aux-price", 0, "stop trigger for STP_LMT")
	c.Flags().Float64Var(&trailingAmt, "trailing-amt", 0, "trailing distance for a TRAIL order")
	c.Flags().StringVar(&trailingType, "trailing-type", "amt", "trailing unit: amt or %")
	c.Flags().BoolVar(&closePos, "close", false, "close the current position in this contract (MKT offset)")
	c.Flags().BoolVar(&confirm, "confirm", false, "actually submit the order")
	return c
}

// bracketPrice computes a child price from an entry and a percent offset. A
// take-profit is above entry for a BUY (below for a SELL); a stop is the reverse.
func bracketPrice(entry, pct float64, side string, takeProfit bool) float64 {
	up := (strings.EqualFold(side, "BUY")) == takeProfit
	f := entry * (1 + pct)
	if !up {
		f = entry * (1 - pct)
	}
	return round2(f)
}

func round2(f float64) float64 {
	return float64(int64(f*100+0.5)) / 100
}

// mustWhatIf runs a preview and returns the payload (or an error map to render).
func mustWhatIf(ctx context.Context, acct string, order map[string]any) any {
	data, err := client.WhatIf(ctx, acct, order)
	if err != nil {
		return map[string]any{"error": err.Error()}
	}
	return data
}

// placeBracket builds an entry plus take-profit/stop children in one submission.
func placeBracket(ctx context.Context, cmd *cobra.Command, acct string, entry map[string]any, side string, takeProfit, stop float64, confirm bool) error {
	if takeProfit <= 0 && stop <= 0 {
		return errors.New("--bracket needs --take-profit and/or --stop")
	}
	coid := fmt.Sprintf("ibkrctl-%d", time.Now().UnixNano())
	entry["cOID"] = coid
	opposite := "SELL"
	if strings.EqualFold(side, "SELL") {
		opposite = "BUY"
	}
	orders := []map[string]any{entry}
	child := func(ot string, p float64) map[string]any {
		return map[string]any{
			"conid": entry["conid"], "side": opposite, "quantity": entry["quantity"],
			"orderType": ot, "price": p, "tif": "GTC", "parentId": coid,
		}
	}
	if takeProfit > 0 {
		orders = append(orders, child("LMT", takeProfit))
	}
	if stop > 0 {
		orders = append(orders, child("STP", stop))
	}
	if !confirm {
		fmt.Printf("DRY RUN bracket (pass --confirm to submit):\n")
		return emit(map[string]any{"account": acct, "orders": orders})
	}
	data, err := client.PlaceOrders(ctx, acct, orders)
	if err != nil {
		return err
	}
	data, err = answerReplies(ctx, data)
	if err != nil {
		return err
	}
	return emit(data)
}

// closingOrder reads the current position and returns the offsetting side and
// quantity to flatten it.
func closingOrder(ctx context.Context, acct, conid string) (string, float64, error) {
	data, err := client.Position(ctx, acct, conid)
	if err != nil {
		return "", 0, err
	}
	rows, ok := data.([]any)
	if !ok || len(rows) == 0 {
		return "", 0, fmt.Errorf("no position in conid %s to close", conid)
	}
	m, ok := rows[0].(map[string]any)
	if !ok {
		return "", 0, fmt.Errorf("unexpected position shape for conid %s", conid)
	}
	pos, _ := m["position"].(float64)
	if pos == 0 {
		return "", 0, fmt.Errorf("position in conid %s is flat", conid)
	}
	if pos > 0 {
		return "SELL", pos, nil
	}
	return "BUY", -pos, nil
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
