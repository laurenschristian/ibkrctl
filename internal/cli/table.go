package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"text/tabwriter"
)

// show prints v as a human table by default, or raw JSON with --json. If the
// renderer cannot make a table (unexpected shape), it falls back to JSON.
func show(v any, render func(any) (string, bool)) error {
	if flagJSON {
		return emit(v)
	}
	g := redact(v)
	if s, ok := render(g); ok {
		fmt.Print(s)
		return nil
	}
	return emit(v)
}

// num coerces an IBKR value (float64 or numeric string) to a float.
func num(v any) (float64, bool) {
	switch t := v.(type) {
	case float64:
		return t, true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(t), 64)
		return f, err == nil
	}
	return 0, false
}

// str returns a display string for a generic value.
func str(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case float64:
		return trimFloat(t)
	default:
		return fmt.Sprint(t)
	}
}

func trimFloat(f float64) string {
	return strconv.FormatFloat(f, 'f', -1, 64)
}

// money formats a float with thousands separators and two decimals.
func money(v any) string {
	f, ok := num(v)
	if !ok {
		return str(v)
	}
	neg := f < 0
	if neg {
		f = -f
	}
	s := strconv.FormatFloat(f, 'f', 2, 64)
	dot := strings.IndexByte(s, '.')
	intPart, frac := s[:dot], s[dot:]
	var b strings.Builder
	for i, d := range intPart {
		if i > 0 && (len(intPart)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(d)
	}
	out := b.String() + frac
	if neg {
		return "-" + out
	}
	return out
}

func asMap(v any) (map[string]any, bool) { m, ok := v.(map[string]any); return m, ok }
func asList(v any) ([]any, bool)         { l, ok := v.([]any); return l, ok }

// firstNonEmpty returns the first present, non-nil key from a map.
func pick(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if val, ok := m[k]; ok && val != nil {
			return val
		}
	}
	return nil
}

func newTab() (*strings.Builder, *tabwriter.Writer) {
	var b strings.Builder
	return &b, tabwriter.NewWriter(&b, 0, 2, 2, ' ', 0)
}

// ---- positions ----

func renderPositions(v any) (string, bool) {
	rows, ok := asList(v)
	if !ok {
		return "", false
	}
	if len(rows) == 0 {
		return "no positions\n", true
	}
	type pos struct {
		sym    string
		qty    float64
		price  any
		value  float64
		avg    any
		unreal any
	}
	var ps []pos
	var total float64
	for _, r := range rows {
		m, ok := asMap(r)
		if !ok {
			return "", false
		}
		val, _ := num(pick(m, "mktValue"))
		qty, _ := num(pick(m, "position"))
		ps = append(ps, pos{
			sym:    str(pick(m, "contractDesc", "ticker", "conid")),
			qty:    qty,
			price:  pick(m, "mktPrice"),
			value:  val,
			avg:    pick(m, "avgCost", "avgPrice"),
			unreal: pick(m, "unrealizedPnl"),
		})
		total += val
	}
	sort.SliceStable(ps, func(i, j int) bool { return ps[i].value > ps[j].value })
	b, w := newTab()
	fmt.Fprintln(w, "SYMBOL\tQTY\tPRICE\tVALUE\tAVG COST\tUNREAL P&L")
	for _, p := range ps {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			p.sym, trimFloat(p.qty), money(p.price), money(p.value), money(p.avg), money(p.unreal))
	}
	fmt.Fprintf(w, "\tTOTAL\t\t%s\t\t\n", money(total))
	_ = w.Flush()
	return b.String(), true
}

// ---- summary ----

var summaryOrder = []struct{ key, label string }{
	{"netliquidation", "Net liquidation"},
	{"equitywithloanvalue", "Equity with loan"},
	{"totalcashvalue", "Total cash"},
	{"availablefunds", "Available funds"},
	{"buyingpower", "Buying power"},
	{"grosspositionvalue", "Gross positions"},
	{"excessliquidity", "Excess liquidity"},
	{"initmarginreq", "Initial margin"},
	{"maintmarginreq", "Maint margin"},
}

