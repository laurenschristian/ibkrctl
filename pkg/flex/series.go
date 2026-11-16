package flex

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ErrNoDate is returned by ParseTime for empty and MULTI values.
var ErrNoDate = errors.New("flex: no date")

// ParseTime parses Flex dates and date-times: yyyyMMdd, yyyy-MM-dd, optionally followed by
// ';', ' ', ',' or 'T' and HHmmss or HH:mm:ss. A trailing zone suffix is ignored; the result is UTC.
func ParseTime(s string) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "MULTI" {
		return time.Time{}, ErrNoDate
	}
	d, tm := s, ""
	if i := strings.IndexAny(s, "; ,T"); i >= 0 {
		d, tm = s[:i], strings.TrimSpace(s[i+1:])
	}
	dl := "20060102"
	if strings.Contains(d, "-") {
		dl = "2006-01-02"
	}
	if tm == "" {
		t, err := time.Parse(dl, d)
		if err != nil {
			return time.Time{}, fmt.Errorf("flex date %q: %w", s, err)
		}
		return t, nil
	}
	tl := "150405"
	if strings.Contains(tm, ":") {
		tl = "15:04:05"
	}
	if len(tm) > len(tl) {
		tm = tm[:len(tl)]
	}
	t, err := time.Parse(dl+" "+tl, d+" "+tm)
	if err != nil {
		return time.Time{}, fmt.Errorf("flex date %q: %w", s, err)
	}
	return t, nil
}

// NAVPoint is one day of base-currency NAV.
type NAVPoint struct {
	Date  time.Time
	Cash  float64
	Stock float64
	Total float64
}

// NAVSeries returns EquitySummary rows as points sorted by date. Rows without a valid
// reportDate are skipped; a repeated date keeps the last row.
func (s Statement) NAVSeries() []NAVPoint {
	byDate := map[time.Time]NAVPoint{}
	for _, e := range s.EquitySummary {
		d, err := ParseTime(e.ReportDate)
		if err != nil {
			continue
		}
		byDate[d] = NAVPoint{Date: d, Cash: float64(e.Cash), Stock: float64(e.Stock), Total: float64(e.Total)}
	}
	out := make([]NAVPoint, 0, len(byDate))
	for _, p := range byDate {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Date.Before(out[j].Date) })
	return out
}

// Dividends returns dividend and payment-in-lieu cash transactions, in statement order.
func (s Statement) Dividends() []CashTransaction {
	var out []CashTransaction
	for _, c := range s.CashTxns {
		if c.Type == CashDividends || c.Type == CashPaymentInLieu {
			out = append(out, c)
		}
	}
	return out
}

// SettledCash returns endingSettledCash for currency base, or for the BASE_SUMMARY row
// when base is "" or BASE_SUMMARY. It is 0 when the CashReport has no such row.
func (s Statement) SettledCash(base string) float64 {
	if base == "" {
		base = "BASE_SUMMARY"
	}
	for _, c := range s.Cash {
		if c.Currency == base {
			return float64(c.EndingSettledCash)
		}
	}
	return 0
}

// LongTerm reports whether a closed lot was held more than one year, from its holding-period
// (else open) date to its close date (dateTime, else tradeDate).
func (t Trade) LongTerm() (bool, error) {
	openS := t.HoldingPeriod
	if openS == "" {
		openS = t.OpenDateTime
	}
	closeS := t.DateTime
	if closeS == "" {
		closeS = t.TradeDate
	}
	open, err := ParseTime(openS)
	if err != nil {
		return false, err
	}
	closed, err := ParseTime(closeS)
	if err != nil {
		return false, err
	}
	day := func(t time.Time) time.Time { return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC) }
	return day(closed).After(day(open).AddDate(1, 0, 0)), nil
}
