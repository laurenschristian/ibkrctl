package flex

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFlexStatementFlow(t *testing.T) {
	var srv *httptest.Server
	warned := false
	mux := http.NewServeMux()
	mux.HandleFunc("/FlexStatementService.SendRequest", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Success</Status><ReferenceCode>REF123</ReferenceCode><Url>` + srv.URL + `/FlexStatementService.GetStatement</Url></FlexStatementResponse>`))
	})
	mux.HandleFunc("/FlexStatementService.GetStatement", func(w http.ResponseWriter, _ *http.Request) {
		if !warned {
			warned = true // first poll: not ready
			_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Warn</Status><ErrorCode>1019</ErrorCode><ErrorMessage>Statement generation in progress</ErrorMessage></FlexStatementResponse>`))
			return
		}
		_, _ = w.Write([]byte(`<FlexQueryResponse><FlexStatements><FlexStatement accountId="U1"><Trade symbol="AAPL"/></FlexStatement></FlexStatements></FlexQueryResponse>`))
	})
	srv = httptest.NewServer(mux)
	defer srv.Close()

	fc := New("tok")
	fc.Base = srv.URL + "/FlexStatementService"
	out, err := fc.Statement(context.Background(), "Q1", 10*time.Millisecond, 5)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "FlexQueryResponse") || !strings.Contains(string(out), "AAPL") {
		t.Fatalf("unexpected statement: %s", out)
	}
}

func TestFlexNoToken(t *testing.T) {
	if _, err := New("").Statement(context.Background(), "Q1", time.Millisecond, 1); err == nil {
		t.Fatal("expected error with no token")
	}
}

func TestFlexRequestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Fail</Status><ErrorCode>1015</ErrorCode><ErrorMessage>Token invalid</ErrorMessage></FlexStatementResponse>`))
	}))
	defer srv.Close()
	fc := New("bad")
	fc.Base = srv.URL + "/FlexStatementService"
	if _, err := fc.Statement(context.Background(), "Q1", time.Millisecond, 2); err == nil {
		t.Fatal("expected request failure")
	}
}

func TestFlexGetStatementFail(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".SendRequest") {
			_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Success</Status><ReferenceCode>R</ReferenceCode></FlexStatementResponse>`))
			return
		}
		_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Fail</Status><ErrorCode>1015</ErrorCode><ErrorMessage>Token invalid</ErrorMessage></FlexStatementResponse>`))
	}))
	defer srv.Close()
	fc := New("tok")
	fc.Base = srv.URL + "/FlexStatementService"
	_, err := fc.Statement(context.Background(), "Q1", time.Millisecond, 3)
	var fe *Error
	if !errors.As(err, &fe) || fe.Code != "1015" {
		t.Fatalf("want *Error 1015, got %v", err)
	}
}

func TestFlexPollHonorsContext(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".SendRequest") {
			_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Success</Status><ReferenceCode>R</ReferenceCode></FlexStatementResponse>`))
			return
		}
		_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Warn</Status><ErrorCode>1019</ErrorCode></FlexStatementResponse>`))
	}))
	defer srv.Close()
	fc := New("tok")
	fc.Base = srv.URL + "/FlexStatementService"
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := fc.Statement(ctx, "Q1", time.Hour, 5)
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(start) > 5*time.Second {
		t.Fatalf("want deadline exceeded quickly, got %v after %s", err, time.Since(start))
	}
}

func TestFlexFetchParses(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.URL.Path, ".SendRequest") {
			_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Success</Status><ReferenceCode>R</ReferenceCode></FlexStatementResponse>`))
			return
		}
		_, _ = w.Write([]byte(`<FlexQueryResponse><FlexStatements><FlexStatement accountId="U1"><OpenPositions><OpenPosition symbol="MSFT" position="3"/></OpenPositions></FlexStatement></FlexStatements></FlexQueryResponse>`))
	}))
	defer srv.Close()
	fc := New("tok")
	fc.Base = srv.URL + "/FlexStatementService"
	r, err := fc.Fetch(context.Background(), "Q1", time.Millisecond, 2)
	if err != nil || len(r.Statements) != 1 || r.Statements[0].Positions[0].Quantity != 3 {
		t.Fatalf("fetch: %v %+v", err, r)
	}
}
