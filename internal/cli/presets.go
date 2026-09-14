package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/laurenschristian/ibkrctl/internal/config"
)

func presetsCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "presets",
		Short: "Manage saved order shapes replayed with `place --preset`",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if flagJSON {
				return emit(cfg.Presets)
			}
			if len(cfg.Presets) == 0 {
				fmt.Println("no presets (add one with `ibkrctl presets add`)")
				return nil
			}
			b, w := newTab()
			fmt.Fprintln(w, "NAME\tSIDE\tTYPE\tTIF\tQTY\tTP%\tSL%")
			for _, p := range cfg.Presets {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					p.Name, p.Side, p.Type, p.TIF, trimFloat(p.Qty),
					pct(p.TakeProfitPct), pct(p.StopLossPct))
			}
			_ = w.Flush()
			fmt.Print(b.String())
			return nil
		},
	}
	c.AddCommand(presetsAddCmd(), presetsRemoveCmd())
	return c
}

func pct(f float64) string {
	if f == 0 {
		return ""
	}
	return trimFloat(round2(f * 100))
}

func presetsAddCmd() *cobra.Command {
	var p config.Preset
	c := &cobra.Command{
		Use:   "add <name>",
		Short: "Add or replace a preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p.Name = args[0]
			p.Side = strings.ToUpper(p.Side)
			p.Type = strings.ToUpper(p.Type)
			p.TIF = strings.ToUpper(p.TIF)
			out := cfg.Presets[:0:0]
			for _, ex := range cfg.Presets {
				if ex.Name != p.Name {
					out = append(out, ex)
				}
			}
			cfg.Presets = append(out, p)
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("saved preset %q\n", p.Name)
			return nil
		},
	}
	c.Flags().StringVar(&p.Side, "side", "", "BUY or SELL")
	c.Flags().StringVar(&p.Type, "type", "", "MKT or LMT")
	c.Flags().StringVar(&p.TIF, "tif", "", "DAY, GTC, IOC")
	c.Flags().Float64Var(&p.Qty, "qty", 0, "default quantity")
	c.Flags().Float64Var(&p.TakeProfitPct, "take-profit-pct", 0, "bracket take-profit offset (0.05 = 5%)")
	c.Flags().Float64Var(&p.StopLossPct, "stop-loss-pct", 0, "bracket stop offset (0.03 = 3%)")
	return c
}

func presetsRemoveCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "rm <name>",
		Short: "Remove a preset",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cfg.Presets[:0:0]
			found := false
			for _, ex := range cfg.Presets {
				if ex.Name == args[0] {
					found = true
					continue
				}
				out = append(out, ex)
			}
			if !found {
				return fmt.Errorf("no preset named %q", args[0])
			}
			cfg.Presets = out
			if err := config.Save(cfg); err != nil {
				return err
			}
			fmt.Printf("removed preset %q\n", args[0])
			return nil
		},
	}
}
