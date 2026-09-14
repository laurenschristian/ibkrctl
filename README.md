# ibkrctl

[![CI](https://github.com/laurenschristian/ibkrctl/actions/workflows/ci.yml/badge.svg)](https://github.com/laurenschristian/ibkrctl/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

One static binary: a CLI and an [MCP](https://modelcontextprotocol.io) server for Interactive Brokers, over the local Client Portal Gateway. It installs the gateway as an always-on launchd agent, keeps the session alive, and logs you in by auto-filling credentials from the macOS Keychain.

```
ibkrctl init                       # store username + password (Keychain)
ibkrctl gateway install            # install the gateway + JRE, load launchd agents
ibkrctl login                      # auto-fill credentials, approve 2FA once
ibkrctl status                     # authenticated / connected
ibkrctl account                    # brokerage accounts
ibkrctl positions                  # positions for the default account
ibkrctl pnl                        # live profit and loss
ibkrctl orders                     # live orders
ibkrctl summary                    # net liquidation, buying power
ibkrctl ledger                     # cash by currency
ibkrctl allocation                 # value by asset class / sector
ibkrctl trades                     # executions, last 7 days
ibkrctl search NVDA                # resolve a ticker to a conid
ibkrctl quote 265598               # market-data snapshot by conid
ibkrctl history 265598 --period 1y --bar 1d
ibkrctl fundamentals 265598        # market cap, P/E, EPS, yield
ibkrctl chain AAPL --month JAN27   # option strikes
ibkrctl scanner --type TOP_PERC_GAIN
ibkrctl watchlists                 # your watchlists; `watchlists get <id>` for members
ibkrctl news --num 5               # top market news
ibkrctl notifications              # IBKR account notifications
ibkrctl fx EUR                     # spot exchange rate vs USD
ibkrctl futures ES NQ CL           # futures contracts by underlying
ibkrctl alerts                     # price alerts
ibkrctl transactions 208813719 --days 30
ibkrctl place 265598 --side BUY --qty 1 --type LMT --price 190 --confirm
ibkrctl cancel <orderId>
ibkrctl raw iserver/accounts       # any /v1/api path
ibkrctl mcp                        # MCP server over stdio
```

Add `--json` to any command for raw output.

## Why

IBKR individual accounts can only authenticate through the Java Client Portal Gateway (OAuth is institutional-only). The usual setup spawns the gateway per tool launch and forces a fresh browser login every time. ibkrctl fixes the lifecycle: one always-on gateway, a keepalive that tickles the session every 60s, and a login that fills your stored credentials so you only approve the second factor.

## Install

```console
brew install --cask laurenschristian/tap/ibkrctl
```

Or `go install github.com/laurenschristian/ibkrctl@latest`, or grab a binary from [Releases](https://github.com/laurenschristian/ibkrctl/releases).

## Setup

1. `ibkrctl init` stores your IBKR username and password. The password goes into the macOS login Keychain (service `ibkrctl`); the config file only keeps a `password_cmd` that reads it back. The password never lives in plain text.
2. `ibkrctl gateway install` copies a Client Portal Gateway and a bundled JRE into `~/Library/Application Support/ibkrctl/gateway`, patches the listen port to 5001 (macOS holds 5000), and loads two launchd agents: the gateway (always on) and a 60s keepalive. Run this from a normal Terminal window, not over SSH, so launchd can bootstrap the GUI agent.
3. `ibkrctl login` opens a headless Chrome, fills your credentials, and waits while you approve the second factor. IBKR mandates 2FA for live accounts, so the second factor (an IB Key push you approve on your phone, or a code you enter) is the one step that stays manual. After that the keepalive holds the session all day.

`ibkrctl login --show` runs the browser visibly (useful the first time or to debug field selectors). `ibkrctl login --manual` just opens the page for you to type into. `ibkrctl login --otp <code>` supplies a code non-interactively.

## Privacy: hide your account numbers

Give each account a stable alias and turn on redaction so real account numbers and holder names never appear in output (or reach an AI agent):

```
ibkrctl account autoname     # assign account-1, account-2, ... to your accounts
ibkrctl account alias U1234567 main   # or name them yourself
ibkrctl account redact on
```

With redaction on, every command and MCP tool shows the alias (`account-1`), and commands accept the alias in place of the real id. The real ids live only in the config file (mode 0600).

## Configure

Precedence is flags, then environment, then the config file
(`~/Library/Application Support/ibkrctl/config.yaml` on macOS, `~/.config/ibkrctl/config.yaml` on Linux).

Environment: `IBKR_URL`, `IBKR_PORT`, `IBKR_ACCOUNT`, `IBKR_USER`, `IBKR_PASS_CMD`, `IBKR_GATEWAY_DIR`, `IBKR_JAVA`, `IBKR_CONFIG`.

## Gateway

```
ibkrctl gateway install [--from <dir>] [--java <path>]   # --from copies an existing clientportal.gw
ibkrctl gateway status
ibkrctl gateway start | stop | restart
ibkrctl gateway uninstall --yes
```

`install` auto-detects a bundled `clientportal.gw` and JRE (for example from an existing IBKR MCP install under `~/.npm`). Pass `--from` to copy one you already have, or `--java` to use a specific JRE.

## MCP

`ibkrctl mcp` exposes read tools: `ibkr_status`, `ibkr_accounts`, `ibkr_positions`, `ibkr_summary`, `ibkr_ledger`, `ibkr_allocation`, `ibkr_pnl`, `ibkr_orders`, `ibkr_trades`, `ibkr_quote`, `ibkr_search`, `ibkr_info`, `ibkr_history`, `ibkr_fundamentals`, `ibkr_chain`, `ibkr_scanner`, `ibkr_watchlists`, `ibkr_watchlist`, `ibkr_news`, `ibkr_notifications`, `ibkr_fx`, `ibkr_futures`, `ibkr_alerts`, and `ibkr_raw`. Order placement and cancellation are deliberately CLI-only (they require `--confirm`).

```console
claude mcp add ibkr -- ibkrctl mcp
```

Any stdio MCP client (Cursor, Claude Desktop, Zed) works the same: command `ibkrctl`, args `["mcp"]`.

## Development

```
make hooks    # gofmt, dash check, gitleaks, build, lint on commit; tests + coverage on push
make test
make cover    # floor in scripts/coverage.sh
make lint
make sec
```

## Notes

- The gateway proxies to `api.ibkr.com` and holds the session, so ibkrctl never sees your password after `login` fills it.
- The stock gateway `conf.yaml` ships a malformed deny IP that 404s the login page; `gateway install` strips it automatically.
- Market-data fields are IBKR field ids (`31` last, `55` symbol, `84` bid, `86` ask, `87` volume). Pass `--fields` to `quote` to choose.

MIT.
