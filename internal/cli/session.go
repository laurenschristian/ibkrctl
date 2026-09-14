package cli

import (
	"errors"
	"fmt"
	"time"

	"github.com/spf13/cobra"
)

func reconnectCmd() *cobra.Command {
	var compete bool
	c := &cobra.Command{
		Use:   "reconnect",
		Short: "Revive a dropped brokerage session without a full login (no 2FA)",
		Long: "Re-initialize and reauthenticate the brokerage session when it has " +
			"dropped but the SSO cookie is still valid. If SSO has fully expired " +
			"this cannot help; run `ibkrctl login` instead.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if st, err := client.AuthStatus(ctx); err == nil && st.Authenticated {
				fmt.Println("already authenticated")
				return nil
			}
			_, _ = client.SSOInit(ctx, compete)
			if _, err := client.Reauthenticate(ctx); err != nil {
				return err
			}
			// Give the gateway a moment to complete the handshake, then confirm.
			for i := 0; i < 8; i++ {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(1500 * time.Millisecond):
				}
				if st, err := client.AuthStatus(ctx); err == nil && st.Authenticated {
					fmt.Println("reconnected")
					return nil
				}
			}
			return errors.New("still not authenticated: SSO likely expired, run `ibkrctl login`")
		},
	}
	c.Flags().BoolVar(&compete, "compete", true, "take over a competing session on another device")
	return c
}

func validateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "validate",
		Short: "Validate the SSO session (username, expiry)",
		RunE: func(cmd *cobra.Command, _ []string) error {
			data, err := client.Validate(cmd.Context())
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
}

func scheduleCmd() *cobra.Command {
	var asset, exchange, filter string
	c := &cobra.Command{
		Use:   "schedule <symbol>",
		Short: "Trading hours and sessions for a symbol",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			data, err := client.TradingSchedule(cmd.Context(), asset, args[0], exchange, filter)
			if err != nil {
				return err
			}
			return emit(data)
		},
	}
	c.Flags().StringVar(&asset, "asset", "STK", "asset class (STK, OPT, FUT, ...)")
	c.Flags().StringVar(&exchange, "exchange", "", "exchange")
	c.Flags().StringVar(&filter, "exchange-filter", "", "exchange filter")
	return c
}
