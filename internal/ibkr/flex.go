package ibkr

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// FlexBase is the IBKR Flex Web Service endpoint (separate from the gateway;
// token-authenticated, no browser login).
const FlexBase = "https://gdcdyn.interactivebrokers.com/Universal/servlet/FlexStatementService"

// FlexClient fetches Flex statements (trades, realized P&L, cash, tax lots).
type FlexClient struct {
	HTTP  *http.Client
	Base  string
	Token string
}

// NewFlex returns a Flex client for a token.
func NewFlex(token string) *FlexClient {
	return &FlexClient{HTTP: &http.Client{Timeout: 60 * time.Second}, Base: FlexBase, Token: token}
}

type flexResp struct {
	Status        string `xml:"Status"`
	ReferenceCode string `xml:"ReferenceCode"`
	URL           string `xml:"Url"`
	ErrorCode     string `xml:"ErrorCode"`
	ErrorMessage  string `xml:"ErrorMessage"`
}

func (c *FlexClient) get(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ibkrctl")
	resp, err := c.HTTP.Do(req)
	if err != nil {
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

// Statement runs the two-step Flex flow: request the query, then poll for the
// generated statement until it is ready. Returns the raw statement XML.
func (c *FlexClient) Statement(ctx context.Context, queryID string, poll time.Duration, tries int) ([]byte, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("no flex token: run `ibkrctl flex init`")
	}
	q := url.Values{"t": {c.Token}, "q": {queryID}, "v": {"3"}}
	body, err := c.get(ctx, c.Base+".SendRequest?"+q.Encode())
	if err != nil {
		return nil, err
	}
	var sr flexResp
	if err := xml.Unmarshal(body, &sr); err != nil {
		return nil, fmt.Errorf("flex send parse: %w", err)
	}
	if sr.Status != "Success" || sr.ReferenceCode == "" {
		return nil, fmt.Errorf("flex request failed: %s %s", sr.ErrorCode, sr.ErrorMessage)
	}
	getURL := sr.URL
	if getURL == "" {
		getURL = c.Base + ".GetStatement"
	}
	gq := url.Values{"t": {c.Token}, "q": {sr.ReferenceCode}, "v": {"3"}}
	if poll <= 0 {
		poll = 3 * time.Second
	}
	if tries <= 0 {
		tries = 10
	}
	for i := 0; i < tries; i++ {
		st, err := c.get(ctx, getURL+"?"+gq.Encode())
		if err != nil {
			return nil, err
		}
		// A not-ready statement comes back as a small FlexStatementResponse with
		// Status=Warn; a ready one is the full FlexQueryResponse document.
		var warn flexResp
		if xml.Unmarshal(st, &warn) == nil && warn.Status == "Warn" {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(poll):
			}
			continue
		}
		return st, nil
	}
	return nil, fmt.Errorf("flex statement not ready after %d tries", tries)
}
