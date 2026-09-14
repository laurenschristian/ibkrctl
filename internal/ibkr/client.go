// Package ibkr is a thin client for the IBKR Client Portal Gateway.
package ibkr

import (
	"net/http"
	"strings"
	"time"
)

// Client talks to the ibkrctl target. Endpoints land here as commands are built.
type Client struct {
	BaseURL string
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{BaseURL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 30 * time.Second}}
}
