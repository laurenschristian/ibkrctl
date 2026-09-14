package ibkr

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthStatus404MapsToNotAuthed(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "Access Denied", http.StatusNotFound)
	}))
	defer s.Close()
	st, err := New(s.URL).AuthStatus(context.Background())
	if err != nil || st.Authenticated {
		t.Fatalf("want not-authed, got %+v %v", st, err)
	}
}

func TestAuthStatusEmptyBodyMapsToNotAuthed(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK) // empty body
	}))
	defer s.Close()
	st, err := New(s.URL).AuthStatus(context.Background())
	if err != nil || st.Authenticated {
		t.Fatalf("want not-authed, got %+v %v", st, err)
	}
}

func TestServerError(t *testing.T) {
	s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer s.Close()
	if _, err := New(s.URL).PnL(context.Background()); err == nil {
		t.Fatal("want 500 error")
	}
}