func renderSummary(v any) (string, bool) {
	m, ok := asMap(v)
	if !ok {
		return "", false
	}
	amt := func(key string) (any, bool) {
		cell, ok := asMap(m[key])
		if !ok {
			return nil, false
		}
		return pick(cell, "amount", "value"), true
	}
	b, w := newTab()
	found := false
	for _, row := range summaryOrder {
		if val, ok := amt(row.key); ok {
			fmt.Fprintf(w, "%s\t%s\n", row.label, money(val))
			found = true
		}
	}
	_ = w.Flush()
	if !found {
		return "", false
	}
	return b.String(), true
}

// ---- ledger ----

func renderLedger(v any) (string, bool) {
	m, ok := asMap(v)
	if !ok {
		return "", false
	}
	b, w := newTab()
	fmt.Fprintln(w, "CCY\tCASH\tSETTLED\tNET LIQ")
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	found := false
	for _, k := range keys {
		cell, ok := asMap(m[k])
		if !ok {
			continue
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", k,
			money(pick(cell, "cashbalance")),
			money(pick(cell, "settledcash")),
			money(pick(cell, "netliquidationvalue")))
		found = true
	}
	_ = w.Flush()
	if !found {
		return "", false
	}
	return b.String(), true
}

// ---- allocation ----

func renderAllocation(v any) (string, bool) {
	m, ok := asMap(v)
	if !ok {
		return "", false
	}
	var b strings.Builder
	found := false
	for _, dim := range []string{"assetClass", "sector", "group"} {
		block, ok := asMap(m[dim])
		if !ok {
			continue
		}
		long, _ := asMap(block["long"])
		if len(long) == 0 {
			continue
		}
		fmt.Fprintf(&b, "%s\n", dim)
		type kv struct {
			k string
			v float64
		}
		var items []kv
		for k, val := range long {
			f, _ := num(val)
			items = append(items, kv{k, f})
		}
		sort.SliceStable(items, func(i, j int) bool { return items[i].v > items[j].v })
		var tb strings.Builder
		w := tabwriterTo(&tb)
		for _, it := range items {
			fmt.Fprintf(w, "  %s\t%s\n", it.k, money(it.v))
		}
		_ = w.Flush()
		b.WriteString(tb.String())
		found = true
	}
	if !found {
		return "", false
	}
	return b.String(), true
}

func tabwriterTo(b *strings.Builder) *tabwriter.Writer {
	return tabwriter.NewWriter(b, 0, 2, 2, ' ', 0)
}

// ---- whatif (order preview) ----

func renderWhatIf(v any) (string, bool) {
	m, ok := asMap(v)
	if !ok {
		return "", false
	}
	b, w := newTab()
	if amt, ok := asMap(m["amount"]); ok {
		fmt.Fprintf(w, "Order value\t%s\n", str(pick(amt, "amount")))
		fmt.Fprintf(w, "Commission\t%s\n", str(pick(amt, "commission")))
		fmt.Fprintf(w, "Total\t%s\n", str(pick(amt, "total")))
	}
	dims := []struct{ key, label string }{
		{"equity", "Equity"},
		{"initial", "Initial margin"},
		{"maintenance", "Maint margin"},
		{"position", "Position"},
	}
	for _, d := range dims {
		blk, ok := asMap(m[d.key])
		if !ok {
			continue
		}
		fmt.Fprintf(w, "%s\t%s -> %s (%s)\n", d.label,
			money(pick(blk, "current")), money(pick(blk, "after")), str(pick(blk, "change")))
	}
	_ = w.Flush()
	out := b.String()
	if w2 := strings.TrimSpace(str(pick(m, "warn"))); w2 != "" {
		out += "warning: " + w2 + "\n"
	}
	if e := strings.TrimSpace(str(pick(m, "error"))); e != "" {
		out += "error: " + e + "\n"
	}
	if strings.TrimSpace(out) == "" {
		return "", false
	}
	return out, true
}

