package cli

import (
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func summaryCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "summary",
		Short: "Account summary: net liquidation, cash, buying power",
		RunE: func(cmd *cobra.Command, _ []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.Summary(cmd.Context(), acct)
			if err != nil {
				return err
			}
			return show(data, renderSummary)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	return c
}

func ledgerCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "ledger",
		Short: "Cash balances by currency",
		RunE: func(cmd *cobra.Command, _ []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.Ledger(cmd.Context(), acct)
			if err != nil {
				return err
			}
			return show(data, renderLedger)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	return c
}

func allocationCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "allocation",
		Short: "Positions grouped by asset class, sector, and group",
		RunE: func(cmd *cobra.Command, _ []string) error {
			acct, err := resolveAccount(cmd.Context(), account)
			if err != nil {
				return err
			}
			data, err := client.Allocation(cmd.Context(), acct)
			if err != nil {
				return err
			}
			return show(data, renderAllocation)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	return c
}

func tradesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "trades",
		Short: "Executions from the last seven days",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := client.Trades(cmd.Context())
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func searchCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "search <symbol>",
		Short: "Resolve a symbol to contracts (conid, exchange, type)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.SecdefSearch(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	return c
}

func infoCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "info <conid> [conid...]",
		Short: "Contract details for one or more conids",
		Args:  cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.SecdefByConid(cmd.Context(), args)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	return c
}

func historyCmd() *cobra.Command {
	var period, bar string
	var outside, refresh bool
	c := &cobra.Command{
		Use:   "history <conid>",
		Short: "Historical price bars for a contract (cached on disk)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := cachedHistory(cmd.Context(), args[0], period, bar, outside, refresh)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&period, "period", "1y", "lookback: 1d, 5d, 1m, 6m, 1y, 5y")
	c.Flags().StringVar(&bar, "bar", "1d", "bar size: 1min, 5min, 1h, 1d, 1w")
	c.Flags().BoolVar(&outside, "outside-rth", false, "include pre/post market")
	c.Flags().BoolVar(&refresh, "refresh", false, "bypass the on-disk cache")
	return c
}

func fundamentalsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "fundamentals <conid>",
		Short: "Ratios snapshot: market cap, P/E, EPS, dividend yield, 52w range",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.Fundamentals(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	return c
}

func scannerCmd() *cobra.Command {
	var list bool
	var instrument, scanType, location, filter string
	c := &cobra.Command{
		Use:   "scanner",
		Short: "Run a market scanner (--list shows the parameter catalog)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if list {
				data, err := client.ScannerParams(cmd.Context())
				if err != nil {
					return err
				}
				return emit(data)
			}
			body := map[string]any{"instrument": instrument, "type": scanType, "location": location}
			if filter != "" {
				body["filter"] = parseScanFilter(filter)
			}
			data, err := client.RunScanner(cmd.Context(), body)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().BoolVar(&list, "list", false, "list scanner parameters instead of running")
	c.Flags().StringVar(&instrument, "instrument", "STK", "instrument type (STK, BOND, ...)")
	c.Flags().StringVar(&scanType, "type", "TOP_PERC_GAIN", "scan code, e.g. TOP_PERC_GAIN, MOST_ACTIVE")
	c.Flags().StringVar(&location, "location", "STK.US.MAJOR", "scan location")
	c.Flags().StringVar(&filter, "filter", "", "filters as k=v,k=v (e.g. priceAbove=5,marketCapAbove=1000000000)")
	return c
}

// parseScanFilter turns "k=v,k=v" into the scanner filter array.
func parseScanFilter(s string) []map[string]any {
	var out []map[string]any
	for _, kv := range strings.Split(s, ",") {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		var v any = parts[1]
		if n, err := strconv.ParseFloat(parts[1], 64); err == nil {
			v = n
		}
		out = append(out, map[string]any{"code": parts[0], "value": v})
	}
	return out
}
