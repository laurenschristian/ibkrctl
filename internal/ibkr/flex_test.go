package ibkr

import (
	"context"
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

	fc := NewFlex("tok")
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
	if _, err := NewFlex("").Statement(context.Background(), "Q1", time.Millisecond, 1); err == nil {
		t.Fatal("expected error with no token")
	}
}

func TestFlexRequestError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`<FlexStatementResponse><Status>Fail</Status><ErrorCode>1015</ErrorCode><ErrorMessage>Token invalid</ErrorMessage></FlexStatementResponse>`))
	}))
	defer srv.Close()
	fc := NewFlex("bad")
	fc.Base = srv.URL + "/FlexStatementService"
	if _, err := fc.Statement(context.Background(), "Q1", time.Millisecond, 2); err == nil {
		t.Fatal("expected request failure")
	}
}
