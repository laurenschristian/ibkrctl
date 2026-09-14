package cli

import (
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
	return &cobra.Command{
		Use:   "get <id>",
		Short: "Show one watchlist's instruments",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.Watchlist(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
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
	return c
}

func fxCmd() *cobra.Command {
	var source string
	c := &cobra.Command{
		Use:   "fx <currency>",
		Short: "Spot exchange rate (e.g. `fx EUR` for EUR/USD)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.ExchangeRate(cmd.Context(), args[0], source)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&source, "source", "USD", "base currency")
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
