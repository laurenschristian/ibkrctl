package ibkr

import (
	"context"
	"testing"
)

func TestBrowserLoginRequiresCreds(t *testing.T) {
	if err := BrowserLogin(context.Background(), LoginParams{}); err == nil {
		t.Fatal("want error for missing creds")
	}
}
