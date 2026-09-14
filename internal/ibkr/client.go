// Package ibkr is a thin client for the IBKR Client Portal Gateway REST API.
package ibkr

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ErrNotAuthenticated is returned when the gateway has no live brokerage session.
var ErrNotAuthenticated = errors.New("not authenticated: run `ibkrctl login`")

// Client talks to the local Client Portal Gateway. The gateway proxies to
// api.ibkr.com and holds the session, so ibkrctl never sees a password.
type Client struct {
	BaseURL string // e.g. https://localhost:5001
	HTTP    *http.Client
}

func New(baseURL string) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP: &http.Client{
			Timeout: 30 * time.Second,
			// The gateway serves a self-signed cert on localhost.
			Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}, //nolint:gosec
		},
	}
}

// APIError is a non-2xx gateway response.
type APIError struct {
	Status int
	Path   string
	Body   string
}

func (e *APIError) Error() string {
	b := strings.TrimSpace(e.Body)
	if len(b) > 200 {
		b = b[:200]
	}
	if b == "" {
		return fmt.Sprintf("%s: HTTP %d", e.Path, e.Status)
	}
	return fmt.Sprintf("%s: HTTP %d: %s", e.Path, e.Status, b)
}

func (c *Client) do(ctx context.Context, method, path string, body, out any) error {
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(b)
	}
	url := c.BaseURL + "/v1/api/" + strings.TrimLeft(path, "/")
	req, err := http.NewRequestWithContext(ctx, method, url, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "ibkrctl")
	// Akamai rejects POST without a length; NewRequest sets it for a body, and
	// http sends Content-Length: 0 for a bodyless POST via this header path.
	if rdr == nil && method != http.MethodGet {
		req.ContentLength = 0
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		if isConnRefused(err) {
			return fmt.Errorf("gateway not reachable at %s: run `ibkrctl gateway start` (%w)", c.BaseURL, err)
		}
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return &APIError{Status: resp.StatusCode, Path: path, Body: string(raw)}
	}
	if out == nil {
		return nil
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return ErrNotAuthenticated
	}
	return json.Unmarshal(raw, out)
}

func isConnRefused(err error) bool {
	s := err.Error()
	return strings.Contains(s, "connection refused") || strings.Contains(s, "connect: ")
}

// Get performs a GET and decodes JSON into out.
func (c *Client) Get(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodGet, path, nil, out)
}

// Post performs a POST (optional JSON body) and decodes JSON into out.
func (c *Client) Post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

// Delete performs a DELETE and decodes JSON into out.
func (c *Client) Delete(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodDelete, path, nil, out)
}

// Raw returns the decoded JSON of any /v1/api path as a generic value.
func (c *Client) Raw(ctx context.Context, method, path string, body any) (any, error) {
	var out any
	if err := c.do(ctx, strings.ToUpper(method), path, body, &out); err != nil {
		return nil, err
	}
	return out, nil
}
