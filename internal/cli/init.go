package cli

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/laurenschristian/ibkrctl/internal/config"
)

func initCmd() *cobra.Command {
	var username, twofa string
	var noKeychain bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Store IBKR username and password (macOS Keychain) for auto-login",
		RunE: func(_ *cobra.Command, _ []string) error {
			r := bufio.NewReader(os.Stdin)
			if username == "" {
				fmt.Print("IBKR username: ")
				line, _ := r.ReadString('\n')
				username = strings.TrimSpace(line)
			}
			if username == "" {
				return fmt.Errorf("username required")
			}
			fmt.Print("IBKR password: ")
			pw, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return err
			}
			password := strings.TrimSpace(string(pw))

			cfg.Username = username
			if twofa != "" {
				cfg.TwoFA = twofa
			}
			if noKeychain {
				// Store the password in the config file (0600). Least secure.
				cfg.PasswordCmd = ""
				return fmt.Errorf("--no-keychain is not supported for the password; use the Keychain path")
			}
			// Store in the login keychain and reference it via password_cmd.
			if err := keychainSet("ibkrctl", username, password); err != nil {
				return fmt.Errorf("keychain store: %w", err)
			}
			cfg.PasswordCmd = fmt.Sprintf("security find-generic-password -s ibkrctl -a %s -w", username)
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("saved. username %s, password in Keychain (service ibkrctl).\n", username)
			fmt.Println("next: ibkrctl login")
			return nil
		},
	}
	c.Flags().StringVar(&username, "username", "", "IBKR username")
	c.Flags().StringVar(&twofa, "twofa", "", "2FA method note: ibkey | code | none")
	c.Flags().BoolVar(&noKeychain, "no-keychain", false, "(reserved)")
	return c
}

// keychainSet writes a generic password to the macOS login keychain, replacing any existing one.
func keychainSet(service, account, password string) error {
	// -U updates if it already exists.
	cmd := exec.CommandContext(context.Background(), "security", "add-generic-password",
		"-s", service, "-a", account, "-w", password, "-U")
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("%w: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}
