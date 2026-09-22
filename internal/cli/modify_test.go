package cli

import (
	"strings"
	"testing"

	"github.com/laurenschristian/ibkrctl/internal/ibkr/ibkrtest"
)

// liveOrder is the shape the gateway actually returns for a resting order:
// orderType is the display form ("Limit"), price is a string, and the account
// fields carry the real id.
func liveOrder() map[string]any {
	return map[string]any{
		"orderId":           "2065831917",
		"conid":             107113386,
		"conidex":           "107113386",
		"ticker":            "META",
		"side":              "BUY",
		"orderType":         "Limit",
		"price":             "748.00",
		"timeInForce":       "GTC",
		"totalSize":         4,
		"remainingQuantity": 4,
		"status":            "Submitted",
		"acct":              "U1234567",
	}
}

func seedOrders(s *ibkrtest.Server) {
	s.Orders = []map[string]any{liveOrder()}
}

// A modify must carry the conid: IBKR rejects the request without it, which is
// what made `modify --price` unusable.
func TestModifySendsConidAndInheritsFields(t *testing.T) {
	s := withGateway(t)
	seedOrders(s)

	if _, err := run(t, "modify", "2065831917", "--price", "755", "--confirm"); err != nil {
		t.Fatalf("modify: %v", err)
	}
	got := s.LastModify
	if got == nil {
		t.Fatal("no modify body captured")
	}
	if got["conid"] == nil {
		t.Fatalf("conid missing from modify body: %#v", got)
	}
	if got["price"] != 755.0 {
		t.Fatalf("price = %#v, want 755", got["price"])
	}
	// Untouched fields come off the live order, and the display order type is
	// mapped back to the API code.
	if got["side"] != "BUY" {
		t.Fatalf("side = %#v, want BUY", got["side"])
	}
	if got["orderType"] != "LMT" {
		t.Fatalf("orderType = %#v, want LMT (mapped from \"Limit\")", got["orderType"])
	}
	if got["tif"] != "GTC" {
		t.Fatalf("tif = %#v, want GTC", got["tif"])
	}
	if got["quantity"] != 4.0 {
		t.Fatalf("quantity = %#v, want 4", got["quantity"])
	}
}

func TestModifyFlagsOverrideLiveOrder(t *testing.T) {
	s := withGateway(t)
	seedOrders(s)

	if _, err := run(t, "modify", "2065831917", "--qty", "6", "--tif", "day", "--confirm"); err != nil {
		t.Fatalf("modify: %v", err)
	}
	if s.LastModify["quantity"] != 6.0 {
		t.Fatalf("quantity = %#v, want 6", s.LastModify["quantity"])
	}
	if s.LastModify["tif"] != "DAY" {
		t.Fatalf("tif = %#v, want DAY", s.LastModify["tif"])
	}
	// The price the caller did not touch is preserved, not dropped.
	if s.LastModify["price"] != 748.0 {
		t.Fatalf("price = %#v, want the live 748", s.LastModify["price"])
	}
}

func TestModifyUnknownOrderIsARealError(t *testing.T) {
	s := withGateway(t)
	seedOrders(s)

	out, err := run(t, "modify", "999999", "--price", "10", "--confirm")
	if err == nil {
		t.Fatalf("want an error for an order not in the book, got:\n%s", out)
	}
	if !strings.Contains(err.Error(), "not in the live book") {
		t.Fatalf("unhelpful error: %v", err)
	}
	if s.LastModify != nil {
		t.Fatal("nothing should have been submitted")
	}
}

func TestModifyDryRunShowsTheBodyAndSendsNothing(t *testing.T) {
	s := withGateway(t)
	seedOrders(s)

	out, err := run(t, "modify", "2065831917", "--price", "755")
	if err != nil {
		t.Fatalf("dry run: %v", err)
	}
	if !strings.Contains(out, "DRY RUN") || !strings.Contains(out, "conid") {
		t.Fatalf("dry run should show the resolved body:\n%s", out)
	}
	if s.LastModify != nil {
		t.Fatal("dry run must not submit")
	}
}

// The gateway answers a cold call with an empty book and snapshot:false. An
// unretried call reports "no orders" while orders are actually live.
func TestOrdersRetriesTheColdCache(t *testing.T) {
	s := withGateway(t)
	seedOrders(s)
	s.ColdOrderCalls = 1

	out, err := run(t, "orders")
	if err != nil {
		t.Fatalf("orders: %v", err)
	}
	if strings.Contains(out, "no orders") {
		t.Fatalf("cold first call was not retried:\n%s", out)
	}
	if !strings.Contains(out, "META") {
		t.Fatalf("live order missing:\n%s", out)
	}
}

func TestOrdersEmptyBookStillReportsEmpty(t *testing.T) {
	withGateway(t)
	out, err := run(t, "orders")
	if err != nil || !strings.Contains(out, "no orders") {
		t.Fatalf("empty book: %v\n%s", err, out)
	}
}

func TestApiOrderTypeMapsDisplayForms(t *testing.T) {
	cases := map[string]string{
		"Limit": "LMT", "Market": "MKT", "Stop": "STP",
		"Stop Limit": "STOP_LIMIT", "Trailing Stop": "TRAIL",
		"LMT": "LMT", "MIDPRICE": "MIDPRICE",
	}
	for in, want := range cases {
		if got := apiOrderType(in); got != want {
			t.Errorf("apiOrderType(%q) = %q, want %q", in, got, want)
		}
	}
}

// The unfiltered book keeps filled and cancelled orders. Matching one sent
// quantity 0 to IBKR, which answered "Order size 0 is not valid" instead of
// anything the caller could act on.
func TestModifyRefusesTerminalOrders(t *testing.T) {
	for _, status := range []string{"Filled", "Cancelled", "Inactive", "Rejected"} {
		s := withGateway(t)
		o := liveOrder()
		o["status"] = status
		o["remainingQuantity"] = 0
		s.Orders = []map[string]any{o}

		_, err := run(t, "modify", "2065831917", "--price", "755", "--confirm")
		if err == nil {
			t.Fatalf("%s order should not be modifiable", status)
		}
		if !strings.Contains(err.Error(), "cannot be modified") {
			t.Fatalf("%s: unhelpful error: %v", status, err)
		}
		if s.LastModify != nil {
			t.Fatalf("%s: nothing should have been submitted", status)
		}
	}
}

// place and modify both need --confirm; typing it on cancel used to be a hard
// "unknown flag" error mid-incident.
func TestCancelAcceptsConfirmFlag(t *testing.T) {
	s := withGateway(t)
	if _, err := run(t, "cancel", "888", "--confirm"); err != nil {
		t.Fatalf("cancel --confirm: %v", err)
	}
	if s.Canceled != "888" {
		t.Fatalf("Canceled = %q, want 888", s.Canceled)
	}
}
