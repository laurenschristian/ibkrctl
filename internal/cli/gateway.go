package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/laurenschristian/ibkrctl/internal/config"
	"github.com/laurenschristian/ibkrctl/internal/ibkr"
)

func newGateway() *ibkr.Gateway {
	return &ibkr.Gateway{
		Dir:     cfg.GatewayDir,
		Java:    cfg.JavaBin,
		Port:    cfg.Port,
		Support: filepath.Dir(config.Path()),
	}
}

func gatewayCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "gateway",
		Short: "Manage the local Client Portal Gateway (launchd agent)",
	}
	c.AddCommand(gatewayInstallCmd(), gatewayStartCmd(), gatewayStopCmd(), gatewayRestartCmd(), gatewayStatusCmd(), gatewayUninstallCmd())
	return c
}

func gatewayInstallCmd() *cobra.Command {
	var from, java string
	c := &cobra.Command{
		Use:   "install",
		Short: "Install the gateway (copy clientportal.gw + JRE), patch the port, load the launchd agents",
		RunE: func(_ *cobra.Command, _ []string) error {
			gw := newGateway()
			// Resolve the gateway source.
			src := from
			if src == "" {
				d, err := ibkr.DiscoverGatewaySrc()
				if err != nil {
					return err
				}
				src = d
			}
			fmt.Printf("copying gateway from %s\n", src)
			if err := gw.CopyFrom(src); err != nil {
				return err
			}
			// Resolve java.
			if java == "" {
				j, err := ibkr.DiscoverJava()
				if err != nil {
					return err
				}
				java = j
			}
			// Copy the JRE into the support dir so the install survives npx eviction.
			if stable, cerr := ibkr.CopyJava(java, config.GatewayHome()); cerr == nil {
				java = stable
			}
			gw.Java = java
			fmt.Printf("java %s\n", java)
			if err := gw.PatchConf(); err != nil {
				return err
			}
			// Persist resolved paths.
			cfg.GatewayDir = gw.Dir
			cfg.JavaBin = java
			if err := config.Save(cfg); err != nil {
				return err
			}
			// Write and load launchd agents.
			gp, err := gw.WriteGatewayPlist()
			if err != nil {
				return err
			}
			self, _ := os.Executable()
			kp, err := gw.WriteKeepalivePlist(self)
			if err != nil {
				return err
			}
			if err := ibkr.Load(ibkr.GatewayLabel); err != nil {
				return err
			}
			if err := ibkr.Load(ibkr.KeepaliveLabel); err != nil {
				return err
			}
			fmt.Printf("agents loaded:\n  %s\n  %s\n", gp, kp)
			fmt.Printf("gateway listening on port %d\n", gw.Port)
			fmt.Println("next: ibkrctl login")
			return nil
		},
	}
	c.Flags().StringVar(&from, "from", "", "copy clientportal.gw from this directory (default: auto-detect)")
	c.Flags().StringVar(&java, "java", "", "java executable to run the gateway (default: auto-detect)")
	return c
}

func gatewayStartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Load the gateway launchd agents",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !newGateway().Installed() {
				return fmt.Errorf("gateway not installed: run `ibkrctl gateway install`")
			}
			if err := ibkr.Load(ibkr.GatewayLabel); err != nil {
				return err
			}
			if err := ibkr.Load(ibkr.KeepaliveLabel); err != nil {
				return err
			}
			fmt.Println("gateway started")
			return nil
		},
	}
}

func gatewayStopCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "stop",
		Short: "Unload the gateway launchd agents",
		RunE: func(_ *cobra.Command, _ []string) error {
			_ = ibkr.Unload(ibkr.KeepaliveLabel)
			if err := ibkr.Unload(ibkr.GatewayLabel); err != nil {
				return err
			}
			fmt.Println("gateway stopped")
			return nil
		},
	}
}

func gatewayRestartCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "restart",
		Short: "Reload the gateway launchd agents",
		RunE: func(_ *cobra.Command, _ []string) error {
			if err := ibkr.Load(ibkr.GatewayLabel); err != nil {
				return err
			}
			if err := ibkr.Load(ibkr.KeepaliveLabel); err != nil {
				return err
			}
			fmt.Println("gateway restarted")
			return nil
		},
	}
}

func gatewayStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show gateway install and agent state",
		RunE: func(cmd *cobra.Command, _ []string) error {
			gw := newGateway()
			state := map[string]any{
				"installed":        gw.Installed(),
				"dir":              gw.Dir,
				"port":             gw.Port,
				"gateway_loaded":   ibkr.AgentLoaded(ibkr.GatewayLabel),
				"keepalive_loaded": ibkr.AgentLoaded(ibkr.KeepaliveLabel),
			}
			if st, err := client.AuthStatus(cmd.Context()); err == nil {
				state["authenticated"] = st.Authenticated
			}
			if flagJSON {
				return emit(state)
			}
			fmt.Printf("installed        %v\n", state["installed"])
			fmt.Printf("dir              %s\n", gw.Dir)
			fmt.Printf("port             %d\n", gw.Port)
			fmt.Printf("gateway agent    %v\n", state["gateway_loaded"])
			fmt.Printf("keepalive agent  %v\n", state["keepalive_loaded"])
			if a, ok := state["authenticated"]; ok {
				fmt.Printf("authenticated    %v\n", a)
			} else {
				fmt.Printf("authenticated    unreachable\n")
			}
			return nil
		},
	}
}

func gatewayUninstallCmd() *cobra.Command {
	var yes bool
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Unload agents and remove the installed gateway files",
		RunE: func(_ *cobra.Command, _ []string) error {
			if !yes {
				return fmt.Errorf("this removes %s and the launchd agents; pass --yes to confirm", cfg.GatewayDir)
			}
			_ = ibkr.Unload(ibkr.KeepaliveLabel)
			_ = ibkr.Unload(ibkr.GatewayLabel)
			_ = os.Remove(filepath.Join(ibkr.AgentDir(), ibkr.GatewayLabel+".plist"))
			_ = os.Remove(filepath.Join(ibkr.AgentDir(), ibkr.KeepaliveLabel+".plist"))
			if err := os.RemoveAll(cfg.GatewayDir); err != nil {
				return err
			}
			fmt.Println("gateway uninstalled")
			return nil
		},
	}
	c.Flags().BoolVar(&yes, "yes", false, "confirm removal")
	return c
}
