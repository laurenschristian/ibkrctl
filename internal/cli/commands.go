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

	"github.com/laurenschristian/ibkrctl/internal/config"
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
	c := &cobra.Command{
		Use:   "account",
		Short: "List brokerage accounts (aliases when redaction is on)",
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
				label := cfg.AliasFor(a.ID)
				id := a.ID
				if cfg.Redact {
					id = label // never print the real id
				}
				fmt.Printf("%-12s %s\n", id, label)
			}
			return nil
		},
	}
	c.AddCommand(accountAliasCmd(), accountAutonameCmd(), redactCmd(), accountSwitchCmd())
	return c
}

func accountAliasCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "alias <account-id|alias> <new-alias>",
		Short: "Give an account a stable alias (so its real number stays hidden)",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			id := cfg.IDFor(args[0])
			set := false
			for i := range cfg.Accounts {
				if cfg.Accounts[i].ID == id {
					cfg.Accounts[i].Alias = args[1]
					set = true
				}
			}
			if !set {
				cfg.Accounts = append(cfg.Accounts, config.AccountAlias{ID: id, Alias: args[1]})
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("aliased %s -> %s\n", args[1], args[1])
			return nil
		},
	}
}

func accountSwitchCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "switch <account>",
		Short: "Set the active trading account (alias or id) for order routing",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.SwitchAccount(cmd.Context(), cfg.IDFor(args[0]))
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func accountAutonameCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "autoname",
		Short: "Assign account-1, account-2, ... to the live real accounts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			accts, err := client.Accounts(cmd.Context())
			if err != nil {
				return err
			}
			n := 0
			for _, a := range accts {
				if !strings.HasPrefix(a.ID, "U") {
					continue // skip pseudo-accounts like "All"
				}
				n++
				alias := fmt.Sprintf("account-%d", n)
				found := false
				for i := range cfg.Accounts {
					if cfg.Accounts[i].ID == a.ID {
						found = true
					}
				}
				if !found {
					cfg.Accounts = append(cfg.Accounts, config.AccountAlias{ID: a.ID, Alias: alias})
				}
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("named %d account(s); run `ibkrctl account redact on` to hide the numbers\n", n)
			return nil
		},
	}
}

func redactCmd() *cobra.Command {
	return &cobra.Command{
		Use:       "redact <on|off>",
		Short:     "Hide real account numbers and names in all output",
		Args:      cobra.ExactArgs(1),
		ValidArgs: []string{"on", "off"},
		RunE: func(_ *cobra.Command, args []string) error {
			switch args[0] {
			case "on":
				cfg.Redact = true
			case "off":
				cfg.Redact = false
			default:
				return fmt.Errorf("use on or off")
			}
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("redaction %s\n", args[0])
			return nil
		},
	}
}

// resolveAccount picks the account id: --account flag, config, or the first account.
func resolveAccount(ctx context.Context, flag string) (string, error) {
	if flag != "" {
		return cfg.IDFor(flag), nil // accept an alias or a real id
	}
	if id := cfg.DefaultAccountID(); id != "" {
		return id, nil
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
