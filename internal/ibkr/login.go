package ibkr

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/chromedp/chromedp"
)

// LoginParams drives the headless-browser login.
type LoginParams struct {
	Username string
	Password string
	// Headful shows the browser window (useful the first time / for selector debugging).
	Headful bool
	// OTP is a static 2FA code to type if the page asks for one. Empty = wait for
	// an out-of-band approval (IB Key push) instead.
	OTP string
	// OTPPrompt is called when the page shows a code field and no OTP was given.
	// It returns the code the user reads off their app/card. Nil = just wait.
	OTPPrompt func() (string, error)
	// ChromePath overrides the Chrome/Chromium executable.
	ChromePath string
	// LoginURL is the gateway login page.
	LoginURL string
	// Timeout bounds the whole flow.
	Timeout time.Duration
	// Log receives progress lines.
	Log func(string)
}

// selectors tried in order for each field (the gateway form is JS-rendered and
// its ids have shifted across gateway builds).
var (
	userSel   = []string{"#user_name", "input[name=username]", "#username", "input[name=user_name]"}
	passSel   = []string{"#password", "input[name=password]", "#pass"}
	submitSel = []string{"#submitForm", "button[type=submit]", "input[type=submit]", ".btn-primary"}
	otpSel    = []string{"input[name=chlginput]", "#chlginput", "input[name=code]", "#code", "input[name=otp]"}
)

func (p *LoginParams) logf(f string, a ...any) {
	if p.Log != nil {
		p.Log(fmt.Sprintf(f, a...))
	}
}

// BrowserLogin fills and submits the gateway login form with a headless browser.
// It returns once the form + 2FA are submitted; the caller polls auth/status.
func BrowserLogin(ctx context.Context, p LoginParams) error {
	if p.Username == "" || p.Password == "" {
		return fmt.Errorf("username and password required: run `ibkrctl init`")
	}
	if p.Timeout == 0 {
		p.Timeout = 3 * time.Minute
	}
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.Flag("headless", !p.Headful),
		chromedp.Flag("ignore-certificate-errors", true),
		chromedp.Flag("disable-gpu", true),
	)
	if p.ChromePath != "" {
		opts = append(opts, chromedp.ExecPath(p.ChromePath))
	}
	alloc, cancelAlloc := chromedp.NewExecAllocator(ctx, opts...)
	defer cancelAlloc()
	bctx, cancelB := chromedp.NewContext(alloc)
	defer cancelB()
	bctx, cancelT := context.WithTimeout(bctx, p.Timeout)
	defer cancelT()

	p.logf("opening login page")
	if err := chromedp.Run(bctx, chromedp.Navigate(p.LoginURL)); err != nil {
		return fmt.Errorf("navigate: %w", err)
	}

	// Fill username, password, submit.
	if err := fillFirst(bctx, userSel, p.Username); err != nil {
		return fmt.Errorf("username field: %w", err)
	}
	if err := fillFirst(bctx, passSel, p.Password); err != nil {
		return fmt.Errorf("password field: %w", err)
	}
	p.logf("credentials entered, submitting")
	if err := clickFirst(bctx, submitSel); err != nil {
		return fmt.Errorf("submit: %w", err)
	}

	// Second factor: if a code field appears within a few seconds, fill it;
	// otherwise assume an out-of-band push (IB Key) the user approves on their phone.
	sel, ok := waitAny(bctx, otpSel, 6*time.Second)
	if ok {
		code := p.OTP
		if code == "" && p.OTPPrompt != nil {
			p.logf("2FA code requested")
			c, err := p.OTPPrompt()
			if err != nil {
				return err
			}
			code = strings.TrimSpace(c)
		}
		if code == "" {
			return fmt.Errorf("the login asked for a 2FA code; pass --otp <code>")
		}
		if err := chromedp.Run(bctx, chromedp.SendKeys(sel, code, chromedp.ByQuery)); err != nil {
			return fmt.Errorf("enter 2FA code: %w", err)
		}
		_ = clickFirst(bctx, submitSel)
		p.logf("2FA code submitted")
	} else {
		p.logf("no code field; approve the push on your phone if prompted")
	}
	return nil
}

func fillFirst(ctx context.Context, sels []string, val string) error {
	sel, ok := waitAny(ctx, sels, 20*time.Second)
	if !ok {
		return fmt.Errorf("none of %v appeared", sels)
	}
	return chromedp.Run(ctx,
		chromedp.Clear(sel, chromedp.ByQuery),
		chromedp.SendKeys(sel, val, chromedp.ByQuery),
	)
}

func clickFirst(ctx context.Context, sels []string) error {
	sel, ok := waitAny(ctx, sels, 15*time.Second)
	if !ok {
		return fmt.Errorf("none of %v appeared", sels)
	}
	return chromedp.Run(ctx, chromedp.Click(sel, chromedp.ByQuery))
}

// waitAny returns the first selector that becomes visible within d.
func waitAny(ctx context.Context, sels []string, d time.Duration) (string, bool) {
	deadline := time.Now().Add(d)
	for time.Now().Before(deadline) {
		for _, s := range sels {
			c, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			err := chromedp.Run(c, chromedp.WaitVisible(s, chromedp.ByQuery))
			cancel()
			if err == nil {
				return s, true
			}
		}
	}
	return "", false
}