// ---- trades ----

func renderTrades(v any) (string, bool) {
	rows, ok := asList(v)
	if !ok {
		return "", false
	}
	if len(rows) == 0 {
		return "no trades\n", true
	}
	b, w := newTab()
	fmt.Fprintln(w, "TIME\tSYMBOL\tSIDE\tQTY\tPRICE\tAMOUNT")
	for _, r := range rows {
		m, ok := asMap(r)
		if !ok {
			return "", false
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			str(pick(m, "trade_time", "trade_time_r", "execution_id")),
			str(pick(m, "symbol", "contract_description_1")),
			str(pick(m, "side")),
			str(pick(m, "size")),
			str(pick(m, "price")),
			str(pick(m, "net_amount", "order_ref")))
	}
	_ = w.Flush()
	return b.String(), true
}

// ---- orders ----

func renderOrders(v any) (string, bool) {
	m, ok := asMap(v)
	if !ok {
		return "", false
	}
	rows, ok := asList(m["orders"])
	if !ok {
		return "", false
	}
	if len(rows) == 0 {
		return "no orders\n", true
	}
	b, w := newTab()
	fmt.Fprintln(w, "ID\tSYMBOL\tSIDE\tTYPE\tQTY\tPRICE\tSTATUS")
	for _, r := range rows {
		om, ok := asMap(r)
		if !ok {
			return "", false
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			str(pick(om, "orderId", "order_id")),
			str(pick(om, "ticker", "symbol")),
			str(pick(om, "side")),
			str(pick(om, "orderType", "order_type")),
			str(pick(om, "totalSize", "remainingQuantity", "sizeAndFills")),
			str(pick(om, "price")),
			str(pick(om, "status", "order_status")))
	}
	_ = w.Flush()
	return b.String(), true
}

// ---- performance (NAV curve) ----

var sparkRunes = []rune("▁▂▃▄▅▆▇█")

func sparkline(vals []float64) string {
	if len(vals) == 0 {
		return ""
	}
	lo, hi := vals[0], vals[0]
	for _, v := range vals {
		if v < lo {
			lo = v
		}
		if v > hi {
			hi = v
		}
	}
	span := hi - lo
	var b strings.Builder
	for _, v := range vals {
		idx := 0
		if span > 0 {
			idx = int((v - lo) / span * float64(len(sparkRunes)-1))
		}
		b.WriteRune(sparkRunes[idx])
	}
	return b.String()
}

func floatsFrom(v any) []float64 {
	list, ok := asList(v)
	if !ok {
		return nil
	}
	out := make([]float64, 0, len(list))
	for _, x := range list {
		if f, ok := num(x); ok {
			out = append(out, f)
		}
	}
	return out
}

func renderPerformance(v any) (string, bool) {
	m, ok := asMap(v)
	if !ok {
		return "", false
	}
	nav, ok := asMap(m["nav"])
	if !ok {
		return "", false
	}
	data, ok := asList(nav["data"])
	if !ok || len(data) == 0 {
		return "", false
	}
	var b strings.Builder
	for _, d := range data {
		dm, ok := asMap(d)
		if !ok {
			continue
		}
		vals := floatsFrom(dm["navValues"])
		if len(vals) == 0 {
			continue
		}
		first, last := vals[0], vals[len(vals)-1]
		chg := 0.0
		if first != 0 {
			chg = (last - first) / first * 100
		}
		fmt.Fprintf(&b, "%s  %s -> %s  %+.2f%%  %s\n",
			str(pick(dm, "id", "idType")),
			trimFloat(round2(first)), trimFloat(round2(last)), chg, sparkline(vals))
	}
	if b.Len() == 0 {
		return "", false
	}
	return b.String(), true
}
