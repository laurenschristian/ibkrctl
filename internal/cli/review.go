package cli

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func reviewCmd() *cobra.Command {
	var account string
	c := &cobra.Command{
		Use:   "review",
		Short: "One-shot portfolio snapshot: summary, positions, allocation, P&L, open orders",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			acct, err := resolveAccount(ctx, account)
			if err != nil {
				return err
			}
			data := portfolioReview(ctx, acct)
			return show(data, renderReview)
		},
	}
	c.Flags().StringVar(&account, "account", "", "account alias or id")
	return c
}

// portfolioReview gathers the read endpoints an investing decision needs. A
// failing section is recorded as an error string rather than aborting the whole
// snapshot, so a single dead endpoint never blanks the report.
func portfolioReview(ctx context.Context, acct string) map[string]any {
	out := map[string]any{"account": cfg.AliasFor(acct)}
	sect := func(key string, v any, err error) {
		if err != nil {
			out[key] = map[string]any{"error": err.Error()}
			return
		}
		out[key] = v
	}
	sm, err := client.Summary(ctx, acct)
	sect("summary", sm, err)
	ps, err := client.Positions(ctx, acct, 0)
	sect("positions", ps, err)
	al, err := client.Allocation(ctx, acct)
	sect("allocation", al, err)
	pl, err := client.PnL(ctx)
	sect("pnl", pl, err)
	or, err := client.Orders(ctx)
	sect("orders", or, err)
	return out
}

func renderReview(v any) (string, bool) {
	m, ok := asMap(v)
	if !ok {
		return "", false
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Account %s\n\n", str(m["account"]))

	if s, ok := renderSummary(m["summary"]); ok {
		b.WriteString("Summary\n")
		b.WriteString(indent(s))
		b.WriteString("\n")
	}
	if s, ok := renderPositions(m["positions"]); ok {
		b.WriteString("Positions\n")
		b.WriteString(indent(s))
		b.WriteString("\n")
	}
	if s, ok := renderAllocation(m["allocation"]); ok {
		b.WriteString("Allocation\n")
		b.WriteString(indent(s))
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "Session P&L  %s\n", reviewDailyPnL(m["pnl"]))
	fmt.Fprintf(&b, "Open orders  %d\n", reviewOrderCount(m["orders"]))
	return b.String(), true
}

func indent(s string) string {
	var b strings.Builder
	for _, line := range strings.Split(strings.TrimRight(s, "\n"), "\n") {
		b.WriteString("  ")
		b.WriteString(line)
		b.WriteByte('\n')
	}
	return b.String()
}

// reviewDailyPnL sums the daily P&L (dpl) across the partitioned upnl blocks.
func reviewDailyPnL(v any) string {
	m, ok := asMap(v)
	if !ok {
		return "n/a"
	}
	upnl, ok := asMap(m["upnl"])
	if !ok {
		return "n/a"
	}
	var total float64
	found := false
	for _, blk := range upnl {
		if bm, ok := asMap(blk); ok {
			if f, ok := num(bm["dpl"]); ok {
				total += f
				found = true
			}
		}
	}
	if !found {
		return "n/a"
	}
	return money(total)
}

func reviewOrderCount(v any) int {
	if m, ok := asMap(v); ok {
		if l, ok := asList(m["orders"]); ok {
			return len(l)
		}
	}
	if l, ok := asList(v); ok {
		return len(l)
	}
	return 0
}
