package flex

import (
	"errors"
	"os"
	"testing"
	"time"
)

func loadFull(t *testing.T) Statement {
	t.Helper()
	return loadStatement(t, "testdata/activity_full.xml")
}

func loadStatement(t *testing.T, path string) Statement {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Statements) != 1 {
		t.Fatalf("statements: %d", len(r.Statements))
	}
	return r.Statements[0]
}

func TestParseTime(t *testing.T) {
	want := time.Date(2026, 11, 13, 9, 35, 7, 0, time.UTC)
	for _, s := range []string{"20261113;093507", "2026-11-13;09:35:07", "2026-11-13T09:35:07", "2026-11-13 09:35:07", "20261113,093507", "20261113;093507 EST", "2026-11-13T09:35:07-05:00"} {
		got, err := ParseTime(s)
		if err != nil || !got.Equal(want) {
			t.Errorf("%q: got %v, %v", s, got, err)
		}
	}
	for _, s := range []string{"20261113", "2026-11-13"} {
		got, err := ParseTime(s)
		if err != nil || !got.Equal(time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC)) {
			t.Errorf("%q: got %v, %v", s, got, err)
		}
	}
	for _, s := range []string{"", "MULTI"} {
		if _, err := ParseTime(s); !errors.Is(err, ErrNoDate) {
			t.Errorf("%q: want ErrNoDate, got %v", s, err)
		}
	}
	for _, s := range []string{"2026111", "20261113;99", "13/11/2026"} {
		if _, err := ParseTime(s); err == nil {
			t.Errorf("%q: want error", s)
		}
	}
}

func TestChangeInNAV(t *testing.T) {
	n := loadFull(t).ChangeInNAV
	if n == nil {
		t.Fatal("ChangeInNAV missing")
	}
	if n.StartingValue != 10000 || n.EndingValue != 10760.45 || n.MTM != 250.5 || n.DepositsWithdrawals != 500 ||
		n.Dividends != 12 || n.Interest != 3.25 || n.Commissions != -2.5 || n.TWR != 2.5021 || n.WithholdingTax != -1.8 {
		t.Fatalf("bad ChangeInNAV: %+v", n)
	}
	if loadStatement(t, "testdata/activity.xml").ChangeInNAV != nil {
		t.Fatal("ChangeInNAV should be nil when the section is absent")
	}
}

func TestNAVSeries(t *testing.T) {
	s := loadFull(t)
	if len(s.EquitySummary) != 4 {
		t.Fatalf("equity rows: %d", len(s.EquitySummary))
	}
	pts := s.NAVSeries()
	if len(pts) != 3 {
		t.Fatalf("points: %+v", pts)
	}
	wantTotals := []float64{10000, 10100, 10760.45}
	for i, p := range pts {
		if p.Total != wantTotals[i] {
			t.Fatalf("point %d: %+v", i, p)
		}
	}
	if !pts[0].Date.Equal(time.Date(2026, 11, 10, 0, 0, 0, 0, time.UTC)) || pts[2].Cash != 1500.25 || pts[2].Stock != 9260.2 {
		t.Fatalf("bad points: %+v", pts)
	}
}

func TestCashTransactions(t *testing.T) {
	s := loadFull(t)
	if len(s.CashTxns) != 6 {
		t.Fatalf("cash txns: %d", len(s.CashTxns))
	}
	c := s.CashTxns[0]
	if c.Type != CashDividends || c.Symbol != "AAPL" || c.Amount != 12 || c.Currency != "USD" || c.DateTime != "20261112;202000" ||
		c.SettleDate != "20261112" || c.TransactionID != "9001" || c.Description == "" {
		t.Fatalf("bad dividend row: %+v", c)
	}
	types := map[string]bool{}
	for _, c := range s.CashTxns {
		types[c.Type] = true
	}
	for _, ty := range []string{CashWithholdingTax, CashPaymentInLieu, CashInterestReceived, CashDepositsWithdrawal, CashOtherFees} {
		if !types[ty] {
			t.Errorf("type %q not parsed", ty)
		}
	}
	divs := s.Dividends()
	if len(divs) != 2 || divs[0].TransactionID != "9001" || divs[1].Type != CashPaymentInLieu {
		t.Fatalf("dividends: %+v", divs)
	}
}

func TestSettledCash(t *testing.T) {
	s := loadFull(t)
	if got := s.SettledCash(""); got != 1400 {
		t.Fatalf("base summary: %v", got)
	}
	if got := s.SettledCash("BASE_SUMMARY"); got != 1400 {
		t.Fatalf("BASE_SUMMARY: %v", got)
	}
	if got := s.SettledCash("USD"); got != 1400 {
		t.Fatalf("USD: %v", got)
	}
	if got := s.SettledCash("EUR"); got != 0 {
		t.Fatalf("EUR: %v", got)
	}
}

func TestOpenPositionLots(t *testing.T) {
	s := loadFull(t)
	var lots []Position
	for _, p := range s.Positions {
		if p.LevelOfDetail == "LOT" {
			lots = append(lots, p)
		}
	}
	if len(lots) != 2 {
		t.Fatalf("lots: %d", len(lots))
	}
	l := lots[0]
	if l.Quantity != 6 || l.CostBasis != 1080 || l.OpenDateTime != "20240105;093500" || l.HoldingPeriodDateTime != "20240105;093500" ||
		l.OriginatingOrderID != "401" || l.OriginatingTxnID != "501" {
		t.Fatalf("bad lot: %+v", l)
	}
}

func TestTradesClosedLots(t *testing.T) {
	s := loadFull(t)
	if len(s.Trades) != 1 || s.Trades[0].RealizedPnL != 120 || s.Trades[0].MTMPnL != 15.5 {
		t.Fatalf("trades: %+v", s.Trades)
	}
	if len(s.ClosedLots) != 2 {
		t.Fatalf("closed lots: %d", len(s.ClosedLots))
	}
	long, short := s.ClosedLots[0], s.ClosedLots[1]
	if long.LevelOfDetail != "CLOSED_LOT" || long.OpenDateTime != "20240601;100000" || long.RealizedPnL != 180 || long.CostBasis != 1080 {
		t.Fatalf("bad lot: %+v", long)
	}
	if lt, err := long.LongTerm(); err != nil || !lt {
		t.Fatalf("lot 0 long term: %v %v", lt, err)
	}
	if lt, err := short.LongTerm(); err != nil || lt {
		t.Fatalf("lot 1 short term: %v %v", lt, err)
	}
}

func TestLongTermBoundary(t *testing.T) {
	cases := []struct {
		open, close string
		want        bool
	}{
		{"20250115", "20260115", false},
		{"20250115", "20260116", true},
		{"20250115;160000", "20260115;093000", false},
	}
	for _, c := range cases {
		got, err := Trade{OpenDateTime: c.open, TradeDate: c.close}.LongTerm()
		if err != nil || got != c.want {
			t.Errorf("%s -> %s: got %v %v", c.open, c.close, got, err)
		}
	}
	if _, err := (Trade{TradeDate: "20260101"}).LongTerm(); !errors.Is(err, ErrNoDate) {
		t.Fatalf("missing open date: %v", err)
	}
	if _, err := (Trade{OpenDateTime: "20260101"}).LongTerm(); !errors.Is(err, ErrNoDate) {
		t.Fatalf("missing close date: %v", err)
	}
}
