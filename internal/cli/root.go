// Package cli wires the cobra commands.
package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/laurenschristian/ibkrctl/internal/config"
	"github.com/laurenschristian/ibkrctl/internal/ibkr"
)

var (
	Version = "dev"

	flagJSON bool
	cfg      *config.Config
	client   *ibkr.Client
)

// noSetup lists commands that must run without loading config/client wiring.
func noSetup(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "completion", "help", "__complete", "__completeNoDesc":
		return true
	}
	if p := cmd.Parent(); p != nil {
		switch p.Name() {
		case "completion", "help":
			return true
		}
	}
	return false
}

func Root() *cobra.Command {
	root := &cobra.Command{
		Use:           "ibkrctl",
		Short:         "CLI and MCP server for Interactive Brokers",
		Version:       Version,
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			if noSetup(cmd) {
				return nil
			}
			var err error
			if cfg, err = config.Load(); err != nil {
				return err
			}
			client = ibkr.New(cfg.URL)
			return nil
		},
	}
	root.PersistentFlags().BoolVar(&flagJSON, "json", false, "print raw JSON")
	root.AddCommand(
		initCmd(),
		gatewayCmd(),
		loginCmd(),
		logoutCmd(),
		statusCmd(),
		tickleCmd(),
		accountCmd(),
		positionsCmd(),
		pnlCmd(),
		ordersCmd(),
		summaryCmd(),
		ledgerCmd(),
		allocationCmd(),
		tradesCmd(),
		quoteCmd(),
		searchCmd(),
		infoCmd(),
		historyCmd(),
		fundamentalsCmd(),
		scannerCmd(),
		chainCmd(),
		watchlistsCmd(),
		newsCmd(),
		notificationsCmd(),
		fxCmd(),
		futuresCmd(),
		alertsCmd(),
		transactionsCmd(),
		placeCmd(),
		modifyCmd(),
		cancelCmd(),
		rulesCmd(),
		positionCmd(),
		rawCmd(),
		doctorCmd(),
		mcpCmd(),
	)
	return root
}

func emit(v any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(redact(v))
}

func doctorCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Check config, gateway install, and reachability",
		RunE: func(cmd *cobra.Command, _ []string) error {
			gw := newGateway()
			state := map[string]any{
				"config":          config.Path(),
				"url":             client.BaseURL,
				"gateway_dir":     cfg.GatewayDir,
				"gateway_present": gw.Installed(),
				"java":            cfg.JavaBin,
				"agent_loaded":    ibkr.AgentLoaded(ibkr.GatewayLabel),
			}
			if st, err := client.AuthStatus(cmd.Context()); err == nil {
				state["authenticated"] = st.Authenticated
			} else {
				state["authenticated"] = false
				state["auth_error"] = err.Error()
			}
			if flagJSON {
				return emit(state)
			}
			fmt.Printf("config          %s\n", config.Path())
			fmt.Printf("url             %s\n", client.BaseURL)
			fmt.Printf("gateway dir     %s\n", cfg.GatewayDir)
			fmt.Printf("gateway present %v\n", gw.Installed())
			fmt.Printf("java            %s\n", orNone(cfg.JavaBin))
			fmt.Printf("agent loaded    %v\n", ibkr.AgentLoaded(ibkr.GatewayLabel))
			fmt.Printf("authenticated   %v\n", state["authenticated"])
			if e, ok := state["auth_error"]; ok {
				fmt.Printf("auth error      %s\n", e)
			}
			return nil
		},
	}
}

func orNone(s string) string {
	if s == "" {
		return "(unset)"
	}
	return s
}
