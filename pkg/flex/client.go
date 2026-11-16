// Package flex is a client for the IBKR Flex Web Service. It depends only on the
// standard library, so it builds for GOOS=js GOARCH=wasm.
package flex

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// DefaultBase is the Flex Web Service endpoint (token-authenticated, no gateway).
const DefaultBase = "https://gdcdyn.interactivebrokers.com/Universal/servlet/FlexStatementService"

// ErrNotReady means the statement is still being generated; poll again later.
var ErrNotReady = errors.New("flex statement not ready")

// ErrNoToken is returned when the client has no Flex token.
var ErrNoToken = errors.New("no flex token")

// Client fetches Flex statements. HTTP, Base and UserAgent may be replaced.
type Client struct {
	HTTP      *http.Client
	Base      string
	Token     string
	UserAgent string
}

// New returns a client for a token with a 60s HTTP timeout.
func New(token string) *Client {
	return &Client{HTTP: &http.Client{Timeout: 60 * time.Second}, Base: DefaultBase, Token: token, UserAgent: "ibkrctl"}
}

// Request is the handle SendRequest returns for a statement being generated.
type Request struct {
	ReferenceCode string
	URL           string
}

// Error is a Flex service error (ErrorCode / ErrorMessage from the response).
type Error struct {
	Code    string
	Message string
}

func (e *Error) Error() string { return fmt.Sprintf("flex error %s: %s", e.Code, e.Message) }

type statusResp struct {
	XMLName       xml.Name
	Status        string `xml:"Status"`
	ReferenceCode string `xml:"ReferenceCode"`
	URL           string `xml:"Url"`
	ErrorCode     string `xml:"ErrorCode"`
	ErrorMessage  string `xml:"ErrorMessage"`
}

func (c *Client) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	if c.UserAgent != "" {
		req.Header.Set("User-Agent", c.UserAgent)
	}
	hc := c.HTTP
	if hc == nil {
		hc = http.DefaultClient
	}
	resp, err := hc.Do(req)
	if err != nil {
		// url.Error embeds the full URL, which carries the token.
		var ue *url.Error
		if errors.As(err, &ue) {
			return nil, fmt.Errorf("flex %s: %w", ue.Op, ue.Err)
		}
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("flex HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return b, nil
}

func (c *Client) base() string {
	if c.Base == "" {
		return DefaultBase
	}
	return c.Base
}

// SendRequest asks Flex to generate the statement for a saved query id.
func (c *Client) SendRequest(ctx context.Context, queryID string) (Request, error) {
	if c.Token == "" {
		return Request{}, ErrNoToken
	}
	q := url.Values{"t": {c.Token}, "q": {queryID}, "v": {"3"}}
	body, err := c.get(ctx, c.base()+".SendRequest?"+q.Encode())
	if err != nil {
		return Request{}, err
	}
	var sr statusResp
	if err := xml.Unmarshal(body, &sr); err != nil {
		return Request{}, fmt.Errorf("flex send parse: %w", err)
	}
	if sr.Status != "Success" || sr.ReferenceCode == "" {
		return Request{}, &Error{Code: sr.ErrorCode, Message: sr.ErrorMessage}
	}
	return Request{ReferenceCode: sr.ReferenceCode, URL: sr.URL}, nil
}

// GetStatement makes one fetch attempt. It returns ErrNotReady while the
// statement is still generating, and the raw FlexQueryResponse XML once ready.
func (c *Client) GetStatement(ctx context.Context, r Request) ([]byte, error) {
	if c.Token == "" {
		return nil, ErrNoToken
	}
	getURL := r.URL
	if getURL == "" {
		getURL = c.base() + ".GetStatement"
	}
	q := url.Values{"t": {c.Token}, "q": {r.ReferenceCode}, "v": {"3"}}
	st, err := c.get(ctx, getURL+"?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var sr statusResp
	if xml.Unmarshal(st, &sr) == nil && sr.XMLName.Local == "FlexStatementResponse" {
		// 1019 = generation in progress, 1018 = rate limited; both clear on retry.
		if sr.Status == "Warn" || sr.ErrorCode == "1019" || sr.ErrorCode == "1018" {
			return nil, ErrNotReady
		}
		return nil, &Error{Code: sr.ErrorCode, Message: sr.ErrorMessage}
	}
	return st, nil
}

// Statement runs SendRequest, then polls GetStatement every poll (default 3s)
// up to tries times (default 10). It stops early when ctx is done.
func (c *Client) Statement(ctx context.Context, queryID string, poll time.Duration, tries int) ([]byte, error) {
	r, err := c.SendRequest(ctx, queryID)
	if err != nil {
		return nil, err
	}
	if poll <= 0 {
		poll = 3 * time.Second
	}
	if tries <= 0 {
		tries = 10
	}
	for i := 0; i < tries; i++ {
		st, err := c.GetStatement(ctx, r)
		if !errors.Is(err, ErrNotReady) {
			return st, err
		}
		if i == tries-1 {
			break
		}
		t := time.NewTimer(poll)
		select {
		case <-ctx.Done():
			t.Stop()
			return nil, ctx.Err()
		case <-t.C:
		}
	}
	return nil, fmt.Errorf("%w after %d tries", ErrNotReady, tries)
}

// Fetch runs Statement and parses the result.
func (c *Client) Fetch(ctx context.Context, queryID string, poll time.Duration, tries int) (*Response, error) {
	b, err := c.Statement(ctx, queryID, poll, tries)
	if err != nil {
		return nil, err
	}
	return Parse(b)
}
