package cli

import (
	"context"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func watchlistsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "watchlists",
		Short: "List watchlists (system and user)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := client.Watchlists(cmd.Context())
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.AddCommand(watchlistGetCmd(), watchlistCreateCmd(), watchlistDeleteCmd())
	return c
}

func watchlistGetCmd() *cobra.Command {
	var quotes bool
	c := &cobra.Command{
		Use:   "get <id>",
		Short: "Show one watchlist's instruments (--quotes merges live prices)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.Watchlist(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			if quotes {
				data = enrichWatchlist(cmd.Context(), data)
			}
			return emit(data)
		},
	}
	c.Flags().BoolVar(&quotes, "quotes", false, "merge last/bid/ask into each instrument")
	return c
}

// enrichWatchlist merges a market-data snapshot into each instrument by conid.
// Any failure returns the watchlist unchanged rather than erroring the command.
func enrichWatchlist(ctx context.Context, data any) any {
	m, ok := data.(map[string]any)
	if !ok {
		return data
	}
	insts, ok := m["instruments"].([]any)
	if !ok {
		return data
	}
	var conids []string
	for _, it := range insts {
		if im, ok := it.(map[string]any); ok {
			if id := conidStr(im["conid"]); id != "" {
				conids = append(conids, id)
			}
		}
	}
	if len(conids) == 0 {
		return data
	}
	snap, err := client.Snapshot(ctx, conids, []string{"31", "84", "86", "87"})
	if err != nil {
		return data
	}
	byConid := map[string]map[string]any{}
	if rows, ok := snap.([]any); ok {
		for _, r := range rows {
			if rm, ok := r.(map[string]any); ok {
				byConid[conidStr(rm["conid"])] = rm
			}
		}
	}
	for _, it := range insts {
		im, ok := it.(map[string]any)
		if !ok {
			continue
		}
		if q, ok := byConid[conidStr(im["conid"])]; ok {
			im["quote"] = map[string]any{"last": q["31"], "bid": q["84"], "ask": q["86"], "volume": q["87"]}
		}
	}
	return data
}

func conidStr(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return strconv.FormatInt(int64(t), 10)
	}
	return ""
}

func watchlistCreateCmd() *cobra.Command {
	var id, name string
	c := &cobra.Command{
		Use:   "create <conid> [conid...]",
		Short: "Create a watchlist from conids",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.CreateWatchlist(cmd.Context(), id, name, args)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&id, "id", "", "watchlist id")
	c.Flags().StringVar(&name, "name", "", "watchlist name")
	return c
}

func watchlistDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete <id>",
		Short: "Delete a watchlist",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.DeleteWatchlist(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func newsCmd() *cobra.Command {
	var conids string
	var num int
	c := &cobra.Command{
		Use:   "news",
		Short: "Top market news (optionally for specific conids)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			var ids []string
			if conids != "" {
				ids = strings.Split(conids, ",")
			}
			data, err := client.News(cmd.Context(), ids, num)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&conids, "conids", "", "comma-separated conids to filter news")
	c.Flags().IntVar(&num, "num", 0, "max headlines")
	return c
}

func notificationsCmd() *cobra.Command {
	var unread bool
	c := &cobra.Command{
		Use:   "notifications",
		Short: "IBKR notifications (FYI); --unread for the unread count",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if unread {
				data, err := client.UnreadCount(cmd.Context())
				if err != nil {
					return err
				}
				return emit(data)
			}
			data, err := client.Notifications(cmd.Context())
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().BoolVar(&unread, "unread", false, "show unread count only")
	c.AddCommand(notificationsReadCmd(), notificationsSettingsCmd())
	return c
}

func notificationsReadCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "read <notificationId>",
		Short: "Mark a notification read",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.MarkNotificationRead(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func notificationsSettingsCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "settings",
		Short: "Notification type settings and delivery options",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := client.NotificationSettings(cmd.Context())
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func fxCmd() *cobra.Command {
	var source string
	var pairs bool
	c := &cobra.Command{
		Use:   "fx <currency>",
		Short: "Spot exchange rate (e.g. `fx EUR`); --pairs lists tradable pairs",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if pairs {
				data, err := client.CurrencyPairs(cmd.Context(), args[0])
				if err != nil {
					return err
				}
				return emit(data)
			}
			data, err := client.ExchangeRate(cmd.Context(), args[0], source)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&source, "source", "USD", "base currency")
	c.Flags().BoolVar(&pairs, "pairs", false, "list tradable FX pairs for the currency")
	return c
}

func futuresCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "futures <symbol> [symbol...]",
		Short: "Futures contracts for underlying symbols (e.g. ES NQ CL)",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.Futures(cmd.Context(), args)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	return c
}

func alertsCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "alerts",
		Short: "List price alerts for an account",
		RunE: func(cmd *cobra.Command, _ []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.Alerts(cmd.Context(), acct)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	c.AddCommand(alertGetCmd(), alertDeleteCmd())
	return c
}

func alertGetCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "get <alertId>",
		Short: "Show one alert's detail",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.Alert(cmd.Context(), acct, args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	return c
}

func alertDeleteCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "delete <alertId>",
		Short: "Delete an alert",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.DeleteAlert(cmd.Context(), acct, args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	return c
}

func transactionsCmd() *cobra.Command {
	var account, conids string
	var days int
	c := &cobra.Command{
		Use:   "transactions <conid> [conid...]",
		Short: "Transaction history for conids over a window of days",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			_ = conids
			data, err := client.Transactions(cmd.Context(), acct, args, days)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	c.Flags().IntVar(&days, "days", 90, "lookback window in days")
	return c
}
