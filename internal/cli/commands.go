package cli

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/laurenschristian/ibkrctl/internal/ibkr"
)

func loginCmd() *cobra.Command {
	var timeout int
	var manual, headless bool
	var otp string
	c := &cobra.Command{
		Use:   "login",
		Short: "Log in to IBKR (auto-fills stored credentials via a headless browser)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if st, err := client.AuthStatus(ctx); err == nil && st.Authenticated {
				fmt.Println("already authenticated")
				return nil
			}
			gw := newGateway()
			loginURL := gw.LoginURL()

			pass, _ := cfg.Password()
			if manual || cfg.Username == "" || pass == "" {
				fmt.Printf("opening %s\n", loginURL)
				if err := openBrowser(loginURL); err != nil {
					fmt.Printf("open this URL to log in: %s\n", loginURL)
				}
				return waitAuthed(ctx, timeout)
			}

			authed := func() bool {
				st, err := client.AuthStatus(ctx)
				return err == nil && st.Authenticated
			}
			if !headless {
				fmt.Println("a browser window will open with your credentials filled in.")
				fmt.Println("complete any 2FA there (pick Text, enter the SMS code); ibkrctl waits for the session.")
			}
			err := ibkr.BrowserLogin(ctx, ibkr.LoginParams{
				Username: cfg.Username,
				Password: pass,
				Headful:  !headless,
				OTP:      otp,
				OTPPrompt: func() (string, error) {
					fmt.Print("2FA code: ")
					var code string
					_, _ = fmt.Scanln(&code)
					return code, nil
				},
				LoginURL: loginURL,
				Timeout:  time.Duration(timeout) * time.Second,
				Log:      func(m string) { fmt.Println(m) },
				Authed:   authed,
			})
			if err != nil {
				return fmt.Errorf("browser login: %w (try `ibkrctl login --manual` to log in yourself)", err)
			}
			fmt.Println("logged in")
			return nil
		},
	}
	c.Flags().IntVar(&timeout, "timeout", 180, "seconds to wait for authentication")
	c.Flags().BoolVar(&manual, "manual", false, "just open your normal browser; type everything yourself")
	c.Flags().BoolVar(&headless, "headless", false, "run the login browser hidden (only works with no interactive 2FA, or --otp)")
	c.Flags().StringVar(&otp, "otp", "", "static 2FA code to enter if the login asks for one")
	return c
}

// waitAuthed polls auth/status until authenticated or timeout.
func waitAuthed(ctx context.Context, timeout int) error {
	fmt.Println("waiting for authentication (Ctrl-C to cancel)...")
	deadline := time.Now().Add(time.Duration(timeout) * time.Second)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(2 * time.Second):
		}
		if st, err := client.AuthStatus(ctx); err == nil && st.Authenticated {
			fmt.Println("authenticated")
			return nil
		}
	}
	return errors.New("login timed out")
}

func logoutCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "End the brokerage session",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := client.Logout(cmd.Context()); err != nil {
				return err
			}
			fmt.Println("logged out")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show authentication and session state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			st, err := client.AuthStatus(cmd.Context())
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(st)
			}
			fmt.Printf("authenticated  %v\n", st.Authenticated)
			fmt.Printf("connected      %v\n", st.Connected)
			fmt.Printf("competing      %v\n", st.Competing)
			if st.Fail != "" {
				fmt.Printf("fail           %s\n", st.Fail)
			}
			if !st.Authenticated {
				fmt.Println("run `ibkrctl login`")
			}
			return nil
		},
	}
}

func tickleCmd() *cobra.Command {
	var quiet bool
	c := &cobra.Command{
		Use:   "tickle",
		Short: "Keep the session alive (used by the keepalive agent)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			t, err := client.Tickle(cmd.Context())
			if err != nil {
				if quiet {
					return nil // keepalive agent must not spam errors when logged out
				}
				return err
			}
			if quiet {
				return nil
			}
			if flagJSON {
				return emit(t)
			}
			fmt.Printf("session        %s\n", t.Session)
			fmt.Printf("authenticated  %v\n", t.Iserver.AuthStatus.Authenticated)
			return nil
		},
	}
	c.Flags().BoolVar(&quiet, "quiet", false, "no output, never error (for automation)")
	return c
}

func accountCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "account",
		Short: "List brokerage accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			accts, err := client.Accounts(cmd.Context())
			if err != nil {
				return err
			}
			if flagJSON {
				return emit(accts)
			}
			if len(accts) == 0 {
				fmt.Println("no accounts (are you logged in?)")
				return nil
			}
			for _, a := range accts {
				name := a.DisplayName
				if name == "" {
					name = a.AccountTitle
				}
				fmt.Printf("%-12s %s\n", a.ID, name)
			}
			return nil
		},
	}
}

// resolveAccount picks the account id: --account flag, config, or the first account.
func resolveAccount(ctx context.Context, flag string) (string, error) {
	if flag != "" {
		return flag, nil
	}
	if cfg.Account != "" {
		return cfg.Account, nil
	}
	accts, err := client.Accounts(ctx)
	if err != nil {
		return "", err
	}
	if len(accts) == 0 {
		return "", errors.New("no account available: run `ibkrctl login`")
	}
	// Skip pseudo-accounts like "All"; prefer a real account id (Uxxxxxxx).
	for _, a := range accts {
		if strings.HasPrefix(a.ID, "U") {
			return a.ID, nil
		}
	}
	return accts[0].ID, nil
}

func openBrowser(url string) error {
	var cmd string
	var args []string
	switch runtime.GOOS {
	case "darwin":
		cmd = "open"
	case "windows":
		cmd, args = "cmd", []string{"/c", "start"}
	default:
		cmd = "xdg-open"
	}
	args = append(args, url)
	return exec.CommandContext(context.Background(), cmd, args...).Start()
}
