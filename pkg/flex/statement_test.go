package flex

import (
	"errors"
	"os"
	"testing"
)

func TestParseActivity(t *testing.T) {
	b, err := os.ReadFile("testdata/activity.xml")
	if err != nil {
		t.Fatal(err)
	}
	r, err := Parse(b)
	if err != nil {
		t.Fatal(err)
	}
	if r.QueryName != "daily" || len(r.Statements) != 1 {
		t.Fatalf("bad response: %+v", r)
	}
	s := r.Statements[0]
	if s.AccountID != "U0000000" || s.ToDate != "20261113" {
		t.Fatalf("bad statement header: %+v", s)
	}
	if len(s.Positions) != 1 {
		t.Fatalf("positions: %d", len(s.Positions))
	}
	p := s.Positions[0]
	if p.Symbol != "AAPL" || p.Conid != "265598" || p.Quantity != 10 || p.CostBasis != 2000 || p.MarketValue != 2305 || p.Currency != "USD" {
		t.Fatalf("bad position: %+v", p)
	}
	if len(s.Trades) != 1 {
		t.Fatalf("trades: %d (Order rows must be skipped)", len(s.Trades))
	}
	tr := s.Trades[0]
	if tr.ExecID != "0001.01" || tr.BuySell != "BUY" || tr.Quantity != 10 || tr.Price != 200 || tr.Commission != -1.25 {
		t.Fatalf("bad trade: %+v", tr)
	}
	if len(s.Cash) != 2 || s.Cash[0].Currency != "BASE_SUMMARY" || s.Cash[1].EndingCash != 1500.25 || s.Cash[1].EndingSettledCash != 0 {
		t.Fatalf("bad cash: %+v", s.Cash)
	}
}

func TestParseErrorDoc(t *testing.T) {
	_, err := Parse([]byte(`<FlexStatementResponse><Status>Fail</Status><ErrorCode>1020</ErrorCode><ErrorMessage>Invalid request</ErrorMessage></FlexStatementResponse>`))
	var fe *Error
	if !errors.As(err, &fe) || fe.Code != "1020" {
		t.Fatalf("want *Error 1020, got %v", err)
	}
}

func TestParseBadNumber(t *testing.T) {
	if _, err := Parse([]byte(`<FlexQueryResponse><FlexStatements><FlexStatement><OpenPositions><OpenPosition position="x"/></OpenPositions></FlexStatement></FlexStatements></FlexQueryResponse>`)); err == nil {
		t.Fatal("expected number parse error")
	}
}
