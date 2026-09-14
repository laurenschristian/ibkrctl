package cli

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/term"

	"github.com/laurenschristian/ibkrctl/internal/config"
	"github.com/laurenschristian/ibkrctl/internal/ibkr"
)

func flexCmd() *cobra.Command {
	var tries int
	var poll int
	c := &cobra.Command{
		Use:   "flex <query>",
		Short: "Fetch a Flex statement (trades, realized P&L, dividends, cash, tax lots)",
		Long: "Run an IBKR Flex Web Service query by saved name or raw query id. " +
			"Flex uses a token (see `ibkrctl flex init`) and needs no gateway or 2FA. " +
			"Enable Flex Web Service and build a query in Client Portal first " +
			"(Settings > Account Settings > Flex Web Service).",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			token, err := cfg.FlexToken()
			if err != nil {
				return fmt.Errorf("read flex token: %w", err)
			}
			if token == "" {
				return fmt.Errorf("no flex token: run `ibkrctl flex init`")
			}
			fc := ibkr.NewFlex(token)
			xmlBytes, err := fc.Statement(cmd.Context(), cfg.FlexQueryByName(args[0]),
				time.Duration(poll)*time.Second, tries)
			if err != nil {
				return err
			}
			fmt.Print(redactFlex(string(xmlBytes)))
			if !strings.HasSuffix(string(xmlBytes), "\n") {
				fmt.Println()
			}
			return nil
		},
	}
	c.Flags().IntVar(&tries, "tries", 10, "times to poll for the generated statement")
	c.Flags().IntVar(&poll, "poll", 3, "seconds between polls")
	c.AddCommand(flexInitCmd(), flexQueryCmd())
	return c
}

func flexInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Store the Flex Web Service token in the macOS Keychain",
		RunE: func(_ *cobra.Command, _ []string) error {
			fmt.Print("Flex Web Service token: ")
			tok, err := term.ReadPassword(int(os.Stdin.Fd()))
			fmt.Println()
			if err != nil {
				return err
			}
			token := strings.TrimSpace(string(tok))
			if token == "" {
				return fmt.Errorf("token required")
			}
			if err := keychainSet("ibkrctl-flex", "flex", token); err != nil {
				return fmt.Errorf("keychain store: %w", err)
			}
			cfg.FlexTokenCmd = "security find-generic-password -s ibkrctl-flex -a flex -w"
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Println("saved. token in Keychain (service ibkrctl-flex).")
			fmt.Println("next: ibkrctl flex query add <name> <queryId>, then ibkrctl flex <name>")
			return nil
		},
	}
}

func flexQueryCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "query",
		Short: "Manage saved Flex query ids",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if len(cfg.FlexQueries) == 0 {
				fmt.Println("no flex queries (add one with `ibkrctl flex query add <name> <id>`)")
				return nil
			}
			b, w := newTab()
			fmt.Fprintln(w, "NAME\tQUERY ID")
			for _, q := range cfg.FlexQueries {
				fmt.Fprintf(w, "%s\t%s\n", q.Name, q.ID)
			}
			_ = w.Flush()
			fmt.Print(b.String())
			return nil
		},
	}
	c.AddCommand(flexQueryAddCmd(), flexQueryRemoveCmd())
	return c
}

func flexQueryAddCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "add <name> <queryId>",
		Short: "Add or replace a saved Flex query",
		Args:  cobra.ExactArgs(2),
		RunE: func(_ *cobra.Command, args []string) error {
			out := cfg.FlexQueries[:0:0]
			for _, q := range cfg.FlexQueries {
				if q.Name != args[0] {
					out = append(out, q)
				}
			}
			cfg.FlexQueries = append(out, config.FlexQuery{Name: args[0], ID: args[1]})
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("saved flex query %q -> %s\n", args[0], args[1])
			return nil
		},
	}
}

func flexQueryRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a saved Flex query",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			out := cfg.FlexQueries[:0:0]
			found := false
			for _, q := range cfg.FlexQueries {
				if q.Name == args[0] {
					found = true
					continue
				}
				out = append(out, q)
			}
			if !found {
				return fmt.Errorf("no flex query named %q", args[0])
			}
			cfg.FlexQueries = out
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("removed flex query %q\n", args[0])
			return nil
		},
	}
}

var flexNameAttr = regexp.MustCompile(`(?i)\b(accountAlias|name|acctAlias|title|firstName|lastName)="[^"]*"`)

// redactFlex hides PII in a Flex XML statement: real account ids become their
// aliases and personal-name attributes are blanked (only when Redact is on).
func redactFlex(s string) string {
	if cfg == nil || !cfg.Redact {
		return s
	}
	for id, alias := range cfg.RedactMap() {
		s = strings.ReplaceAll(s, id, alias)
	}
	return flexNameAttr.ReplaceAllString(s, `$1="***"`)
}
